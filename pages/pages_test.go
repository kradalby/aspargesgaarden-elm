package pages

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/kradalby/asparges/booking"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

const testBase = "https://aspargesgaarden.no"

// testUmami is analytics switched on, as the live site has it.
var testUmami = ui.Umami{SiteID: "00000000-0000-4000-8000-000000000000", Domain: "aspargesgaarden.no"}

var testSite = Site{
	BaseURL: testBase,
	Menu:    Menu(site.MenuEntry{Label: "Julemarked", Path: "/arrangementer/2026/julemarked"}),
	Umami:   testUmami,
}

// rendered is every page this package builds, by address ("404" for NotFound).
func rendered(t *testing.T, s Site) map[string]string {
	t.Helper()

	out := map[string]string{
		"404":                    NotFound(s).Render(),
		"/galleri":               Galleri(s, testGalleryPhotos).Render(),
		"/arrangementer":         Events(s, testEvents(), testNow).Render(),
		"/arrangementer (empty)": Events(s, nil, testNow).Render(),
	}
	for path, page := range Fixed {
		out[path] = page(s).Render()
	}

	for _, e := range testEvents() {
		out[e.Path()] = Event(s, e, testNow).Render()
	}

	withCalendar := s
	withCalendar.Calendar = booking.NewCalendar(booking.Days{site.Today(testNow).AddDays(3): true}, site.Today(testNow))
	out["/kontakt (calendar)"] = Kontakt(withCalendar).Render()

	return out
}

func TestMenu(t *testing.T) {
	got := Menu(site.MenuEntry{Label: "Julemarked", Path: "/arrangementer/2026/julemarked"})

	labels := make([]string, 0, len(got))
	for _, m := range got {
		labels = append(labels, m.Label)
	}

	require.Equal(t, []string{"Lokaler", "Arrangementer", "Julemarked", "Galleri", "Om gården", "Kontakt"}, labels)
	require.Equal(t, "/arrangementer/2026/julemarked", got[2].Href)
	require.Len(t, Menu(), 5)

	// llms.txt names the menu's own pages by their titles.
	for _, m := range Menu() {
		require.NotEmpty(t, Titles[m.Href], m.Href)
	}

	for path, page := range Fixed {
		require.Contains(t, page(testSite).Render(), "<title>"+Titles[path]+" | ", path)
	}
}

func TestFixedPagesAreCanonicalAtTheirAddress(t *testing.T) {
	for path, page := range Fixed {
		require.Contains(t, page(testSite).Render(), `<link href="`+testBase+path+`" rel="canonical">`, path)
	}
}

func TestEveryPageHasOneH1(t *testing.T) {
	for path, page := range rendered(t, testSite) {
		require.Equal(t, 1, strings.Count(page, "<h1"), "%s must have exactly one <h1>", path)
	}
}

// head is what a page's <head> says about it.
type head struct {
	titles   []string
	meta     map[string][]string // by name or property
	links    map[string]string   // by rel
	ldTypes  []string
	ldBlocks int
}

func readHead(t *testing.T, page string) head {
	t.Helper()

	doc, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)

	h := head{meta: map[string][]string{}, links: map[string]string{}}

	for n := range doc.Descendants() {
		if n.Type != html.ElementNode || n.Parent == nil || n.Parent.Data != "head" {
			continue
		}

		switch n.Data {
		case "title":
			h.titles = append(h.titles, n.FirstChild.Data)
		case "meta":
			key := attr(n, "name") + attr(n, "property")
			h.meta[key] = append(h.meta[key], attr(n, "content"))
		case "link":
			h.links[attr(n, "rel")] = attr(n, "href")
		case "script":
			if attr(n, "type") != "application/ld+json" {
				continue
			}

			h.ldBlocks++

			var ld struct {
				Graph []struct {
					Type any `json:"@type"`
				} `json:"@graph"`
			}
			require.NoError(t, json.Unmarshal([]byte(n.FirstChild.Data), &ld))

			for _, g := range ld.Graph {
				h.ldTypes = append(h.ldTypes, fmt.Sprint(g.Type))
			}
		}
	}

	return h
}

// The SEO checklist, page by page: a page that forgets its title, its
// description or its share image fails here.
func TestEveryPageDescribesItself(t *testing.T) {
	titles, descriptions := map[string]string{}, map[string]string{}

	for path, page := range rendered(t, testSite) {
		h := readHead(t, page)

		require.Len(t, h.titles, 1, "%s: one <title>", path)
		title := h.titles[0]
		require.True(t, strings.HasSuffix(title, " | Aspargesgården"), "%s: %q names the site last", path, title)

		for _, key := range []string{
			"description", "og:type", "og:site_name", "og:locale", "og:title", "og:description",
			"og:image", "og:image:type", "og:image:width", "og:image:height", "og:image:alt", "twitter:card",
		} {
			require.Len(t, h.meta[key], 1, "%s: one %s", path, key)
			require.NotEmpty(t, h.meta[key][0], "%s: empty %s", path, key)
		}

		require.Equal(t, "nb_NO", h.meta["og:locale"][0])
		require.Equal(t, "summary_large_image", h.meta["twitter:card"][0])
		require.Regexp(t, `^`+testBase+`/(static|img)/`, h.meta["og:image"][0], "%s: share images are ours", path)
		require.LessOrEqual(t, h.ldBlocks, 1, "%s: one JSON-LD block", path)

		if path == "404" {
			require.Empty(t, h.links["canonical"])
			continue
		}

		// "/arrangementer (empty)": the same page in another state.
		address, _, variant := strings.Cut(path, " (")
		require.Equal(t, testBase+address, h.links["canonical"], path)
		require.Equal(t, h.links["canonical"], h.meta["og:url"][0], path)

		description := h.meta["description"][0]
		require.LessOrEqual(t, len([]rune(description)), 160, "%s: %q is cut in results", path, description)

		if variant {
			continue
		}

		// An event's words are the editor's; the other pages' are ours.
		if _, fixed := Fixed[path]; fixed || path == "/arrangementer" {
			require.LessOrEqual(t, len([]rune(title)), 65, "%s: %q is cut in results", path, title)
			require.GreaterOrEqual(t, len([]rune(description)), 70, "%s: %q says too little", path, description)
		}

		require.NotContains(t, titles, title, "%s has the title of %s", path, titles[title])
		require.NotContains(t, descriptions, description, "%s has the description of %s", path, descriptions[description])
		titles[title], descriptions[description] = path, path
	}
}

func TestStructuredData(t *testing.T) {
	pages := rendered(t, testSite)

	for _, path := range []string{"/", "/kontakt"} {
		require.Contains(t, readHead(t, pages[path]).ldTypes, "[LocalBusiness EventVenue]", path)
	}

	require.Contains(t, readHead(t, pages["/"]).ldTypes, "WebSite")

	for _, e := range testEvents() {
		require.Contains(t, readHead(t, pages[e.Path()]).ldTypes, "Event", e.Path())
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

// The same rule on the rendered pages, which also covers ui/: every photo is
// an <img> in a <picture> with its sources, whichever component drew it.
func TestEveryPhotoIsAPicture(t *testing.T) {
	isPhoto := func(src string) bool {
		return strings.HasPrefix(src, "/img/") || strings.HasPrefix(src, static.Prefix+"photos/")
	}

	photos := 0

	for path, page := range rendered(t, testSite) {
		doc, err := html.Parse(strings.NewReader(page))
		require.NoError(t, err)

		for n := range doc.Descendants() {
			if n.Type != html.ElementNode || n.Data != "img" || !isPhoto(attr(n, "src")) {
				continue
			}

			photos++
			src := attr(n, "src")

			require.Equal(t, "picture", n.Parent.Data, "%s: %s is not in a <picture>", path, src)

			for _, key := range []string{"srcset", "sizes", "width", "height"} {
				require.NotEmpty(t, attr(n, key), "%s: %s has no %s", path, src, key)
			}

			require.True(t, slices.ContainsFunc(n.Attr, func(a html.Attribute) bool { return a.Key == "alt" }), "%s: %s has no alt", path, src)

			sources := 0

			for c := range n.Parent.ChildNodes() {
				if c.Type != html.ElementNode || c.Data != "source" {
					continue
				}

				sources++

				for _, key := range []string{"type", "srcset", "sizes"} {
					require.NotEmpty(t, attr(c, key), "%s: a <source> of %s has no %s", path, src, key)
				}
			}

			require.Positive(t, sources, "%s: %s has no <source>", path, src)
		}
	}

	require.Positive(t, photos, "the pages show photos, or this checks nothing")
}

// Everything a page loads is built into the binary, apart from the named
// outside services in ui/external.go. Links a reader follows are not loads.
func TestPagesLoadNothingFromOutside(t *testing.T) {
	// With analytics on and off: off, the tracker's origin is not allowed.
	for _, umami := range []ui.Umami{testUmami, {}} {
		s := testSite
		s.Umami = umami

		for path, page := range rendered(t, s) {
			doc, err := html.Parse(strings.NewReader(page))
			require.NoError(t, err)

			for n := range doc.Descendants() {
				if n.Type != html.ElementNode {
					continue
				}

				for _, load := range loads(n) {
					require.True(t, ui.ExternalLoadAllowed(umami, load.directive, load.ref),
						"%s: <%s> loads %s (%s); serve it from /static/ or add the origin to ui/external.go", path, n.Data, load.ref, load.directive)
				}

				require.Empty(t, attr(n, "style"), "%s: inline style on <%s> (CSP style-src 'self' blocks it)", path, n.Data)
			}
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
func staticFile(t *testing.T, name string) string {
	t.Helper()

	rec := httptest.NewRecorder()
	static.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, static.URL(name), nil))
	require.Equal(t, http.StatusOK, rec.Code)

	return rec.Body.String()
}

func TestStylesheetLoadsNothingFromOutside(t *testing.T) {
	css := staticFile(t, "site.css")
	require.NotContains(t, css, "@import", "the built stylesheet must be self-contained")

	urls := regexp.MustCompile(`url\(([^)]*)\)`).FindAllStringSubmatch(css, -1)
	require.NotEmpty(t, urls, "expected the @font-face urls")

	for _, m := range urls {
		ref := strings.Trim(m[1], `'"`)
		require.True(t, ui.ExternalLoadAllowed(ui.Umami{}, "font-src", ref), "site.css loads %s", ref)
	}
}

// The lightbox only shows the gallery's own images, and the calendar has
// its taken days in the page: neither requests anything itself, and the
// only addresses in them are links in Elm's error messages.
func TestPageScriptsLoadNothingFromOutside(t *testing.T) {
	for _, name := range []string{"gallery.js", "calendar.js"} {
		js := staticFile(t, name)

		for _, api := range []string{"XMLHttpRequest", "fetch(", "WebSocket", "EventSource", "sendBeacon", "import("} {
			require.NotContains(t, js, api, name)
		}

		for _, u := range regexp.MustCompile(`https?://[^\s"'\x60)]+`).FindAllString(js, -1) {
			require.Regexp(t, `^https://(github\.com/elm/|package\.elm-lang\.org/)`, u, "%s names an outside address", name)
		}
	}
}
