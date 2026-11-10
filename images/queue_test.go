package images

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
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

	// Start-up's backlog is waiting. What someone asks for goes ahead of it,
	// in the order asked; asking again keeps the place, and asking for a
	// photo of the backlog takes it out of there.
	s.push("a", false)
	s.push("b", false)
	s.Enqueue("c")
	s.Enqueue("d")
	s.Enqueue("c")
	s.push("a", false)
	s.Enqueue("b")

	var got []string
	for {
		sha, ok := s.next()
		if !ok {
			break
		}

		got = append(got, sha)
	}

	require.Equal(t, []string{"c", "d", "b", "a"}, got)

	// Asked after the front was taken: still ahead of the backlog.
	s.push("x", false)
	s.Enqueue("y")
	sha, _ := s.next()
	require.Equal(t, "y", sha)
}

// A crop saved while the worker goes through start-up's backlog is built
// after the group in hand; the photo it interrupted carries on next.
func TestCropChangeOvertakesBacklog(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := t.Context()

	x := photoOf(intake(t, s, "upright.jpg"))
	y := photoOf(intake(t, s, "p3.jpg"))
	p := photoOf(intake(t, s, "orient6.jpg"))

	for _, ph := range []Photo{x, y, p} {
		cat.put(ph)
	}

	require.NoError(t, s.prepare(ctx, p.SHA256))
	s.push(x.SHA256, false)
	s.push(y.SHA256, false)

	// The admin recrops p as the worker starts on x.
	cat.onPhoto = func(string) {
		p.Crops = map[Kind]Rect{Card: {X: 0, Y: 0, W: 480, H: 600}}
		cat.put(p)
		s.Enqueue(p.SHA256)
	}

	state := func(sha string) State {
		st, err := s.Status(ctx, sha)
		require.NoError(t, err)

		return st.State
	}

	built := func(sha string) int {
		st, err := s.Status(ctx, sha)
		require.NoError(t, err)

		return st.Ready
	}

	require.True(t, s.step(ctx))
	require.Equal(t, len(groups(x)[0].specs), built(x.SHA256), "x stops after its first group")
	require.Equal(t, StateQueued, state(x.SHA256))
	require.Equal(t, StateQueued, state(p.SHA256))

	require.True(t, s.step(ctx))
	require.Equal(t, StateReady, state(p.SHA256), "the crop goes next")
	require.Equal(t, StateQueued, state(x.SHA256))

	require.True(t, s.step(ctx))
	require.Equal(t, StateReady, state(x.SHA256), "then x, ahead of the backlog")
	require.Equal(t, StateQueued, state(y.SHA256))

	require.True(t, s.step(ctx))
	require.Equal(t, StateReady, state(y.SHA256))
	require.False(t, s.step(ctx))
}

func TestStatus(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := t.Context()

	o := intake(t, s, "upright.jpg")
	p := photoOf(o)
	cat.put(p)
	s.Enqueue(p.SHA256)

	st, err := s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, Status{
		State: StateQueued, Ready: 2, Total: len(Expected(p)),
		Preview: o.Preview, Width: 1200, Height: 800,
	}, st)

	// The worker's view while it builds.
	cat.onPhoto = func(string) {
		st, err := s.Status(ctx, p.SHA256)
		require.NoError(t, err)
		require.Equal(t, StatePreparing, st.State)
	}

	require.True(t, s.step(ctx))

	st, err = s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateReady, st.State)
	require.Equal(t, st.Total, st.Ready)

	// A crop change: only the card is rebuilt, and status says so.
	p.Crops = map[Kind]Rect{Card: {X: 0, Y: 0, W: 480, H: 600}}
	cat.put(p)
	s.Enqueue(p.SHA256)

	st, err = s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateQueued, st.State)
	require.Equal(t, st.Total-3, st.Ready)

	before := s.builds.Load()
	require.True(t, s.step(ctx))
	require.Equal(t, before+1, s.builds.Load())
}

// A failed photo stays failed, with its reason, until someone asks again;
// then it is queued, and ready once the cause is gone.
func TestStatusFailed(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := t.Context()

	// A damaged original that skipped Intake, as a bad restore would leave.
	b, err := os.ReadFile(fixture(t, "truncated.jpg"))
	require.NoError(t, err)

	p := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 1200, Height: 800}
	orig := s.originalPath(p.SHA256, p.MIME)
	require.NoError(t, os.MkdirAll(filepath.Dir(orig), dirPerm))
	require.NoError(t, os.WriteFile(orig, b, 0o600))
	cat.put(p)

	status := func() Status {
		st, err := s.Status(ctx, p.SHA256)
		require.NoError(t, err)

		return st
	}

	s.Enqueue(p.SHA256)
	require.True(t, s.step(ctx))
	require.Equal(t, StateFailed, status().State)
	require.Equal(t, ReasonDamaged, status().Reason)
	require.Empty(t, status().Preview)

	// Other work comes and goes; the failure stays.
	other := photoOf(intake(t, s, "upright.jpg"))
	cat.put(other)
	s.Enqueue(other.SHA256)
	require.True(t, s.step(ctx))
	require.Equal(t, StateFailed, status().State)

	// Gone: a restore is the only fix.
	require.NoError(t, os.Remove(orig))
	require.NoError(t, s.Retry(ctx, p.SHA256))
	require.Equal(t, StateQueued, status().State)
	require.True(t, s.step(ctx))
	require.Equal(t, ReasonMissingOriginal, status().Reason)

	// Restored: the next try succeeds.
	good, err := os.ReadFile(fixture(t, "upright.jpg"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(orig, good, 0o600))
	require.NoError(t, s.Retry(ctx, p.SHA256))
	require.True(t, s.step(ctx))
	require.Equal(t, StateReady, status().State)
}

// Files missing that nothing is building is offered for a retry, whatever
// removed them.
func TestStatusMissingIsFailed(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)

	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)

	st, err := s.Status(t.Context(), p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateFailed, st.State)
	require.Equal(t, ReasonInternal, st.Reason)
}

// A build stopped by shutdown is no failure: not logged as one, and not
// remembered against the photo.
func TestStoppedIsNoFailure(t *testing.T) {
	logs := &bytes.Buffer{}
	cat := newFakeCatalog()
	s := newTestStore(t, cat, WithLogger(slog.New(slog.NewTextHandler(logs, nil))))
	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	s.Enqueue(p.SHA256)
	require.True(t, s.step(ctx))
	require.Empty(t, s.failed)
	require.NotContains(t, logs.String(), "level=ERROR")

	s.Enqueue(p.SHA256)
	require.True(t, s.step(t.Context()))

	st, err := s.Status(t.Context(), p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateReady, st.State)
}

// The original is probed once per photo, not once per group.
func TestPrepareProbesOnce(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)

	p := photoOf(intake(t, s, "upright.jpg"))
	p.Crops[Banner] = Rect{X: 0, Y: 62, W: 1200, H: 675}
	cat.put(p)

	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := filepath.Join(dir, "vipsheader")
	require.NoError(t, os.WriteFile(script, fmt.Appendf(nil,
		"#!/bin/sh\necho \"$@\" >> %s\nexec %s \"$@\"\n", calls, s.vips.header), 0o700)) //nolint:gosec // a test's own script
	s.vips.header = script

	require.NoError(t, s.prepare(t.Context(), p.SHA256))
	require.Equal(t, int64(4), s.builds.Load(), "the intake, then full, card and banner")

	b, err := os.ReadFile(calls)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(b), "vips-loader"))
}

func TestRejectsReplacedOriginal(t *testing.T) {
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	p := photoOf(intake(t, s, "upright.jpg"))
	// The catalogue remembers another size: the crop would land elsewhere.
	p.Width, p.Height = 1600, 1000
	cat.put(p)

	require.ErrorContains(t, s.prepare(t.Context(), p.SHA256), "the catalogue says 1600x1000")
}

func TestRun(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)

	// Known before a restart: Run finds it again.
	old := photoOf(intake(t, s, "orient6.jpg"))
	cat.put(old)

	ctx, cancel := context.WithCancel(t.Context())
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

// A crop change leaves the new crop's files and none of the old's.
func TestPruneOldCrop(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := t.Context()

	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)
	require.NoError(t, s.prepare(ctx, p.SHA256))

	old := Expected(p)
	p.Crops = map[Kind]Rect{Card: {X: 0, Y: 0, W: 480, H: 600}}
	cat.put(p)
	require.NoError(t, s.prepare(ctx, p.SHA256))

	want := make([]string, 0, len(Expected(p)))
	for _, sp := range Expected(p) {
		want = append(want, sp.file())
	}

	entries, err := os.ReadDir(s.photoDir(p.SHA256))
	require.NoError(t, err)

	got := make([]string, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.Name())
	}

	require.ElementsMatch(t, want, got)
	require.NotEqual(t, Expected(p), old)
}

// A deleted photo loses every file: variants and original.
func TestForgetDeletedPhoto(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := t.Context()

	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)
	require.NoError(t, s.prepare(ctx, p.SHA256))

	cat.remove(p.SHA256)
	s.Enqueue(p.SHA256)

	before := s.builds.Load()
	require.True(t, s.step(ctx))
	require.Equal(t, before, s.builds.Load(), "nothing built for it")

	require.NoDirExists(t, s.photoDir(p.SHA256))
	require.NoFileExists(t, s.originalPath(p.SHA256, p.MIME))

	for _, d := range []string{"originals", "variants/v1"} {
		entries, err := os.ReadDir(filepath.Join(s.root, d))
		require.NoError(t, err)
		require.Empty(t, entries, d)
	}
}

// The same bytes uploaded again after a delete, before the worker got to
// the delete: the upload's row is on the way, so nothing is removed.
func TestForgetSparesReupload(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := t.Context()

	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)
	require.NoError(t, s.prepare(ctx, p.SHA256))

	cat.remove(p.SHA256)
	s.Enqueue(p.SHA256)
	intake(t, s, "upright.jpg")
	require.True(t, s.step(ctx))
	require.FileExists(t, s.originalPath(p.SHA256, p.MIME))
	require.FileExists(t, s.path(Expected(p)[0]))

	// Its row seen once, a later delete removes it as usual.
	cat.put(p)
	require.NoError(t, s.prepare(ctx, p.SHA256))
	cat.remove(p.SHA256)
	require.NoError(t, s.prepare(ctx, p.SHA256))
	require.NoFileExists(t, s.originalPath(p.SHA256, p.MIME))
}

// A start with a database that does not know the originals on disk (a bad
// restore) touches none of them.
func TestRunLeavesUnknownOriginals(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)

	unknown := intake(t, s, "upright.jpg")
	known := photoOf(intake(t, s, "p3.jpg"))
	cat.put(known)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { done <- s.Run(ctx) }()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		st, err := s.Status(ctx, known.SHA256)
		require.NoError(c, err)
		assert.Equal(c, StateReady, st.State)
	}, time.Minute, 50*time.Millisecond)

	cancel()
	require.NoError(t, <-done)
	require.FileExists(t, s.originalPath(unknown.SHA256, unknown.MIME))
	pv, _ := previewSpec(photoOf(unknown))
	require.FileExists(t, s.path(pv), "nor its variants")
}

func TestPrepareRejectsBadNames(t *testing.T) {
	t.Parallel()
	s := newTestStore(t, newFakeCatalog())

	require.Error(t, s.prepare(t.Context(), "../../etc"))
	require.Zero(t, s.builds.Load())
}

func TestAdminURLs(t *testing.T) {
	p := Photo{SHA256: testSHA, MIME: "image/jpeg", Width: 3000, Height: 2000}
	require.Equal(t, "/img/v1/"+testSHA+"/full-"+(Rect{W: 3000, H: 2000}).ID()+"-1200.jpg", PreviewURL(p))
	require.Equal(t, "/img/v1/"+testSHA+"/full-"+(Rect{W: 3000, H: 2000}).ID()+"-480.jpg", ThumbURL(p))

	bad := Photo{SHA256: "nope", Width: 3000, Height: 2000}
	require.Empty(t, PreviewURL(bad))
	require.Empty(t, ThumbURL(bad))
}
