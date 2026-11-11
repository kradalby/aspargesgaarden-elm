package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/kradalby/asparges/static"

	"github.com/chasefleming/elem-go"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func testPage(path string) string {
	return Page(PageProps{
		BaseURL: "https://aspargesgaarden.no",
		Meta:    Meta{Title: "Selskapslokaler", Description: "Lokaler & mat", Path: path},
		Menu:    testMenu,
		Umami:   testUmami,
	}, Container(Headline("Selskapslokaler"), elem.P(nil, elem.Text("Tekst")))).Render()
}

func TestPage(t *testing.T) {
	golden.HTML(t, "page", testPage("/lokaler"))
}

func TestPageWithoutAddressHasNoCanonical(t *testing.T) {
	require.NotContains(t, testPage(""), `rel="canonical"`)
}

// The lightbox script loads once on a page with galleries and not at all on
// one without, whatever the page author wrote.
func TestPageLoadsLightboxOnlyWithGallery(t *testing.T) {
	script := `<script defer src="` + static.URL("gallery.js") + `"></script>`
	page := func(body ...elem.Node) string {
		return Page(PageProps{BaseURL: "https://aspargesgaarden.no", Meta: Meta{Title: "x"}}, body...).Render()
	}

	require.NotContains(t, testPage("/"), "gallery.js")
	require.NotContains(t, page(Gallery(GalleryPage)), "gallery.js", "an empty gallery has nothing to open")
	require.Equal(t, 1, strings.Count(page(Container(Gallery(GalleryPage, testGallery...), Gallery(GalleryColumn, testGallery[:1]...))), script))
}

// The display font is preloaded only where a heading uses it.
func TestPagePreloadsOnlyFontsInUse(t *testing.T) {
	preloads := func(body ...elem.Node) int {
		return strings.Count(Page(PageProps{BaseURL: "https://aspargesgaarden.no", Meta: Meta{Title: "x"}}, body...).Render(), `rel="preload"`)
	}
	all := static.FontURLs()

	require.True(t, slices.ContainsFunc(all, func(u string) bool { return strings.Contains(u, "Cardillac") }),
		"Page tells the display font by its file name")
	require.Equal(t, len(all), preloads(Container(Headline("Galleri"))))
	require.Equal(t, len(all)-1, preloads(Backdrop("Forside", testPhoto())))
}

// The old site rendered the Umami tag as an empty <p>, so it never loaded;
// then the rebuild copied v1's /umami.js, which the v2 server no longer has.
func TestPageLoadsUmami(t *testing.T) {
	require.Contains(t, testPage("/"),
		`<script data-domains="aspargesgaarden.no" data-website-id="00000000-0000-4000-8000-000000000000" defer src="https://umami.kradalby.no/script.js"></script>`)
}

// No site to report to, no tracker: it would only log rejected beacons.
func TestPageWithoutUmami(t *testing.T) {
	page := Page(PageProps{BaseURL: "https://aspargesgaarden.no", Meta: Meta{Title: "x"}}).Render()
	require.NotContains(t, page, "umami")
	require.NotContains(t, page, "data-website-id")
}

// The tracker posts to /api/send next to its own address (Umami v2, unless
// data-host-url says otherwise); CSP must let both through, and only them.
func TestUmamiFitsTheCSP(t *testing.T) {
	beacon := umamiScript[:strings.LastIndex(umamiScript, "/")] + "/api/send"

	require.True(t, ExternalLoadAllowed(testUmami, "script-src", umamiScript))
	require.True(t, ExternalLoadAllowed(testUmami, "connect-src", beacon))
	require.Contains(t, CSP(testUmami), "script-src 'self' "+UmamiOrigin+";")
	require.Contains(t, CSP(testUmami), "connect-src 'self' "+UmamiOrigin+";")

	// Off, the policy holds no Umami origin at all.
	require.NotContains(t, CSP(Umami{}), UmamiOrigin)
	require.False(t, ExternalLoadAllowed(Umami{}, "script-src", umamiScript))
	require.NotContains(t, testPage("/"), "data-host-url", "a beacon host elsewhere would need its own connect-src")
}
