package pages

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

func TestAdmin(t *testing.T) {
	got := Admin().Render()

	golden.HTML(t, "admin", got)
	require.Contains(t, got, static.URL("admin.css"))
	require.Contains(t, got, static.URL("admin.js"))
	require.NotContains(t, got, "site.css", "the admin has its own stylesheet")
	require.NotContains(t, got, ui.UmamiOrigin, "admin visits are not site visits")
}

func TestAdminHandler(t *testing.T) {
	want := Admin().Render()

	for _, target := range []string{"/admin/", "/admin/arrangementer/12", "/admin/galleri"} {
		t.Run(target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			AdminHandler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, want, rec.Body.String())
			require.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
			require.Equal(t, ui.AdminCSP(), rec.Header().Get("Content-Security-Policy"))
			require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
		})
	}
}

func TestAdminHandlerRefusesWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	AdminHandler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/", nil))

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Equal(t, "GET, HEAD", rec.Header().Get("Allow"))
}

// Every address in admin.js is a string that is never loaded: links in
// Elm's error messages, the URL scheme the Url package knows, and the
// Cropper.js licence banner (kept, as its licence asks). Requests go through
// elm/http's XMLHttpRequest to relative /admin/api/ paths; nothing else
// reaches the network.
func TestAdminScriptLoadsNothingFromOutside(t *testing.T) {
	js := staticFile(t, "admin.js")

	for _, api := range []string{"fetch(", "WebSocket", "EventSource", "sendBeacon", "import("} {
		require.NotContains(t, js, api)
	}

	for _, u := range regexp.MustCompile(`https?://[^\s"'\x60)]+`).FindAllString(js, -1) {
		require.Regexp(t, `^(https://(github\.com/elm/|package\.elm-lang\.org/|fengyuanchen\.github\.io/cropperjs$)|https?://\.?$)`, u,
			"admin.js names an outside address")
	}
}

func TestAdminStylesheetLoadsNothingFromOutside(t *testing.T) {
	css := staticFile(t, "admin.css")
	require.NotContains(t, css, "@import", "the built stylesheet must be self-contained")

	for _, m := range regexp.MustCompile(`url\(([^)]*)\)`).FindAllStringSubmatch(css, -1) {
		ref := strings.Trim(m[1], `'"`)
		require.True(t, ui.ExternalLoadAllowed(ui.Umami{}, "font-src", ref), "admin.css loads %s", ref)
	}
}
