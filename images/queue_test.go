package images

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueueOrder(t *testing.T) {
	s := newTestStore(t, newFakeCatalog())

	// A bulk upload's tail and start-up's backlog are waiting. A new upload
	// jumps ahead of both, its own tail goes behind the upload's tail, and
	// both tails go ahead of the backlog.
	s.enqueue("a", tierBacklog)
	s.enqueue("b", tierBacklog)
	s.enqueue("u", tierSecond)
	s.Enqueue("c")
	s.Enqueue("c")

	var got []string
	for {
		sha, q, ok := s.next()
		if !ok {
			break
		}

		got = append(got, fmt.Sprintf("%s/%d", sha, q))
		if q == tierFirst {
			s.enqueue(sha, tierSecond)
		}
	}

	require.Equal(t, []string{"c/0", "u/1", "c/1", "a/2", "b/2"}, got)
}

// A crop saved while the queue works through a backlog is built next, all
// of it, before the photo being built goes on.
func TestCropChangeOvertakesBacklog(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := context.Background()

	x := photoOf(intake(t, s, "upright.jpg"))
	cat.put(x)

	p := photoOf(intake(t, s, "orient6.jpg"))
	cat.put(p)
	require.NoError(t, s.warm(ctx, p.SHA256, allSpecs))

	s.enqueue(x.SHA256, tierBacklog)

	// The admin recrops p as the worker starts on x.
	cat.onPhoto = func(string) {
		p.Crops = map[Kind]Rect{Card: {X: 0, Y: 0, W: 480, H: 600}}
		cat.put(p)
		s.Enqueue(p.SHA256)
	}

	missing := func(sha string) int {
		st, err := s.Status(ctx, sha)
		require.NoError(t, err)

		return st.Total - st.Ready
	}

	require.True(t, s.step(ctx))
	require.Equal(t, 3, missing(x.SHA256), "x stops after its full group")
	require.Equal(t, 3, missing(p.SHA256))

	require.True(t, s.step(ctx))
	require.Equal(t, 2, missing(p.SHA256), "first pass: the status moves")

	require.True(t, s.step(ctx))
	require.Zero(t, missing(p.SHA256), "then the rest of the new crop")
	require.Equal(t, 3, missing(x.SHA256))

	require.True(t, s.step(ctx))
	require.Zero(t, missing(x.SHA256))
	require.False(t, s.step(ctx))
}

func TestFirstPass(t *testing.T) {
	p := Photo{
		SHA256: testSHA, Width: 3000, Height: 2000,
		Crops: map[Kind]Rect{
			Card:   {X: 0, Y: 0, W: 1600, H: 2000},
			Banner: {X: 0, Y: 0, W: 3000, H: 1687},
			Share:  {X: 0, Y: 0, W: 3000, H: 1575},
		},
	}

	got := map[Kind][]string{}
	for _, g := range groups(p) {
		for _, s := range firstPass(p, g) {
			got[g.kind] = append(got[g.kind], fmt.Sprintf("%s/%d", s.Format, s.Width))
		}
	}

	require.Equal(t, map[Kind][]string{
		Full:   {"jpeg/1200", "avif/800"},
		Card:   {"avif/800"},
		Banner: {"avif/800"},
		Share:  {"jpeg/1200"},
	}, got)
}

func TestStatus(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := context.Background()

	o := intake(t, s, "upright.jpg")
	p := photoOf(o)
	cat.put(p)
	s.Changed()

	st, err := s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, Status{
		State: StateQueued, Ready: 1, Total: len(Expected(p)),
		Preview: o.Preview, Width: 1200, Height: 800,
	}, st)

	require.NoError(t, s.warm(ctx, p.SHA256, allSpecs))

	st, err = s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateReady, st.State)
	require.Equal(t, st.Total, st.Ready)

	// A crop change: only the card is rebuilt, and status says so.
	p.Crops = map[Kind]Rect{Card: {X: 0, Y: 0, W: 480, H: 600}}
	cat.put(p)
	s.Changed()

	st, err = s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateQueued, st.State)
	require.Equal(t, st.Total-3, st.Ready)

	before := s.builds.Load()
	require.NoError(t, s.warm(ctx, p.SHA256, allSpecs))
	require.Equal(t, before+1, s.builds.Load())
}

func TestStatusFailed(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := context.Background()

	// A damaged original that skipped Intake, as a bad restore would leave.
	b, err := os.ReadFile(fixture(t, "truncated.jpg"))
	require.NoError(t, err)

	p := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 1200, Height: 800}
	orig := s.originalPath(p.SHA256, p.MIME)
	require.NoError(t, os.MkdirAll(filepath.Dir(orig), dirPerm))
	require.NoError(t, os.WriteFile(orig, b, 0o600))
	cat.put(p)

	require.ErrorIs(t, s.warm(ctx, p.SHA256, allSpecs), ErrBroken)

	st, err := s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateFailed, st.State)
	require.Equal(t, ReasonDamaged, st.Reason)
	require.Empty(t, st.Preview)

	// Gone: a restore is the only fix.
	require.NoError(t, os.Remove(orig))
	s.failed = map[string]failure{}
	require.Error(t, s.warm(ctx, p.SHA256, allSpecs))

	st, err = s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, ReasonMissingOriginal, st.Reason)
}

func TestRun(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)

	// Queued before a restart: Run finds it again.
	old := photoOf(intake(t, s, "orient6.jpg"))
	cat.put(old)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() { done <- s.Run(ctx) }()

	o := intake(t, s, "upright.jpg")
	p := photoOf(o)
	share, err := DefaultCrop(Share, o.Width, o.Height)
	require.NoError(t, err)

	p.Crops[Share] = share
	cat.put(p)
	s.Enqueue(p.SHA256)
	s.Enqueue(p.SHA256)

	for _, sha := range []string{p.SHA256, old.SHA256} {
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			st, err := s.Status(ctx, sha)
			require.NoError(c, err)
			assert.Equal(c, StateReady, st.State)
		}, time.Minute, 50*time.Millisecond)
	}

	cancel()
	require.NoError(t, <-done)
}

func TestQueueSkipsDeletedPhotos(t *testing.T) {
	t.Parallel()
	s := newTestStore(t, newFakeCatalog())

	require.NoError(t, s.warm(context.Background(), testSHA, allSpecs))
	require.Zero(t, s.builds.Load())
}

func TestRetry(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := context.Background()

	b, err := os.ReadFile(fixture(t, "truncated.jpg"))
	require.NoError(t, err)

	p := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 1200, Height: 800}
	orig := s.originalPath(p.SHA256, p.MIME)
	require.NoError(t, os.MkdirAll(filepath.Dir(orig), dirPerm))
	require.NoError(t, os.WriteFile(orig, b, 0o600))
	cat.put(p)

	require.ErrorIs(t, s.warm(ctx, p.SHA256, allSpecs), ErrBroken)
	require.NotEmpty(t, s.failed)

	require.NoError(t, s.Retry(ctx, p.SHA256))
	require.Empty(t, s.failed, "the failures are forgotten")

	st, err := s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateQueued, st.State)

	sha, q, ok := s.next()
	require.True(t, ok)
	require.Equal(t, tierFirst, q)
	require.Equal(t, p.SHA256, sha)

	require.ErrorIs(t, s.Retry(ctx, strings.Repeat("f", 64)), ErrNotFound)
}

func TestAdminURLs(t *testing.T) {
	p := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 3000, Height: 2000}
	require.Equal(t, "/img/v1/"+testSHA+"/full-"+(Rect{W: 3000, H: 2000}).ID()+"-1200.jpg", PreviewURL(p))
	require.Equal(t, "/img/v1/"+testSHA+"/full-"+(Rect{W: 3000, H: 2000}).ID()+"-480.jpg", ThumbURL(p))

	bad := Photo{SHA256: "nope", Width: 3000, Height: 2000}
	require.Empty(t, PreviewURL(bad))
	require.Empty(t, ThumbURL(bad))
}

func TestSweepBeforeReconcile(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	cat := newFakeCatalog()

	unused := strings.Repeat("d", 64)
	cat.put(Photo{SHA256: unused, MIME: "image/jpeg", Width: 1200, Height: 800})
	// An empty catalogue holds every original back; see reconcileOriginals.
	cat.put(Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 1200, Height: 800})

	swept := 0
	s := newTestStore(t, cat, WithClock(clock.now), WithSweep(func(context.Context) error {
		swept++
		cat.remove(unused)

		return nil
	}))

	orig := s.originalPath(unused, "image/jpeg")
	plant(t, orig, clock.now().Add(-8*24*time.Hour))

	s.reconcileAndLog(context.Background())
	require.Equal(t, 1, swept)
	require.NoFileExists(t, orig, "the swept photo's original goes in the same pass")
}

func TestWarmAll(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)

	good := photoOf(intake(t, s, "upright.jpg"))
	cat.put(good)
	// A row whose original is gone: reported, the rest still built.
	cat.put(Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 1200, Height: 800})

	err := s.WarmAll(context.Background())
	require.ErrorContains(t, err, "original of "+testSHA)

	st, err := s.Status(context.Background(), good.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateReady, st.State)
}
