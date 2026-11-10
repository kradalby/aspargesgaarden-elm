package server

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/kradalby/asparges/pages"
	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/site"
)

// seoRoutes adds the files crawlers read besides the pages.
func (s *Server) seoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		text(w, "text/plain; charset=utf-8", []byte(seo.Robots(s.cfg.BaseURL, s.cfg.NoIndex)))
	})
	mux.HandleFunc("GET /sitemap.xml", s.sitemap)
	mux.HandleFunc("GET /llms.txt", s.llms)

	if key := s.cfg.IndexNowKey; key != "" {
		mux.HandleFunc("GET /"+key+".txt", func(w http.ResponseWriter, _ *http.Request) {
			text(w, "text/plain; charset=utf-8", []byte(key))
		})
	}
}

func (s *Server) newIndexNow() *seo.IndexNow {
	if s.cfg.IndexNowEndpoint == "" || s.cfg.IndexNowKey == "" {
		return nil
	}

	n, err := seo.NewIndexNow(seo.IndexNowConfig{
		Endpoint: s.cfg.IndexNowEndpoint,
		Key:      s.cfg.IndexNowKey,
		BaseURL:  s.cfg.BaseURL,
		Pages:    s.indexable,
		Log:      s.cfg.Log,
	})
	if err != nil {
		s.cfg.Log.Error("indexnow off", "err", err)
		return nil
	}

	return n
}

// Run does the server's background work until ctx ends: telling search
// engines about changed pages, when that is on.
func (s *Server) Run(ctx context.Context) error {
	if s.indexNow == nil {
		<-ctx.Done()
		return nil
	}

	return s.indexNow.Run(ctx)
}

func text(w http.ResponseWriter, ctype string, body []byte) {
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}

func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	list, err := s.indexable(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}

	b, err := seo.Sitemap(s.cfg.BaseURL, list)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	text(w, "application/xml; charset=utf-8", b)
}

func (s *Server) llms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	st, err := s.site(ctx)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	events, err := s.cfg.Store.PublishedEvents(ctx)
	if err != nil {
		s.fail(w, r, fmt.Errorf("llms.txt: %w", err))
		return
	}

	links := make([]seo.Link, len(st.Menu))
	for i, m := range st.Menu {
		links[i] = seo.Link{Title: cmp.Or(pages.Titles[m.Href], m.Label), Path: m.Href}
	}

	upcoming, _ := site.Split(events, s.cfg.Now())
	text(w, "text/plain; charset=utf-8", []byte(seo.LLMs(s.cfg.BaseURL, links, upcoming)))
}

// indexable lists every page search engines should know, as routing
// answers them: the fixed pages, the event list, the gallery, and every
// published event, past ones too (they keep their page).
func (s *Server) indexable(ctx context.Context) ([]seo.Page, error) {
	events, err := s.cfg.Store.PublishedEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("sitemap events: %w", err)
	}

	gallery, err := s.cfg.Store.GalleryUpdatedAt(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]seo.Page, 0, len(pages.Fixed)+2+len(events))
	for _, p := range slices.Sorted(maps.Keys(pages.Fixed)) {
		out = append(out, seo.Page{Path: p})
	}

	var latest time.Time
	for _, e := range events {
		latest = later(latest, lastMod(e))
	}

	out = append(out, seo.Page{Path: "/arrangementer", LastMod: latest}, seo.Page{Path: "/galleri", LastMod: gallery})

	for _, e := range events {
		out = append(out, seo.Page{Path: e.Path(), LastMod: lastMod(e)})
	}

	return out, nil
}

// lastMod is the last edit of anything an event page shows: the event or
// one of its photos (alt text, crop, credit).
func lastMod(e site.Event) time.Time {
	t := e.UpdatedAt
	if e.MainPhoto != nil {
		t = later(t, e.MainPhoto.UpdatedAt)
	}

	for _, p := range e.ExtraPhotos {
		t = later(t, p.UpdatedAt)
	}

	return t
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}

	return a
}
