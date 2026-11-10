package images

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func plant(t *testing.T, path string, mtime time.Time) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), dirPerm))
	require.NoError(t, os.WriteFile(path, []byte("planted"), 0o600))
	require.NoError(t, os.Chtimes(path, mtime, mtime))
}

func TestReconcile(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	cat := newFakeCatalog()
	s := newTestStore(t, cat, WithClock(clock.now))
	ctx := context.Background()

	kept := photoOf(intake(t, s, "upright.jpg"))
	cat.put(kept)
	require.NoError(t, s.WarmAll(ctx))

	// A photo whose original is gone: reported, never "fixed".
	lost := Photo{SHA256: strings.Repeat("c", 64), MIME: "image/jpeg", Width: 1200, Height: 800}
	cat.put(lost)

	now := clock.now()
	old := now.Add(-2 * time.Hour)
	ancient := now.Add(-8 * 24 * time.Hour)
	gone := strings.Repeat("d", 64)
	young := strings.Repeat("e", 64)
	cur := filepath.Join(s.root, "variants", fmt.Sprintf("v%d", PipelineVersion))

	oldCrop := VariantSpec{Photo: kept.SHA256, Kind: Card, Crop: "deadbeef", Width: 480, Format: AVIF}
	deleted := VariantSpec{Photo: gone, Kind: Full, Crop: "deadbeef", Width: 480, Format: AVIF}
	fresh := VariantSpec{Photo: kept.SHA256, Kind: Card, Crop: "0badf00d", Width: 480, Format: AVIF}
	plant(t, filepath.Join(s.root, oldCrop.Path()), old)
	plant(t, filepath.Join(s.root, deleted.Path()), old)
	plant(t, filepath.Join(s.root, fresh.Path()), now) // written after the catalogue was read
	plant(t, filepath.Join(s.root, "variants", "v0", "ab", "x.avif"), old)
	plant(t, filepath.Join(s.root, "variants", "junk"), old)
	plant(t, filepath.Join(s.root, "tmp", "build-1", "scaled.v"), old)
	require.NoError(t, os.Chtimes(filepath.Join(s.root, "tmp", "build-1"), old, old))
	plant(t, filepath.Join(s.root, "tmp", "upload-2"), now)
	plant(t, filepath.Join(s.root, "originals", gone[:2], gone+".jpg"), ancient)
	plant(t, filepath.Join(s.root, "originals", young[:2], young+".jpg"), now.Add(-24*time.Hour))
	plant(t, filepath.Join(s.root, "originals", "README"), ancient)
	plant(t, filepath.Join(s.root, "originals", "ab", gone+".jpg"), ancient) // wrong prefix folder

	want := Report{
		OrphanVariants:   2,
		OldPipelines:     []string{"junk", "v0"},
		StaleTmp:         1,
		OrphanOriginals:  []string{gone},
		GraceOriginals:   1,
		MissingOriginals: []string{lost.SHA256},
		MissingVariants:  len(Expected(lost)),
		Unknown:          []string{"README", filepath.Join("ab", gone+".jpg")},
	}

	dry, err := s.Reconcile(ctx, true)
	require.NoError(t, err)
	require.Positive(t, dry.Freed)
	dry.Freed = 0
	want.DryRun = true
	require.Equal(t, want, dry)
	require.FileExists(t, filepath.Join(s.root, oldCrop.Path()), "a dry run deletes nothing")

	rep, err := s.Reconcile(ctx, false)
	require.NoError(t, err)
	rep.Freed = 0
	want.DryRun = false
	require.Equal(t, want, rep)

	for _, p := range []string{
		oldCrop.Path(), deleted.Path(),
		filepath.Join("variants", "v0"), filepath.Join("variants", "junk"),
		filepath.Join("tmp", "build-1"),
		filepath.Join("originals", gone[:2], gone+".jpg"),
		filepath.Join(cur, gone[:2]), // emptied folders go too
	} {
		if !filepath.IsAbs(p) {
			p = filepath.Join(s.root, p)
		}

		require.NoFileExists(t, p)
		require.NoDirExists(t, p)
	}

	for _, p := range []string{
		fresh.Path(),
		filepath.Join("tmp", "upload-2"),
		filepath.Join("originals", young[:2], young+".jpg"),
		filepath.Join("originals", "README"),
	} {
		require.FileExists(t, filepath.Join(s.root, p))
	}

	for _, sp := range Expected(kept) {
		require.FileExists(t, filepath.Join(s.root, sp.Path()))
	}

	// Once the fresh file is old enough to judge, it goes; then nothing is left.
	clock.advance(2 * freshSlack)

	rep, err = s.Reconcile(ctx, false)
	require.NoError(t, err)
	require.Equal(t, 1, rep.OrphanVariants)

	rep, err = s.Reconcile(ctx, false)
	require.NoError(t, err)
	require.Zero(t, rep.OrphanVariants+rep.StaleTmp+len(rep.OrphanOriginals)+len(rep.OldPipelines))
}

func TestReconcileKeepsOriginalsForEmptyCatalogue(t *testing.T) {
	t.Parallel()
	clock := newFakeClock()
	cat := newFakeCatalog()
	s := newTestStore(t, cat, WithClock(clock.now))

	sha := strings.Repeat("d", 64)
	path := filepath.Join(s.root, "originals", sha[:2], sha+".jpg")
	plant(t, path, clock.now().Add(-30*24*time.Hour))

	_, err := s.Reconcile(context.Background(), false)
	require.ErrorContains(t, err, "kept them")
	require.FileExists(t, path)
}

func TestReconcileNeedsTheCatalogue(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)

	orphan := filepath.Join(s.root, VariantSpec{Photo: testSHA, Kind: Full, Crop: "deadbeef", Width: 480, Format: AVIF}.Path())
	plant(t, orphan, time.Now().Add(-time.Hour))

	cat.err = errors.New("database is locked")
	_, err := s.Reconcile(context.Background(), false)
	require.ErrorContains(t, err, "database is locked")
	require.FileExists(t, orphan)
}
