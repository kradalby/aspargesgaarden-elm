package static

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

var (
	fontFaceBlock = regexp.MustCompile(`@font-face\{([^}]*)\}`)
	faceFamily    = regexp.MustCompile(`font-family:\s*"?([^;"}]+)"?`)
	faceURL       = regexp.MustCompile(`url\(([^)]+)\)`)
	varDecl       = regexp.MustCompile(`(--[\w-]+):`)
	varFallback   = regexp.MustCompile(`var\((--[\w-]+),[^)]*\)`)
	familyValue   = regexp.MustCompile(`(font-family|--font-[\w-]+|--default-[\w-]*font-family):([^;}]+)`)
)

// CSS generic families need no @font-face: the browser always has one.
var genericFamilies = []string{"serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui", "math", "emoji", "fangsong"}

// The plan's font check: every family a served stylesheet asks for has an
// embedded @font-face, and every @font-face points at a file in the binary.
// A var() fallback only counts when its variable is undefined, since only
// then can the browser reach it.
func TestFontsAreEmbedded(t *testing.T) {
	for _, sheet := range []string{"site.css", "admin.css"} {
		t.Run(sheet, func(t *testing.T) { checkFonts(t, string(assets[sheet].data)) })
	}
}

func checkFonts(t *testing.T, css string) {
	t.Helper()

	faces := map[string]bool{}

	for _, block := range fontFaceBlock.FindAllStringSubmatch(css, -1) {
		family := faceFamily.FindStringSubmatch(block[1])
		require.NotNil(t, family, "@font-face without font-family: %s", block[0])
		faces[family[1]] = true

		for _, u := range faceURL.FindAllStringSubmatch(block[1], -1) {
			name, hash, ok := strings.Cut(strings.TrimPrefix(strings.Trim(u[1], `"'`), Prefix), "?v=")
			require.True(t, ok, "font URL %s is not content-hashed", u[1])

			a, ok := assets[name]
			require.True(t, ok, "@font-face %s: %s is not embedded", family[1], name)
			require.Equal(t, a.hash, hash, "@font-face %s: stale hash", family[1])
		}
	}

	require.NotEmpty(t, faces)

	defined := map[string]bool{}
	for _, m := range varDecl.FindAllStringSubmatch(css, -1) {
		defined[m[1]] = true
	}

	rules := fontFaceBlock.ReplaceAllString(css, "")
	rules = varFallback.ReplaceAllStringFunc(rules, func(m string) string {
		if v := varFallback.FindStringSubmatch(m)[1]; defined[v] {
			return "var(" + v + ")"
		}

		return m
	})

	for _, m := range familyValue.FindAllStringSubmatch(rules, -1) {
		// v4 keeps weights in their own --font-weight-* namespace.
		if strings.HasPrefix(m[1], "--font-weight-") {
			continue
		}

		for f := range strings.SplitSeq(m[2], ",") {
			f = strings.Trim(strings.TrimSpace(f), `"'`)
			if strings.HasPrefix(f, "var(") || slices.Contains(genericFamilies, f) {
				continue
			}

			require.True(t, faces[f], "the stylesheet asks for font %q but embeds no @font-face for it (%s)", f, m[0])
		}
	}
}

func TestHandler(t *testing.T) {
	cssURL := URL("site.css")
	hash := assets["site.css"].hash

	tests := []struct {
		name        string
		target      string
		header      http.Header
		wantCode    int
		wantType    string
		wantCaching string
	}{
		{name: "hashed", target: cssURL, wantCode: http.StatusOK, wantType: "text/css; charset=utf-8", wantCaching: "public, max-age=31536000, immutable"},
		{name: "stale-hash", target: "/static/site.css?v=000000000000", wantCode: http.StatusOK, wantType: "text/css; charset=utf-8", wantCaching: "no-cache"},
		{name: "no-hash", target: "/static/fonts/Hoftype-Cardillac.woff2", wantCode: http.StatusOK, wantType: "font/woff2", wantCaching: "no-cache"},
		{name: "committed-file", target: URL("favicon.svg"), wantCode: http.StatusOK, wantType: "image/svg+xml", wantCaching: "public, max-age=31536000, immutable"},
		{name: "revalidate", target: cssURL, header: http.Header{"If-None-Match": {`"` + hash + `"`}}, wantCode: http.StatusNotModified, wantCaching: "public, max-age=31536000, immutable"},
		{name: "unknown", target: "/static/nope.css", wantCode: http.StatusNotFound},
		{name: "source-not-served", target: "/static/input.css", wantCode: http.StatusNotFound},
		{name: "traversal", target: "/static/../static.go", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.target, nil)
			for k, v := range tt.header {
				req.Header[k] = v
			}

			rec := httptest.NewRecorder()
			Handler().ServeHTTP(rec, req)

			require.Equal(t, tt.wantCode, rec.Code)

			if tt.wantType != "" {
				require.Equal(t, tt.wantType, rec.Header().Get("Content-Type"))
				require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
			}

			if tt.wantCaching != "" {
				require.Equal(t, tt.wantCaching, rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestURLPanicsOnUnknownFile(t *testing.T) {
	require.Panics(t, func() { URL("nope.css") })
}

func TestLoad(t *testing.T) {
	const css = `@font-face{src:url(/static/fonts/a.woff2)format("woff2")}`

	t.Run("rewrites-css-urls", func(t *testing.T) {
		got, err := load(fstest.MapFS{
			"dist/site.css":      {Data: []byte(css)},
			"dist/fonts/a.woff2": {Data: []byte("font")},
		})
		require.NoError(t, err)
		require.Equal(t,
			`@font-face{src:url(/static/fonts/a.woff2?v=`+digest([]byte("font"))+`)format("woff2")}`,
			string(got["site.css"].data))
		require.Equal(t, digest(got["site.css"].data), got["site.css"].hash)
	})

	failures := []struct {
		name    string
		fsys    fstest.MapFS
		wantErr string
	}{
		{
			name:    "no-css",
			fsys:    fstest.MapFS{"files/a.png": {}},
			wantErr: "go generate ./static",
		},
		{
			name:    "css-names-missing-file",
			fsys:    fstest.MapFS{"dist/site.css": {Data: []byte(css)}},
			wantErr: "fonts/a.woff2",
		},
		{
			name:    "shadowed",
			fsys:    fstest.MapFS{"files/site.css": {}, "dist/site.css": {}},
			wantErr: "both files/ and dist/",
		},
		{
			name:    "unknown-type",
			fsys:    fstest.MapFS{"dist/site.css": {}, "files/a.gif": {}},
			wantErr: `".gif"`,
		},
	}

	for _, tt := range failures {
		t.Run(tt.name, func(t *testing.T) {
			_, err := load(tt.fsys)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
