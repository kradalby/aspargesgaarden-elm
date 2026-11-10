package ui

import (
	"strconv"
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
