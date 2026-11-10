package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/db"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *clock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.t = t
}

func oslo(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, site.Oslo)
	if err != nil {
		panic(err)
	}

	return t
}

// fakeImages records what the server tells the photo store.
type fakeImages struct {
	changed  int
	enqueued []string
}

func (*fakeImages) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("photo")) })
}

func (f *fakeImages) Changed()              { f.changed++ }
func (f *fakeImages) Enqueue(sha256 string) { f.enqueued = append(f.enqueued, sha256) }

type fixture struct {
	db     *db.DB
	srv    *Server
	store  *site.Store
	clock  *clock
	images *fakeImages
	h      http.Handler
}

func newFixture(t *testing.T, dev bool) *fixture {
	t.Helper()

	d, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	clk := &clock{t: oslo("2026-11-10 12:00")}
	store := site.NewStore(d, clk.now)
	imgs := &fakeImages{}
	srv := New(Config{BaseURL: "https://aspargesgaarden.no", Dev: dev, Store: store, Images: imgs, Now: clk.now})

	return &fixture{db: d, srv: srv, store: store, clock: clk, images: imgs, h: srv.Public()}
}

func (f *fixture) get(t *testing.T, target string, header ...string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}

	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)

	return rec
}

// publish stores and publishes an event with a main photo.
func (f *fixture) publish(t *testing.T, e site.Event) site.Event {
	t.Helper()

	ctx := t.Context()

	p, err := f.store.AddPhoto(ctx, site.Photo{
		SHA256: strings.Repeat(string(rune('a'+len(e.Title)%6)), 64), MIME: "image/jpeg", Width: 1600, Height: 1200, Alt: e.Title,
	})
	require.NoError(t, err)

	e.MainPhoto = &p

	e, err = f.store.CreateEvent(ctx, e)
	require.NoError(t, err)

	e, err = f.store.Publish(ctx, e.ID)
	require.NoError(t, err)

	return e
}

func date(s string) site.Date {
	d, err := site.ParseDate(s)
	if err != nil {
		panic(err)
	}

	return d
}

// withMarkets has the 2025 and 2026 Christmas markets and a draft.
func withMarkets(t *testing.T) *fixture {
	t.Helper()

	f := newFixture(t, false)
	f.publish(t, site.Event{Title: "Julemarked", Start: date("2025-11-15"), End: date("2025-11-16")})
	f.publish(t, site.Event{Title: "Julemarked", Start: date("2026-11-14"), End: date("2026-11-15"), ShowInMenu: true})

	_, err := f.store.CreateEvent(t.Context(), site.Event{Title: "Kladd", Start: date("2026-12-01")})
	require.NoError(t, err)

	return f
}

// The plan's address table. Every row is here, so a redirect cannot be
// dropped by accident.
func TestAddresses(t *testing.T) {
	f := withMarkets(t)

	tests := []struct {
		path     string
		wantCode int
		wantTo   string // Location for redirects
		wantBody string // for pages
	}{
		{path: "/", wantCode: http.StatusOK, wantBody: `<link href="https://aspargesgaarden.no/" rel="canonical">`},
		{path: "/lokaler", wantCode: http.StatusOK, wantBody: ">Muligheter</h1>"},
		{path: "/om-oss", wantCode: http.StatusOK, wantBody: ">Om gården</h1>"},
		{path: "/kontakt", wantCode: http.StatusOK, wantBody: ">Kontakt oss</h1>"},
		{path: "/arrangementer", wantCode: http.StatusOK, wantBody: ">Arrangementer</h1>"},
		{path: "/arrangementer/2026/julemarked", wantCode: http.StatusOK, wantBody: `<link href="https://aspargesgaarden.no/arrangementer/2026/julemarked" rel="canonical">`},
		// Past events keep their page.
		{path: "/arrangementer/2025/julemarked", wantCode: http.StatusOK, wantBody: "Arrangementet er over."},
		{path: "/galleri", wantCode: http.StatusOK, wantBody: ">Galleri</h1>"},

		{path: "/facilities", wantCode: http.StatusMovedPermanently, wantTo: "/lokaler"},
		{path: "/about", wantCode: http.StatusMovedPermanently, wantTo: "/om-oss"},
		{path: "/contact", wantCode: http.StatusMovedPermanently, wantTo: "/kontakt"},
		{path: "/events", wantCode: http.StatusMovedPermanently, wantTo: "/arrangementer"},
		{path: "/christmas", wantCode: http.StatusMovedPermanently, wantTo: "/julemarked"},
		{path: "/christmas/2026", wantCode: http.StatusMovedPermanently, wantTo: "/arrangementer/2026/julemarked"},
		{path: "/gallery", wantCode: http.StatusMovedPermanently, wantTo: "/galleri"},
		// Temporary: next year the newest market is another page.
		{path: "/julemarked", wantCode: http.StatusFound, wantTo: "/arrangementer/2026/julemarked"},

		// Trailing slashes and old addresses with one: a single hop.
		{path: "/lokaler/", wantCode: http.StatusMovedPermanently, wantTo: "/lokaler"},
		{path: "/facilities/", wantCode: http.StatusMovedPermanently, wantTo: "/lokaler"},
		{path: "/about/?ref=fb", wantCode: http.StatusMovedPermanently, wantTo: "/om-oss?ref=fb"},
		{path: "/contact?ref=fb", wantCode: http.StatusMovedPermanently, wantTo: "/kontakt?ref=fb"},
		{path: "/events/", wantCode: http.StatusMovedPermanently, wantTo: "/arrangementer"},
		{path: "/arrangementer/", wantCode: http.StatusMovedPermanently, wantTo: "/arrangementer"},
		{path: "/arrangementer/2026/julemarked/", wantCode: http.StatusMovedPermanently, wantTo: "/arrangementer/2026/julemarked"},
		{path: "/christmas/2026/", wantCode: http.StatusMovedPermanently, wantTo: "/arrangementer/2026/julemarked"},
		{path: "/galleri/", wantCode: http.StatusMovedPermanently, wantTo: "/galleri"},
		{path: "/gallery/", wantCode: http.StatusMovedPermanently, wantTo: "/galleri"},
		{path: "/julemarked/?ref=fb", wantCode: http.StatusFound, wantTo: "/arrangementer/2026/julemarked?ref=fb"},

		// Unknown addresses get the 404 page with a real 404, slash or not.
		{path: "/finnes-ikke", wantCode: http.StatusNotFound, wantBody: ">Fant ikke siden</h1>"},
		{path: "/finnes-ikke/", wantCode: http.StatusNotFound, wantBody: ">Fant ikke siden</h1>"},
		{path: "/lokaler/x", wantCode: http.StatusNotFound, wantBody: ">Fant ikke siden</h1>"},
		{path: "/index.html", wantCode: http.StatusNotFound},
		{path: "/arrangementer/2026/finnes-ikke", wantCode: http.StatusNotFound, wantBody: ">Fant ikke siden</h1>"},
		{path: "/arrangementer/2026/finnes-ikke/", wantCode: http.StatusNotFound},
		{path: "/arrangementer/2024/julemarked", wantCode: http.StatusNotFound},
		{path: "/arrangementer/02026/julemarked", wantCode: http.StatusNotFound},
		{path: "/arrangementer/2026", wantCode: http.StatusNotFound},
		{path: "/arrangementer/2026/julemarked/x", wantCode: http.StatusNotFound},
		{path: "/christmas/2025", wantCode: http.StatusNotFound},
		// Drafts are not public.
		{path: "/arrangementer/2026/kladd", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := f.get(t, tt.path)

			require.Equal(t, tt.wantCode, rec.Code)
			require.Equal(t, tt.wantTo, rec.Header().Get("Location"))
			require.Contains(t, rec.Body.String(), tt.wantBody)

			if tt.wantTo == "" {
				require.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
				require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestJulemarkedWithoutAMarketIsNotFound(t *testing.T) {
	f := newFixture(t, false)

	require.Equal(t, http.StatusNotFound, f.get(t, "/julemarked").Code)
	require.Equal(t, http.StatusNotFound, f.get(t, "/julemarked/").Code)
}

// An empty database is a working site: no events yet, nothing in the menu.
func TestEmptyDatabase(t *testing.T) {
	f := newFixture(t, false)

	rec := f.get(t, "/arrangementer")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), ">Ingen kommende arrangementer for øyeblikket.</p>")
	require.NotContains(t, rec.Body.String(), ">Julemarked</a>")
}

// The menu shows an event marked "Vis i menyen" until it is over.
func TestMenuFollowsTheEvents(t *testing.T) {
	f := withMarkets(t)

	link := `href="/arrangementer/2026/julemarked">Julemarked</a>`
	require.Contains(t, f.get(t, "/").Body.String(), link)

	f.clock.set(oslo("2026-11-16 00:00"))
	require.NotContains(t, f.get(t, "/").Body.String(), link)
}

// At midnight in Oslo the market moves from coming to past, and the kept
// list is replaced without anyone calling Changed.
func TestEventCrossingMidnightMovesToPast(t *testing.T) {
	f := withMarkets(t)
	f.clock.set(oslo("2026-11-15 23:59"))

	before := f.get(t, "/arrangementer")
	coming, _, _ := strings.Cut(before.Body.String(), "Tidligere arrangementer")
	require.Contains(t, coming, `href="/arrangementer/2026/julemarked"`)
	require.NotContains(t, f.get(t, "/arrangementer/2026/julemarked").Body.String(), "Arrangementet er over")

	f.clock.set(oslo("2026-11-16 00:00"))

	after := f.get(t, "/arrangementer")
	coming, past, _ := strings.Cut(after.Body.String(), "Tidligere arrangementer")
	require.NotContains(t, coming, `href="/arrangementer/2026/julemarked"`)
	require.Contains(t, past, `href="/arrangementer/2026/julemarked"`)
	require.NotEqual(t, before.Header().Get("ETag"), after.Header().Get("ETag"))
	require.Contains(t, f.get(t, "/arrangementer/2026/julemarked").Body.String(), "Arrangementet er over")
}

func TestPagesAreKeptUntilChanged(t *testing.T) {
	f := withMarkets(t)

	first := f.get(t, "/arrangementer")
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)

	again := f.get(t, "/arrangementer", "If-None-Match", etag)
	require.Equal(t, http.StatusNotModified, again.Code)
	require.Empty(t, again.Body.String())

	// A save the admin has not announced is not seen: the page is kept.
	e := f.publish(t, site.Event{Title: "Vårkonsert", Start: date("2027-05-19")})
	require.Equal(t, http.StatusNotModified, f.get(t, "/arrangementer", "If-None-Match", etag).Code)

	f.srv.Changed(e.MainPhoto.SHA256)

	changed := f.get(t, "/arrangementer", "If-None-Match", etag)
	require.Equal(t, http.StatusOK, changed.Code)
	require.Contains(t, changed.Body.String(), "Vårkonsert")
	require.NotEqual(t, etag, changed.Header().Get("ETag"))

	// The photo store hears of the change too, and builds the new photo.
	require.Equal(t, 1, f.images.changed)
	require.Equal(t, []string{e.MainPhoto.SHA256}, f.images.enqueued)
}

// Fixed pages carry the menu, so they are kept and invalidated like the rest.
func TestFixedPagesAreKept(t *testing.T) {
	f := withMarkets(t)

	rec := f.get(t, "/kontakt", "Accept-Encoding", "gzip")
	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	require.Equal(t, http.StatusNotModified, f.get(t, "/kontakt", "Accept-Encoding", "gzip", "If-None-Match", rec.Header().Get("ETag")).Code)
}

func TestNotFoundIsNotKept(t *testing.T) {
	f := withMarkets(t)

	rec := f.get(t, "/arrangementer/2027/varkonsert")
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Empty(t, rec.Header().Get("ETag"))

	e := f.publish(t, site.Event{Title: "Vårkonsert", Start: date("2027-05-19")})
	require.Equal(t, "/arrangementer/2027/varkonsert", e.Path())
	require.Equal(t, http.StatusOK, f.get(t, e.Path()).Code, "a 404 is never kept, even without Changed")
}

// The gallery is kept like any page, shows the admin's order once Changed,
// and loads the lightbox.
func TestGallery(t *testing.T) {
	f := withMarkets(t)
	ctx := t.Context()

	empty := f.get(t, "/galleri")
	require.Equal(t, http.StatusOK, empty.Code)
	require.NotContains(t, empty.Body.String(), "gallery.js", "no photos, nothing to open")

	ids := make([]int64, 0, 2)

	for _, c := range "12" { // hex, and not a market photo's
		p, err := f.store.AddPhoto(ctx, site.Photo{SHA256: strings.Repeat(string(c), 64), MIME: "image/jpeg", Width: 1600, Height: 1200, Alt: "Bilde " + string(c)})
		require.NoError(t, err)

		ids = append(ids, p.ID)
	}

	require.NoError(t, f.store.SetGallery(ctx, ids))

	require.NotContains(t, f.get(t, "/galleri").Body.String(), "data-gallery", "kept until Changed")

	f.srv.Changed()

	body := f.get(t, "/galleri").Body.String()
	require.Contains(t, body, `alt="Bilde 1"`)
	require.Less(t, strings.Index(body, `alt="Bilde 1"`), strings.Index(body, `alt="Bilde 2"`))
	require.Contains(t, body, `<script defer src="`+static.URL("gallery.js")+`">`)
}

// A database failure must not look like a missing page to a search engine,
// leak the error, or be kept as the page.
func TestGalleryDatabaseFailure(t *testing.T) {
	f := newFixture(t, false)
	require.NoError(t, f.db.Close())

	rec := f.get(t, "/galleri")
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Empty(t, rec.Header().Get("ETag"))
	require.NotContains(t, rec.Body.String(), "sql")
}

func TestImages(t *testing.T) {
	f := withMarkets(t)

	rec := f.get(t, "/img/x.jpeg")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "photo", rec.Body.String())
	require.Equal(t, ui.CSP(), rec.Header().Get("Content-Security-Policy"))
}

func TestEveryResponseCarriesCSP(t *testing.T) {
	f := withMarkets(t)

	for _, target := range []string{"/", "/facilities", "/finnes-ikke", "/arrangementer", "/julemarked", "/galleri", static.URL("site.css"), static.URL("gallery.js")} {
		rec := f.get(t, target)

		require.Equal(t, ui.CSP(), rec.Header().Get("Content-Security-Policy"), target)
		require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"), target)
	}
}

func TestStatic(t *testing.T) {
	rec := withMarkets(t).get(t, static.URL("site.css"))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "public, max-age=31536000, immutable", rec.Header().Get("Cache-Control"))
}

func TestPublicCarriesNoInternals(t *testing.T) {
	tests := []struct {
		name     string
		dev      bool
		path     string
		wantCode int
		wantBody string
	}{
		// The public listener is the one nginx exposes: no metrics, debug or
		// admin may ever answer there outside --dev.
		{name: "metrics", path: "/metrics", wantCode: http.StatusNotFound},
		{name: "debug", path: "/debug/", wantCode: http.StatusNotFound},
		{name: "admin", path: "/admin/", wantCode: http.StatusNotFound},
		{name: "metrics-dev", dev: true, path: "/metrics", wantCode: http.StatusOK, wantBody: "go_goroutines"},
		{name: "debug-dev", dev: true, path: "/debug/", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := newFixture(t, tt.dev).get(t, tt.path)

			require.Equal(t, tt.wantCode, rec.Code)
			require.Contains(t, rec.Body.String(), tt.wantBody)
		})
	}
}
