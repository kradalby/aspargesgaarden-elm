package images

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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

func TestEnsureOnlyCurrent(t *testing.T) {
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)

	ctx := context.Background()
	spec := VariantSpec{Photo: p.SHA256, Kind: Card, Crop: exampleCrop.ID(), Width: 480, Format: AVIF}

	path, err := s.Ensure(ctx, spec)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(s.root, spec.Path()), path)
	require.FileExists(t, path)

	notCurrent := []VariantSpec{
		{Photo: p.SHA256, Kind: Card, Crop: Rect{X: 0, Y: 0, W: 480, H: 600}.ID(), Width: 480, Format: AVIF},
		{Photo: p.SHA256, Kind: Card, Crop: exampleCrop.ID(), Width: 800, Format: AVIF}, // wider than the crop
		{Photo: p.SHA256, Kind: Share, Crop: exampleCrop.ID(), Width: 1200, Format: JPEG},
		{Photo: p.SHA256, Kind: Full, Crop: Rect{W: 1200, H: 800}.ID(), Width: 1600, Format: WebP},
		{Photo: testSHA, Kind: Card, Crop: exampleCrop.ID(), Width: 480, Format: AVIF},
	}
	for _, sp := range notCurrent {
		_, err := s.Ensure(ctx, sp)
		require.ErrorIs(t, err, ErrNotFound, sp.Name())
		require.NoFileExists(t, filepath.Join(s.root, sp.Path()))
	}

	// A new crop retires the old names.
	p.Crops = map[Kind]Rect{Card: {X: 0, Y: 0, W: 480, H: 600}}
	cat.put(p)
	s.Changed()

	_, err = s.Ensure(ctx, spec)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestEnsureBuildsOnce(t *testing.T) {
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)

	spec := VariantSpec{Photo: p.SHA256, Kind: Card, Crop: exampleCrop.ID(), Width: 480, Format: WebP}
	before := s.builds.Load() // Intake built the preview

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, err := s.Ensure(context.Background(), spec)
			require.NoError(t, err)
		})
	}

	wg.Wait()

	require.Equal(t, before+1, s.builds.Load())
}

// A visitor asking for a file the queue is building waits for that file,
// not for the rest of its group.
func TestEnsureWaitsOnlyForItsFile(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := context.Background()

	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)

	g := groups(p)[0]
	// The very last is the preview, which Intake built.
	first, last := g.specs[0], g.specs[len(g.specs)-2]
	require.NoFileExists(t, s.path(last))
	before := s.builds.Load()

	done := make(chan error, 1)

	go func() { done <- s.warm(ctx, p.SHA256, allSpecs) }()

	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()

		return s.inflight[s.path(first)] != nil
	}, time.Minute, time.Millisecond)

	_, err := s.Ensure(ctx, first)
	require.NoError(t, err)
	require.NoFileExists(t, s.path(last), "the visitor waited for the whole group")

	require.NoError(t, <-done)
	require.Equal(t, before+int64(len(groups(p))), s.builds.Load(), "nothing built twice")
}

func TestEnsureRemembersFailures(t *testing.T) {
	clock := newFakeClock()
	cat := newFakeCatalog()
	s := newTestStore(t, cat, WithClock(clock.now))

	// A truncated original that skipped Intake, as a bad restore would leave.
	b, err := os.ReadFile(fixture(t, "truncated.jpg"))
	require.NoError(t, err)

	p := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 1200, Height: 800, Crops: map[Kind]Rect{Card: exampleCrop}}
	orig := s.originalPath(p.SHA256, p.MIME)
	require.NoError(t, os.MkdirAll(filepath.Dir(orig), dirPerm))
	require.NoError(t, os.WriteFile(orig, b, 0o600))
	cat.put(p)

	spec := VariantSpec{Photo: p.SHA256, Kind: Card, Crop: exampleCrop.ID(), Width: 480, Format: JPEG}
	_, err = s.Ensure(context.Background(), spec)
	require.ErrorIs(t, err, ErrBroken)
	require.Equal(t, int64(1), s.builds.Load())

	_, err = s.Ensure(context.Background(), spec)
	require.ErrorContains(t, err, "failed recently")
	require.Equal(t, int64(1), s.builds.Load(), "no second build inside the TTL")

	st, err := s.Status(context.Background(), p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateFailed, st.State)

	clock.advance(failTTL + time.Second)

	_, err = s.Ensure(context.Background(), spec)
	require.ErrorIs(t, err, ErrBroken)
	require.Equal(t, int64(2), s.builds.Load())
}

// A build stopped by shutdown is no failure: not logged as one, and not
// remembered against the photo.
func TestEnsureStoppedIsNoFailure(t *testing.T) {
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

	_, err := s.Ensure(context.Background(), spec)
	require.NoError(t, err)
	require.NotContains(t, logs.String(), "level=ERROR")
}

func TestEnsureRejectsReplacedOriginal(t *testing.T) {
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	p := photoOf(intake(t, s, "upright.jpg"))
	// The catalogue remembers another size: the crop would land elsewhere.
	p.Width, p.Height = 1600, 1000
	cat.put(p)

	_, err := s.Ensure(context.Background(), VariantSpec{Photo: p.SHA256, Kind: Card, Crop: exampleCrop.ID(), Width: 480, Format: JPEG})
	require.ErrorContains(t, err, "the catalogue says 1600x1000")
}

func TestStoreRef(t *testing.T) {
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	p := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 1200, Height: 800, Crops: map[Kind]Rect{Card: exampleCrop}}
	cat.put(p)

	ref, err := s.Ref(context.Background(), testSHA, Card)
	require.NoError(t, err)

	want, err := RefFor(p, Card)
	require.NoError(t, err)
	require.Equal(t, want, ref)

	_, err = s.Ref(context.Background(), strings.Repeat("0", 64), Card)
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCatalogueView(t *testing.T) {
	clock := newFakeClock()
	cat := newFakeCatalog()
	s := newTestStore(t, cat, WithClock(clock.now))
	ctx := context.Background()

	a := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 1200, Height: 800}
	cat.put(a)
	_, err := s.Ref(ctx, a.SHA256, Full)
	require.NoError(t, err)

	// Not told: the view is kept, so serving costs no query...
	b := a
	b.SHA256 = strings.Repeat("ab", 32)
	cat.put(b)
	_, err = s.Ref(ctx, b.SHA256, Full)
	require.ErrorIs(t, err, ErrNotFound)

	// ...until it ages out,
	clock.advance(viewMaxAge + time.Second)
	_, err = s.Ref(ctx, b.SHA256, Full)
	require.NoError(t, err)

	// or Changed says so.
	cat.remove(a.SHA256)
	s.Changed()
	_, err = s.Ref(ctx, a.SHA256, Full)
	require.ErrorIs(t, err, ErrNotFound)

	// A failing reload keeps serving the last good view.
	cat.err = errors.New("database is locked")
	s.Changed()
	_, err = s.Ref(ctx, b.SHA256, Full)
	require.NoError(t, err)
}
