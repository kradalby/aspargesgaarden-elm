package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
	"github.com/kradalby/asparges/images"
)

// testPhoto is a 2:3 photo in the given formats (default all), its
// variants listed out of order.
func testPhoto(formats ...images.Format) images.Ref {
	if formats == nil {
		formats = []images.Format{images.JPEG, images.AVIF, images.WebP}
	}

	ref := images.Ref{Width: 1600, Height: 2400}

	for _, f := range formats {
		for _, w := range []int{1600, 480, 800, 1200} {
			ref.Variants = append(ref.Variants, images.Variant{Format: f, Width: w, URL: "/img/x-" + strconv.Itoa(w) + "." + string(f)})
		}
	}

	return ref
}

func TestPicture(t *testing.T) {
	golden.HTML(t, "picture-lazy", Picture(PictureProps{
		Photo: testPhoto(), Alt: "Låven", Layout: LayoutRowPhoto, Class: "w-full",
	}).Render())
	golden.HTML(t, "picture-hero", Picture(PictureProps{
		Photo: testPhoto(), Decorative: true, Layout: LayoutFullscreen, Priority: Hero,
	}).Render())
}

func TestPictureWithoutModernFormats(t *testing.T) {
	got := Picture(PictureProps{Photo: testPhoto(images.JPEG), Alt: "a", Layout: LayoutFullscreen}).Render()

	require.NotContains(t, got, "<source")
	require.Contains(t, got, `src="/img/x-1200.jpeg"`)
}

func TestPictureRefusesIncompleteProps(t *testing.T) {
	for name, p := range map[string]PictureProps{
		"no-alt":         {Photo: testPhoto(), Layout: LayoutFullscreen},
		"alt-decorative": {Photo: testPhoto(), Alt: "a", Decorative: true, Layout: LayoutFullscreen},
		"no-layout":      {Photo: testPhoto(), Alt: "a"},
		"no-variants":    {Alt: "a", Layout: LayoutFullscreen},
	} {
		t.Run(name, func(t *testing.T) {
			require.Panics(t, func() { Picture(p) })
		})
	}
}

// layoutKinds are the photo kinds the pages show in each layout. Fullscreen
// shows the site's own photos only, which no admin rule covers.
var layoutKinds = map[Layout][]images.Kind{
	LayoutFullscreen: nil,
	LayoutRowPhoto:   {images.Card},
	LayoutBanner:     {images.Banner},
	LayoutCardGrid:   {images.Card},
	LayoutHalfColumn: {images.Full},
	LayoutGallery:    {images.Full},
}

func isLayout(l Layout) (ok bool) {
	defer func() { ok = recover() == nil }()
	l.sizes(images.Ref{Width: 1, Height: 1})

	return true
}

// cssPixels reads a sizes length (Nrem, Nvw) at a viewport vw pixels wide.
func cssPixels(t *testing.T, s string, vw int) float64 {
	t.Helper()

	unit := map[string]float64{"rem": 16, "vw": float64(vw) / 100}
	for suffix, px := range unit {
		if n, ok := strings.CutSuffix(s, suffix); ok {
			f, err := strconv.ParseFloat(n, 64)
			require.NoError(t, err, s)

			return f * px
		}
	}

	t.Fatalf("unknown length %q", s)

	return 0
}

// drawn is the width sizes gives at a viewport vw pixels wide, read as a
// browser does: the first entry whose (min-width) holds.
func drawn(t *testing.T, sizes string, vw int) float64 {
	t.Helper()

	for entry := range strings.SplitSeq(sizes, ", ") {
		cond, length, ok := strings.Cut(entry, ") ")
		if !ok {
			return cssPixels(t, entry, vw)
		}

		if float64(vw) >= cssPixels(t, strings.TrimPrefix(cond, "(min-width: "), vw) {
			return cssPixels(t, length, vw)
		}
	}

	t.Fatalf("%q has no size at %d px", sizes, vw)

	return 0
}

// The admin calls a crop soft below its kind's SharpWidth, the widest box a
// page draws that kind in. A layout drawing a kind wider, at any viewport,
// would make that rule lie; widen both or neither.
func TestLayoutsWithinSharpWidth(t *testing.T) {
	widest := map[images.Kind]float64{}

	for l := Layout(1); isLayout(l); l++ {
		kinds, ok := layoutKinds[l]
		require.True(t, ok, "layout %d: add it to layoutKinds", l)

		for _, k := range kinds {
			for vw := 320; vw <= 3840; vw++ {
				w := drawn(t, l.sizes(images.Ref{}), vw)
				require.LessOrEqual(t, w, float64(k.SharpWidth()), "layout %d draws %s %.0f px wide at a %d px viewport", l, k, w, vw)
				widest[k] = max(widest[k], w)
			}
		}
	}

	// Where the pages draw a kind themselves the bound is tight (to the
	// pixel below a breakpoint), so the admin never warns about a photo that
	// is sharp everywhere.
	require.InDelta(t, images.Card.SharpWidth(), widest[images.Card], 1)
	require.InDelta(t, images.Banner.SharpWidth(), widest[images.Banner], 1)
}

// Full's widest box is the lightbox's, which elm/gallery sizes; read it from
// there so the two cannot drift.
func TestFullSharpWidthIsTheLightbox(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "elm", "gallery", "src", "Lightbox.elm"))
	require.NoError(t, err)

	m := regexp.MustCompile(`(?m)^widest =\n +([0-9]+)$`).FindSubmatch(src)
	require.NotNil(t, m, "Lightbox.elm no longer names its bound widest")
	require.Equal(t, strconv.Itoa(images.Full.SharpWidth()), string(m[1]))
}
