package seo

import (
	"strings"
	"testing"

	"github.com/chasefleming/elem-go"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

const testBase = "https://aspargesgaarden.no"

// testPhoto is a w×h photo with the import's centred crops for every kind
// it is large enough for.
func testPhoto(c byte, w, h int) site.Photo {
	p := site.Photo{
		ID: int64(c), SHA256: strings.Repeat(string(c), 64), MIME: "image/jpeg", Width: w, Height: h,
		Alt: "Bålfat på tunet", Crops: map[images.Kind]images.Rect{},
	}

	for _, k := range images.CropKinds {
		r, err := images.DefaultCrop(k, w, h)
		if err != nil {
			continue // too small for that use, as the import leaves it
		}

		p.Crops[k] = r
	}

	return p
}

func render(nodes []elem.Node) string {
	var b strings.Builder
	for _, n := range nodes {
		b.WriteString(n.Render())
	}

	return b.String()
}

func TestTags(t *testing.T) {
	img, ok := PhotoImage(testPhoto('a', 2400, 1600))
	require.True(t, ok)

	golden.HTML(t, "tags", render(Tags(testBase, Meta{
		Title: "Julemarked – 14. november 2026", Description: "Mer enn 30 utstillere.",
		Path: "/arrangementer/2026/julemarked", Image: img,
		JSONLD: []any{WebSiteNode(testBase)},
	})))
}

func TestTagsWithoutImageUseTheDefault(t *testing.T) {
	got := render(Tags(testBase, Meta{Title: "Om gården", Description: "Om oss.", Path: "/om-oss"}))

	require.Contains(t, got, `<meta content="https://aspargesgaarden.no/static/share.jpg?v=`)
	require.Contains(t, got, `<meta content="1200" property="og:image:width">`)
	require.Contains(t, got, `<meta content="630" property="og:image:height">`)
	require.NotContains(t, got, "ld+json", "no structured data unless the page has some")
}

// The beta site's header says noindex; a robots tag beside it would only
// muddle that.
func TestTagsRobots(t *testing.T) {
	require.Contains(t, render(Tags(testBase, Meta{Title: "Kontakt"})), `<meta content="max-image-preview:large" name="robots">`)
	require.NotContains(t, render(Tags(testBase, Meta{Title: "Kontakt", NoIndex: true})), "robots")
}

func TestTagsWithoutPathHaveNoAddress(t *testing.T) {
	got := render(Tags(testBase, Meta{Title: "Fant ikke siden", Description: "Finnes ikke."}))

	require.NotContains(t, got, "canonical")
	require.NotContains(t, got, "og:url")
}

func TestFullTitle(t *testing.T) {
	require.Equal(t, "Kontakt | Aspargesgården", Meta{Title: "Kontakt"}.FullTitle())
}

func TestClip(t *testing.T) {
	require.Equal(t, "kort", Clip("kort", 10))
	require.Equal(t, "Mer enn 30…", Clip("Mer enn 30 utstillere", 15))
	require.LessOrEqual(t, len([]rune(Clip(strings.Repeat("å ", 200), 160))), 160)
}

func TestPhotoImage(t *testing.T) {
	tests := []struct {
		name          string
		photo         site.Photo
		wantKind      string
		width, height int
	}{
		{name: "share crop", photo: testPhoto('a', 2400, 1600), wantKind: "/share-", width: 1200, height: 630},
		// Too small for the share image: the banner's largest JPEG instead.
		{name: "small", photo: testPhoto('b', 1000, 800), wantKind: "/banner-", width: 1000, height: 562},
		{name: "no crops", photo: func() site.Photo { p := testPhoto('c', 1600, 1200); p.Crops = nil; return p }(), wantKind: "/banner-", width: 1600, height: 900},
		// Under the banner's MinWidth too: its soft centred banner, not the portrait.
		{name: "small portrait", photo: testPhoto('d', 768, 1024), wantKind: "/banner-", width: 768, height: 432},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, ok := PhotoImage(tt.photo)
			require.True(t, ok)
			require.Contains(t, img.URL, tt.wantKind)
			require.True(t, strings.HasSuffix(img.URL, ".jpg"), img.URL)
			require.Equal(t, [2]int{tt.width, tt.height}, [2]int{img.Width, img.Height})
			require.Equal(t, "Bålfat på tunet", img.Alt)
		})
	}
}

// A poster is shared whole in place of the main photo's share crop, tall or
// wide; the JSON-LD leads with it and keeps the main photo's other shapes.
func TestEventImagePoster(t *testing.T) {
	e := testEvent()

	img, ok := EventImage(e)
	require.True(t, ok)
	require.Contains(t, img.URL, "/share-", "without a poster, the main photo's share crop")

	poster := testPhoto('f', 1600, 2200)
	poster.Alt = "Plakat for julemarkedet"
	e.Poster = &poster

	img, ok = EventImage(e)
	require.True(t, ok)
	require.Contains(t, img.URL, "/"+strings.Repeat("f", 64)+"/full-")
	require.Equal(t, Image{URL: img.URL, Width: 1600, Height: 2200, Alt: "Plakat for julemarkedet"}, img)

	ev := EventNodes(testBase, EventInput{Event: e})[0].(Event) //nolint:forcetypeassert // fails the test otherwise
	require.Equal(t, testBase+img.URL, ev.Image[0])
	require.Len(t, ev.Image, 3, "poster, banner and card")
}
