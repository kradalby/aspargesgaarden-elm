package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

func get(t *testing.T, dev bool, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	Public("https://aspargesgaarden.no", dev).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))

	return rec
}

// The plan's address table. Every row that exists so far is here, so a
// redirect cannot be dropped by accident.
func TestAddresses(t *testing.T) {
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

		{path: "/facilities", wantCode: http.StatusMovedPermanently, wantTo: "/lokaler"},
		{path: "/about", wantCode: http.StatusMovedPermanently, wantTo: "/om-oss"},
		{path: "/contact", wantCode: http.StatusMovedPermanently, wantTo: "/kontakt"},

		// Events and gallery rows go here with their pages:
		// /arrangementer, /arrangementer/{year}/{slug}, /julemarked (302),
		// /galleri, and the 301s from /events, /christmas, /christmas/2026,
		// /gallery.

		// Trailing slashes and old addresses with one: a single hop.
		{path: "/lokaler/", wantCode: http.StatusMovedPermanently, wantTo: "/lokaler"},
		{path: "/facilities/", wantCode: http.StatusMovedPermanently, wantTo: "/lokaler"},
		{path: "/about/?ref=fb", wantCode: http.StatusMovedPermanently, wantTo: "/om-oss?ref=fb"},
		{path: "/contact?ref=fb", wantCode: http.StatusMovedPermanently, wantTo: "/kontakt?ref=fb"},

		// Unknown addresses get the 404 page with a real 404, slash or not.
		{path: "/finnes-ikke", wantCode: http.StatusNotFound, wantBody: ">Fant ikke siden</h1>"},
		{path: "/finnes-ikke/", wantCode: http.StatusNotFound, wantBody: ">Fant ikke siden</h1>"},
		{path: "/lokaler/x", wantCode: http.StatusNotFound, wantBody: ">Fant ikke siden</h1>"},
		{path: "/index.html", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := get(t, false, tt.path)

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

func TestEveryResponseCarriesCSP(t *testing.T) {
	for _, target := range []string{"/", "/facilities", "/finnes-ikke", static.URL("site.css")} {
		rec := get(t, false, target)

		require.Equal(t, ui.CSP(), rec.Header().Get("Content-Security-Policy"), target)
		require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"), target)
	}
}

func TestStatic(t *testing.T) {
	rec := get(t, false, static.URL("site.css"))

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
			rec := get(t, tt.dev, tt.path)

			require.Equal(t, tt.wantCode, rec.Code)
			require.Contains(t, rec.Body.String(), tt.wantBody)
		})
	}
}
