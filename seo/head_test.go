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
		Alt: "Bålfat på tunet", Crops: map[site.CropKind]site.Rect{},
	}

	for _, k := range images.CropKinds {
		r, err := images.DefaultCrop(k, w, h)
		if err != nil {
			continue // too small for that use, as the import leaves it
		}

		p.Crops[site.CropKind(k)] = site.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
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
		{name: "small", photo: testPhoto('b', 1000, 800), wantKind: "/banner-", width: 800, height: 450},
		{name: "no crops", photo: func() site.Photo { p := testPhoto('c', 1600, 1200); p.Crops = nil; return p }(), wantKind: "/full-", width: 1600, height: 1200},
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
