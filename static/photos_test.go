package static

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
)

func tinyJPEG(t *testing.T, w, h int) []byte {
	t.Helper()

	var buf bytes.Buffer

	err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil)
	require.NoError(t, err)

	return buf.Bytes()
}

func TestIndexPhotos(t *testing.T) {
	got, err := indexPhotos(map[string]asset{
		"site.css":                    {hash: "c"},
		"photos/a/two-part-800.jpeg":  {hash: "j8", data: tinyJPEG(t, 8, 10)},
		"photos/a/two-part-480.jpeg":  {hash: "j4", data: tinyJPEG(t, 5, 6)},
		"photos/a/two-part-800.avif":  {hash: "a8"},
		"photos/a/two-part-480.webp":  {hash: "w4"},
		"photos/other-480.jpeg":       {hash: "o4", data: tinyJPEG(t, 4, 3)},
		"fonts/not-a-photo-400.woff2": {hash: "f"},
	})
	require.NoError(t, err)

	want := map[string]images.Ref{
		"a/two-part": {
			// From the largest JPEG, not the first one walked.
			Width: 8, Height: 10,
			Variants: []images.Variant{
				{Format: images.AVIF, Width: 800, URL: "/static/photos/a/two-part-800.avif?v=a8"},
				{Format: images.JPEG, Width: 480, URL: "/static/photos/a/two-part-480.jpeg?v=j4"},
				{Format: images.JPEG, Width: 800, URL: "/static/photos/a/two-part-800.jpeg?v=j8"},
				{Format: images.WebP, Width: 480, URL: "/static/photos/a/two-part-480.webp?v=w4"},
			},
		},
		"other": {
			Width: 4, Height: 3,
			Variants: []images.Variant{{Format: images.JPEG, Width: 480, URL: "/static/photos/other-480.jpeg?v=o4"}},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("indexPhotos mismatch (-want +got):\n%s", diff)
	}
}

func TestIndexPhotosRejects(t *testing.T) {
	for name, a := range map[string]map[string]asset{
		"no-width":   {"photos/x.jpeg": {}},
		"bad-width":  {"photos/x-big.jpeg": {}},
		"no-jpeg":    {"photos/x-480.avif": {}},
		"broken-jpg": {"photos/x-480.jpeg": {data: []byte("nope")}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := indexPhotos(a)
			require.Error(t, err)
		})
	}
}

// The embedded photos the fixed pages use, in every format Picture offers.
func TestPhoto(t *testing.T) {
	ref := Photo("forside")
	require.Equal(t, 2500, ref.Width)

	formats := map[images.Format]bool{}
	for _, v := range ref.Variants {
		formats[v.Format] = true
	}

	require.Equal(t, map[images.Format]bool{images.AVIF: true, images.WebP: true, images.JPEG: true}, formats)
	require.Panics(t, func() { Photo("nope") })
}
