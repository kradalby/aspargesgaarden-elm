package static

import (
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
)

const testManifest = `{"a": {"Width": 800, "Height": 600, "Variants": [
  {"Format": "avif", "Width": 480, "URL": "/static/photos/v1/s/full-c-480.avif"},
  {"Format": "jpeg", "Width": 800, "URL": "/static/photos/v1/s/full-c-800.jpg"}]}}`

func TestIndexPhotos(t *testing.T) {
	fsys := fstest.MapFS{"dist/" + photoManifest: {Data: []byte(testManifest)}}
	a := map[string]asset{
		"photos/v1/s/full-c-480.avif": {hash: "a4"},
		"photos/v1/s/full-c-800.jpg":  {hash: "j8"},
	}

	got, err := indexPhotos(fsys, a)
	require.NoError(t, err)

	want := map[string]images.Ref{"a": {
		Width: 800, Height: 600,
		Variants: []images.Variant{
			{Format: images.AVIF, Width: 480, URL: "/static/photos/v1/s/full-c-480.avif?v=a4"},
			{Format: images.JPEG, Width: 800, URL: "/static/photos/v1/s/full-c-800.jpg?v=j8"},
		},
	}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("indexPhotos mismatch (-want +got):\n%s", diff)
	}

	// A manifest naming a file that is not embedded fails at start.
	delete(a, "photos/v1/s/full-c-800.jpg")

	_, err = indexPhotos(fsys, a)
	require.ErrorContains(t, err, "full-c-800.jpg is not embedded")

	_, err = indexPhotos(fstest.MapFS{}, a)
	require.ErrorContains(t, err, "go generate ./static")
}

// The embedded photos the fixed pages use, in every format Picture offers.
func TestPhoto(t *testing.T) {
	for _, name := range []string{"forside", "loven", "fjoset", "kjokken", "lysthus", "skafferiet"} {
		// The ladder stops below each master's width.
		ref := Photo(name)
		require.GreaterOrEqual(t, ref.Width, 1600, name)

		formats := map[images.Format]bool{}
		for _, v := range ref.Variants {
			formats[v.Format] = true
		}

		require.Equal(t, map[images.Format]bool{images.AVIF: true, images.WebP: true, images.JPEG: true}, formats, name)
	}

	require.Panics(t, func() { Photo("nope") })
}
