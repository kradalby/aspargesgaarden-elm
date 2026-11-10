package images

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// photoFromFixture describes a fixture as the catalogue would after Intake.
func photoFromFixture(t *testing.T, v *vips, name string, crops map[Kind]Rect) (Photo, string) {
	t.Helper()

	path := fixture(t, name)
	b, err := os.ReadFile(path)
	require.NoError(t, err)

	sum := sha256.Sum256(b)
	p := Photo{SHA256: hex.EncodeToString(sum[:]), MIME: sniff(b), Crops: crops}

	hdr, err := v.preflight(context.Background(), path, p.MIME)
	require.NoError(t, err)

	p.Width, p.Height = hdr.width, hdr.height

	return p, path
}

func buildAll(t *testing.T, v *vips, p Photo, orig, out string, only ...Kind) []string {
	t.Helper()

	var files []string
	for _, g := range groups(p) {
		if len(only) > 0 && g.kind != only[0] {
			continue
		}

		dst := func(s VariantSpec) string { return filepath.Join(out, s.Path()) }
		require.NoError(t, v.build(context.Background(), p, orig, g, g.specs, dst), g.kind)

		for _, s := range g.specs {
			require.FileExists(t, dst(s))
			files = append(files, dst(s))
		}
	}

	return files
}

func near(t *testing.T, got color.NRGBA, want [3]uint8, what string) {
	t.Helper()

	g := [3]uint8{got.R, got.G, got.B}
	for i := range 3 {
		require.InDelta(t, want[i], g[i], 12, "%s: got %v want %v", what, g, want)
	}
}

// TestBuildUpright cuts the manifest's example card from every accepted
// fixture: whatever the orientation, colour space or format, the markers
// must land where they are in the upright sRGB scene.
func TestBuildUpright(t *testing.T) {
	t.Parallel()
	v := newTestVips(t)

	for _, name := range []string{
		"upright.jpg", "orient2.jpg", "orient3.jpg", "orient4.jpg",
		"orient5.jpg", "orient6.jpg", "orient7.jpg", "orient8.jpg",
		"rotated.heic", "p3.jpg", "gps.jpg", "cmyk.jpg", "alpha.png",
		"rgb16.png", "upright.webp", "orient6.webp",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p, orig := photoFromFixture(t, v, name, map[Kind]Rect{Card: exampleCrop})
			files := buildAll(t, v, p, orig, t.TempDir(), Card)
			require.Len(t, files, 3)

			for _, f := range files {
				img := look(t, f)
				require.Equal(t, image.Rect(0, 0, 480, 600), img.Bounds(), f)

				// The built-in CMYK profile shifts colours; geometry is enough.
				if name == "cmyk.jpg" {
					continue
				}

				near(t, at(img, 180, 170), [3]uint8{255, 0, 255}, f+" magenta")
				near(t, at(img, 410, 550), [3]uint8{255, 255, 0}, f+" yellow")
				near(t, at(img, 410, 4), [3]uint8{0, 255, 0}, f+" green")
			}
		})
	}
}

func TestBuildAlpha(t *testing.T) {
	t.Parallel()
	v := newTestVips(t)
	p, orig := photoFromFixture(t, v, "alpha.png", map[Kind]Rect{Card: exampleCrop})

	for _, f := range buildAll(t, v, p, orig, t.TempDir(), Card) {
		// (10,250) is inside the transparent hole.
		px := at(look(t, f), 10, 250)
		if filepath.Ext(f) == ".jpg" {
			near(t, px, [3]uint8{255, 255, 255}, f+" flattened on white")
			continue
		}

		require.LessOrEqual(t, px.A, uint8(8), "%s keeps the hole transparent", f)
	}
}

// TestBuildStripsMetadata builds every kind of the GPS fixture: no output
// may carry EXIF, XMP, IPTC or an ICC profile.
func TestBuildStripsMetadata(t *testing.T) {
	t.Parallel()
	v := newTestVips(t)

	crops := map[Kind]Rect{}
	for _, k := range CropKinds {
		r, err := DefaultCrop(k, 1200, 800)
		require.NoError(t, err)

		crops[k] = r
	}

	p, orig := photoFromFixture(t, v, "gps.jpg", crops)
	require.True(t, hasField(t, orig, "exif-data"), "the fixture must carry EXIF")

	files := buildAll(t, v, p, orig, t.TempDir())
	require.Len(t, files, len(Expected(p)))

	for _, f := range files {
		for _, field := range []string{"exif-data", "xmp-data", "iptc-data", "icc-profile-data"} {
			require.False(t, hasField(t, f, field), "%s still has %s", f, field)
		}
	}
}

func TestBuildSizes(t *testing.T) {
	t.Parallel()
	v := newTestVips(t)
	crops := map[Kind]Rect{Banner: {X: 0, Y: 62, W: 1200, H: 675}, Share: {X: 0, Y: 85, W: 1200, H: 630}}
	p, orig := photoFromFixture(t, v, "upright.jpg", crops)

	out := t.TempDir()
	buildAll(t, v, p, orig, out)

	for _, s := range Expected(p) {
		w, h := dims(t, filepath.Join(out, s.Path()))
		require.Equal(t, s.Width, w, s.Name())

		var r Rect
		if s.Kind == Full {
			r = Rect{W: 1200, H: 800}
		} else {
			r = crops[s.Kind]
		}

		require.InDelta(t, s.Kind.height(r, s.Width), h, 1, s.Name())
	}
}

func TestBuildLeavesNothingOnFailure(t *testing.T) {
	v := newTestVips(t)
	p := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 1200, Height: 800, Crops: map[Kind]Rect{Card: exampleCrop}}
	out := t.TempDir()

	g := groups(p)[1]
	require.Equal(t, Card, g.kind)

	err := v.build(context.Background(), p, fixture(t, "truncated.jpg"), g, g.specs,
		func(s VariantSpec) string { return filepath.Join(out, s.Path()) })
	require.ErrorIs(t, err, ErrBroken)

	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	require.Empty(t, entries)

	entries, err = os.ReadDir(v.tmp)
	require.NoError(t, err)
	require.Empty(t, entries, "work files are removed")
}
