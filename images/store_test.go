package images

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeClock is the injected clock for the negative cache and grace periods.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Now()} }

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func fixture(t *testing.T, name string) string {
	t.Helper()

	p, err := filepath.Abs(filepath.Join("testdata", name))
	require.NoError(t, err)

	return p
}

func newTestStore(t *testing.T, cat Catalog, opts ...Option) *Store {
	t.Helper()

	s, err := New(t.TempDir(), cat, opts...)
	require.NoError(t, err, "the tests run the real vips: it must be on PATH")

	return s
}

// intake stores a fixture through the real upload path.
func intake(t *testing.T, s *Store, name string) Original {
	t.Helper()

	f, err := os.Open(fixture(t, name))
	require.NoError(t, err)
	defer f.Close()

	o, err := s.Intake(context.Background(), f)
	require.NoError(t, err, name)

	return o
}

// photoOf turns an intake result into a catalogue row with the example crop
// of testdata/manifest.json as its card.
func photoOf(o Original) Photo {
	return Photo{
		SHA256: o.SHA256, MIME: o.MIME, Width: o.Width, Height: o.Height,
		Crops: map[Kind]Rect{Card: exampleCrop},
	}
}

// exampleCrop and the colours below are the scene in testdata/manifest.json.
var exampleCrop = Rect{X: 700, Y: 150, W: 480, H: 600}

// look decodes an output through PNG with vips, since Go reads neither AVIF
// nor WebP.
func look(t *testing.T, path string) image.Image {
	t.Helper()

	out := filepath.Join(t.TempDir(), "look.png")
	b, err := exec.Command("vips", "copy", path, out).CombinedOutput()
	require.NoError(t, err, "%s: %s", path, b)

	f, err := os.Open(out)
	require.NoError(t, err)
	defer f.Close()

	img, err := png.Decode(f)
	require.NoError(t, err)

	return img
}

func at(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

func hasField(t *testing.T, path, field string) bool {
	t.Helper()

	out, err := exec.Command("vipsheader", "-f", field, path).CombinedOutput()
	if err != nil {
		require.Contains(t, string(out), "not found", path)
		return false
	}

	return true
}

func dims(t *testing.T, path string) (int, int) {
	t.Helper()

	out, err := exec.Command("vipsheader", "-f", "width", "-f", "height", path).Output()
	require.NoError(t, err, path)

	var w, h int
	_, err = fmt.Sscan(string(out), &w, &h)
	require.NoError(t, err)

	return w, h
}

func TestNewNeedsVips(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := New(t.TempDir(), newFakeCatalog())
	require.ErrorIs(t, err, ErrNoVips)
}

// A start leaves no work files of a killed process and no variants of an
// older pipeline; the current version's files stay.
func TestNewCleansUp(t *testing.T) {
	root := t.TempDir()
	cur := VariantSpec{Photo: testSHA, Kind: Full, Crop: Rect{W: 1200, H: 800}.ID(), Width: 480, Format: JPEG}
	stale := []string{"tmp/upload-123", "tmp/build-9/scaled.v", "variants/v0/ab/" + testSHA + "/full-00000000-480.jpg"}

	for _, f := range append(stale, cur.Path()) {
		path := filepath.Join(root, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), dirPerm))
		require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	}

	_, err := New(root, newFakeCatalog())
	require.NoError(t, err)

	for _, f := range stale {
		require.NoFileExists(t, filepath.Join(root, f))
	}

	require.NoDirExists(t, filepath.Join(root, "variants", "v0"))
	require.FileExists(t, filepath.Join(root, cur.Path()))

	tmp, err := os.ReadDir(filepath.Join(root, "tmp"))
	require.NoError(t, err)
	require.Empty(t, tmp)
}

// A build stopped by shutdown is no failure: not logged as one, and not
// remembered against the photo.
func TestBuildStoppedIsNoFailure(t *testing.T) {
	logs := &bytes.Buffer{}
	cat := newFakeCatalog()
	s := newTestStore(t, cat, WithLogger(slog.New(slog.NewTextHandler(logs, nil))))
	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)

	spec := VariantSpec{Photo: p.SHA256, Kind: Card, Crop: exampleCrop.ID(), Width: 480, Format: JPEG}
	g := groups(p)[slices.IndexFunc(groups(p), func(g group) bool { return g.kind == Card })]

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, s.ensure(ctx, p, g, []VariantSpec{spec}), context.Canceled)

	require.NoError(t, s.warm(context.Background(), p.SHA256, allSpecs))
	require.FileExists(t, s.path(spec))
	require.NotContains(t, logs.String(), "level=ERROR")
}

func TestBuildRejectsReplacedOriginal(t *testing.T) {
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	p := photoOf(intake(t, s, "upright.jpg"))
	// The catalogue remembers another size: the crop would land elsewhere.
	p.Width, p.Height = 1600, 1000
	cat.put(p)

	err := s.warm(context.Background(), p.SHA256, allSpecs)
	require.ErrorContains(t, err, "the catalogue says 1600x1000")
}
