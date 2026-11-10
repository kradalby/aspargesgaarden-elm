package images

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testSHA = "3fa91c2e5b8d4f6a7c9e0b1d2f3a4b5c6d7e8f9011223344556677889900aabb"

func TestSpecNames(t *testing.T) {
	r := Rect{X: 700, Y: 150, W: 480, H: 600}
	s := VariantSpec{Photo: testSHA, Kind: Card, Crop: r.ID(), Width: 480, Format: AVIF}

	v := fmt.Sprintf("v%d", PipelineVersion)
	require.Len(t, r.ID(), 8)
	require.Equal(t, v+"/"+testSHA+"/card-"+r.ID()+"-480.avif", s.Name())
	require.Equal(t, "/img/"+s.Name(), s.URL())
	require.Equal(t,
		filepath.Join("variants", v, "3f", testSHA, "card-"+r.ID()+"-480.avif"),
		s.Path())

	s.Format = JPEG
	require.True(t, strings.HasSuffix(s.URL(), "-480.jpg"), s.URL())
}

func TestRectIDChangesWithCrop(t *testing.T) {
	a := Rect{X: 0, Y: 0, W: 800, H: 1000}
	b := a
	b.X = 1
	require.Equal(t, a.ID(), Rect{W: 800, H: 1000}.ID())
	require.NotEqual(t, a.ID(), b.ID())
}

func TestParseURLRoundTrip(t *testing.T) {
	crop := Rect{X: 1, Y: 2, W: 1600, H: 900}.ID()
	for _, k := range kinds {
		for _, f := range formats {
			for _, w := range widths {
				s := VariantSpec{Photo: testSHA, Kind: k, Crop: crop, Width: w, Format: f}
				got, err := ParseURL(s.URL())
				require.NoError(t, err, s.URL())
				require.Equal(t, s, got)
			}
		}
	}
}

func TestParseURLRejects(t *testing.T) {
	good := VariantSpec{Photo: testSHA, Kind: Card, Crop: "0123abcd", Width: 800, Format: WebP}.URL()
	_, err := ParseURL(good)
	require.NoError(t, err)

	bad := []string{
		"",
		"/img/",
		strings.Replace(good, fmt.Sprintf("/v%d/", PipelineVersion), fmt.Sprintf("/v%d/", PipelineVersion+1), 1),
		strings.Replace(good, "/v", "/v0", 1),
		strings.Replace(good, "3fa9", "3FA9", 1),
		strings.Replace(good, testSHA, testSHA[:63], 1),
		strings.Replace(good, "card-", "poster-", 1),
		strings.Replace(good, "0123abcd", "0123abc", 1),
		strings.Replace(good, "-800.", "-0800.", 1),
		strings.Replace(good, "-800.", "-.", 1),
		strings.Replace(good, ".webp", ".jpeg", 1),
		strings.Replace(good, ".webp", ".png", 1),
		good + "/",
		good + "?x=1",
		strings.Replace(good, "/img/", "/img//", 1),
		strings.Replace(good, "/img/", "/img/../", 1),
		strings.Replace(good, "/img/", "/static/", 1),
	}
	for _, u := range bad {
		_, err := ParseURL(u)
		require.ErrorIs(t, err, ErrNotFound, u)
	}
}
