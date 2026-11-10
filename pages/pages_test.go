package pages

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

const testBase = "https://aspargesgaarden.no"

var testSite = Site{BaseURL: testBase, Menu: Menu(ui.MenuItem{Label: "Julemarked", Href: "/julemarked"})}

// rendered is every page this package builds, by address ("404" for NotFound).
func rendered(t *testing.T) map[string]string {
	t.Helper()

	out := map[string]string{"404": NotFound(testSite).Render()}
	for path, page := range Fixed {
		out[path] = page(testSite).Render()
	}

	return out
}

func TestMenu(t *testing.T) {
	labels := make([]string, 0, 6)
	for _, m := range Menu(ui.MenuItem{Label: "Julemarked", Href: "/julemarked"}) {
		labels = append(labels, m.Label)
	}

	require.Equal(t, []string{"Muligheter", "Arrangementer", "Julemarked", "Galleri", "Om gården", "Kontakt"}, labels)
	require.Len(t, Menu(), 5)
}

func TestFixedPagesAreCanonicalAtTheirAddress(t *testing.T) {
	for path, page := range Fixed {
		require.Contains(t, page(testSite).Render(), `<link href="`+testBase+path+`" rel="canonical">`, path)
	}
}

func TestEveryPageHasOneH1(t *testing.T) {
	for path, page := range rendered(t) {
		require.Equal(t, 1, strings.Count(page, "<h1"), "%s must have exactly one <h1>", path)
	}
}

// Photos go through ui.Picture (srcset, sizes, alt, dimensions); a page that
// builds <img>, <picture> or <source> itself bypasses all of that.
func TestPagesBuildNoRawImages(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)

	banned := map[string]bool{"Img": true, "Picture": true, "Source": true}
	rawImg := regexp.MustCompile(`(?i)<\s*(img|picture|source)\b`)

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		require.NoError(t, err)

		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && id.Name == "elem" && banned[n.Sel.Name] {
					t.Errorf("%s: elem.%s; show photos with ui.Picture (or a ui component built on it)", f, n.Sel.Name)
				}
			case *ast.BasicLit:
				if n.Kind == token.STRING && rawImg.MatchString(n.Value) {
					t.Errorf("%s: raw image markup in %s; use ui.Picture", f, n.Value)
				}
			}

			return true
		})
	}
}

// Everything a page loads is built into the binary, apart from the named
// outside services in ui/external.go. Links a reader follows are not loads.
func TestPagesLoadNothingFromOutside(t *testing.T) {
	for path, page := range rendered(t) {
		doc, err := html.Parse(strings.NewReader(page))
		require.NoError(t, err)

		for n := range doc.Descendants() {
			if n.Type != html.ElementNode {
				continue
			}

			for _, load := range loads(n) {
				require.True(t, ui.ExternalLoadAllowed(load.directive, load.ref),
					"%s: <%s> loads %s (%s); serve it from /static/ or add the origin to ui/external.go", path, n.Data, load.ref, load.directive)
			}

			require.Empty(t, attr(n, "style"), "%s: inline style on <%s> (CSP style-src 'self' blocks it)", path, n.Data)
		}
	}
}

type load struct{ directive, ref string }

func loads(n *html.Node) []load {
	var out []load

	directive := "img-src"

	switch n.Data {
	case "script":
		directive = "script-src"
	case "iframe":
		directive = "frame-src"
	case "link":
		switch rel := attr(n, "rel"); {
		case rel == "canonical" || rel == "alternate":
			return nil
		case rel == "stylesheet":
			directive = "style-src"
		case rel == "preload" && attr(n, "as") == "font":
			directive = "font-src"
		}

		out = append(out, load{directive, attr(n, "href")})
	}

	for _, a := range n.Attr {
		switch a.Key {
		case "src", "poster", "data":
			out = append(out, load{directive, a.Val})
		case "srcset":
			for c := range strings.SplitSeq(a.Val, ",") {
				out = append(out, load{directive, strings.Fields(c)[0]})
			}
		}
	}

	return out
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}

	return ""
}

// The stylesheet must not pull anything from outside either: no @import of
// a web font service, no url() to a CDN.
func TestStylesheetLoadsNothingFromOutside(t *testing.T) {
	rec := httptest.NewRecorder()
	static.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, static.URL("site.css"), nil))
	require.Equal(t, http.StatusOK, rec.Code)

	css := rec.Body.String()
	require.NotContains(t, css, "@import", "the built stylesheet must be self-contained")

	urls := regexp.MustCompile(`url\(([^)]*)\)`).FindAllStringSubmatch(css, -1)
	require.NotEmpty(t, urls, "expected the @font-face urls")

	for _, m := range urls {
		ref := strings.Trim(m[1], `'"`)
		require.True(t, ui.ExternalLoadAllowed("font-src", ref), "site.css loads %s", ref)
	}
}
