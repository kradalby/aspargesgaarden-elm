// Package server routes HTTP requests for asparges.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chasefleming/elem-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kradalby/asparges/pages"
	"github.com/kradalby/asparges/render"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

// redirects maps every old address to its new one, permanently. Old links
// and search results must keep working, so rows are never removed.
var redirects = map[string]string{
	"/facilities":     "/lokaler",
	"/about":          "/om-oss",
	"/contact":        "/kontakt",
	"/events":         "/arrangementer",
	"/christmas":      "/julemarked",
	"/christmas/2026": "/arrangementer/2026/julemarked",
	"/gallery":        "/galleri",
}

// evergreen addresses lead to the newest published event with a slug. The
// redirect is temporary: next year it points elsewhere.
var evergreen = map[string]string{
	"/julemarked": "julemarked",
}

// Config is what the public site is built from.
type Config struct {
	// BaseURL is the public origin, without a trailing slash.
	BaseURL string
	// Dev serves metrics on the public handler; see Public.
	Dev    bool
	Store  *site.Store
	Images Images
	// Now is the clock that decides upcoming and past.
	Now func() time.Time
	Log *slog.Logger
	// NoIndex keeps the site out of search engines (the beta site): every
	// response says noindex and robots.txt shuts crawlers out.
	NoIndex bool
}

// Images is the photo store (images.Store) as the server uses it.
type Images interface {
	// Handler serves the variants under /img/.
	Handler() http.Handler
	// Changed and Enqueue tell it the database's photos or crops changed,
	// and which photos need their variants built.
	Changed()
	Enqueue(sha256 string)
}

// Server answers the public site, keeping rendered pages until a change.
type Server struct {
	cfg   Config
	pages *render.Store
}

// New returns a Server; cfg.Store, cfg.Images and cfg.Now are required.
func New(cfg Config) *Server {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}

	return &Server{cfg: cfg, pages: render.New(cfg.Now)}
}

// Changed is the one call every admin write makes once it has committed:
// events, photos, crops, photographers, the gallery. It names the photos
// that were uploaded, recropped or newly published, so their variants are
// built now rather than on first view; every kept page is dropped and
// renders afresh on its next request.
func (s *Server) Changed(photos ...string) {
	s.cfg.Images.Changed()

	for _, sha := range photos {
		s.cfg.Images.Enqueue(sha)
	}

	s.pages.Invalidate()
}

// Public returns the handler for the local listener. In production nginx
// exposes that listener to the internet, so it must only carry public pages;
// metrics, debug and admin belong on the tsnet listener.
func (s *Server) Public() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET "+static.Prefix, static.Handler())

	mux.Handle("GET /img/", s.cfg.Images.Handler())
	s.seoRoutes(mux)
	mux.HandleFunc("GET /", s.route)

	// --dev runs without tsnet, so its loopback-only listener stands in for
	// the tailnet one to keep metrics reachable during local work.
	if s.cfg.Dev {
		mux.Handle("GET /metrics", promhttp.Handler())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", ui.CSP())
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if s.cfg.NoIndex {
			w.Header().Set("X-Robots-Tag", "noindex")
		}
		mux.ServeHTTP(w, r)
	})
}

// answer is how the site answers an address: a page kept under it, or a
// redirect.
type answer struct {
	build render.Builder
	to    string
	code  int
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	ctx, path := r.Context(), r.URL.Path

	a, ok, err := s.lookup(ctx, path)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	// An address with a trailing slash goes, in one hop, wherever the address
	// without it does, but only if that exists: anything else is a plain 404.
	if trimmed := strings.TrimRight(path, "/"); !ok && trimmed != path && trimmed != "" {
		a, ok, err = s.lookup(ctx, trimmed)
		if err != nil {
			s.fail(w, r, err)
			return
		}

		if ok && a.build != nil {
			a = answer{to: trimmed, code: http.StatusMovedPermanently, build: a.build}
		}
	}

	switch {
	case !ok:
		s.notFound(w, r)
	case a.to == "":
		s.servePage(w, r, path, a.build)
	default:
		// A page behind a slash redirect must exist before we point at it.
		if a.build != nil {
			if _, err := s.pages.Get(ctx, a.to, a.build); err != nil {
				s.pageErr(w, r, err)
				return
			}
		}

		to := a.to
		if r.URL.RawQuery != "" {
			to += "?" + r.URL.RawQuery
		}

		// The path is one of ours (a known address or an event's); only the
		// query comes from the request, and it cannot change the host.
		http.Redirect(w, r, to, a.code) //nolint:gosec // see above
	}
}

// lookup finds the answer for path; ok is false for an unknown address.
func (s *Server) lookup(ctx context.Context, path string) (a answer, ok bool, err error) {
	if page, fixed := pages.Fixed[path]; fixed {
		return answer{build: func(ctx context.Context) (*elem.Element, error) {
			st, err := s.site(ctx)
			return page(st), err
		}}, true, nil
	}

	switch path {
	case "/arrangementer":
		return answer{build: s.eventList}, true, nil
	case "/galleri":
		return answer{build: s.gallery}, true, nil
	}

	if year, slug, isEvent := eventAddress(path); isEvent {
		return answer{build: s.eventPage(year, slug)}, true, nil
	}

	if to, old := redirects[path]; old {
		return answer{to: to, code: http.StatusMovedPermanently}, true, nil
	}

	if slug, ever := evergreen[path]; ever {
		e, err := s.cfg.Store.NewestPublishedEvent(ctx, slug)
		switch {
		case errors.Is(err, site.ErrNotFound):
			return answer{}, false, nil
		case err != nil:
			return answer{}, false, err
		}

		return answer{to: e.Path(), code: http.StatusFound}, true, nil
	}

	return answer{}, false, nil
}

// eventAddress parses /arrangementer/{year}/{slug}. Only the canonical
// spelling counts, so one page is never kept under two keys.
func eventAddress(path string) (year int, slug string, ok bool) {
	rest, ok := strings.CutPrefix(path, "/arrangementer/")
	if !ok {
		return 0, "", false
	}

	y, slug, ok := strings.Cut(rest, "/")
	if !ok || slug == "" || strings.Contains(slug, "/") {
		return 0, "", false
	}

	year, err := strconv.Atoi(y)
	if err != nil || strconv.Itoa(year) != y || year < 1 {
		return 0, "", false
	}

	return year, slug, true
}

func (s *Server) eventList(ctx context.Context) (*elem.Element, error) {
	st, err := s.site(ctx)
	if err != nil {
		return nil, err
	}

	events, err := s.cfg.Store.PublishedEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("event list: %w", err)
	}

	return pages.Events(st, events, s.cfg.Now()), nil
}

func (s *Server) eventPage(year int, slug string) render.Builder {
	return func(ctx context.Context) (*elem.Element, error) {
		e, err := s.cfg.Store.PublishedEvent(ctx, year, slug)
		if err != nil {
			return nil, err
		}

		st, err := s.site(ctx)
		if err != nil {
			return nil, err
		}

		return pages.Event(st, e, s.cfg.Now()), nil
	}
}

func (s *Server) gallery(ctx context.Context) (*elem.Element, error) {
	st, err := s.site(ctx)
	if err != nil {
		return nil, err
	}

	photos, err := s.cfg.Store.Gallery(ctx)
	if err != nil {
		return nil, fmt.Errorf("gallery: %w", err)
	}

	return pages.Galleri(st, photos), nil
}

// site is what pages need, with today's menu. On error the menu lacks the
// events' entries but is still usable, for the 404 page.
func (s *Server) site(ctx context.Context) (pages.Site, error) {
	st := pages.Site{BaseURL: s.cfg.BaseURL, Menu: pages.Menu()}

	entries, err := s.cfg.Store.Menu(ctx)
	if err != nil {
		return st, fmt.Errorf("menu: %w", err)
	}

	st.Menu = pages.Menu(entries...)

	return st, nil
}

func (s *Server) servePage(w http.ResponseWriter, r *http.Request, key string, build render.Builder) {
	p, err := s.pages.Get(r.Context(), key, build)
	if err != nil {
		s.pageErr(w, r, err)
		return
	}

	p.ServeHTTP(w, r)
}

func (s *Server) pageErr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, site.ErrNotFound) {
		s.notFound(w, r)
		return
	}

	s.fail(w, r, err)
}

// notFound is rendered each time and never kept: unknown addresses are
// endless, and the store holds only real pages.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	st, err := s.site(r.Context())
	if err != nil {
		s.cfg.Log.ErrorContext(r.Context(), "rendering 404", "err", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(pages.NotFound(st).Render()))
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	s.cfg.Log.ErrorContext(r.Context(), "serving page", "path", r.URL.Path, "err", err)
	http.Error(w, "Noe gikk galt. Prøv igjen om litt.", http.StatusInternalServerError)
}
