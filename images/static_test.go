package images

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func copyFixture(t *testing.T, name, dst string) {
	t.Helper()

	b, err := os.ReadFile(fixture(t, name))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dst, b, 0o600))
}

func TestBuildStatic(t *testing.T) {
	t.Parallel()

	src, out := t.TempDir(), t.TempDir()
	copyFixture(t, "upright.jpg", filepath.Join(src, "forside.jpg"))
	copyFixture(t, "orient6.jpg", filepath.Join(src, "loven.jpeg"))

	refs, err := BuildStatic(context.Background(), src, out, "/static/photos/")
	require.NoError(t, err)
	require.Len(t, refs, 2)

	b, err := os.ReadFile(filepath.Join(out, ManifestFile))
	require.NoError(t, err)

	parsed, err := ParseManifest(b)
	require.NoError(t, err)
	require.Equal(t, refs, parsed)

	for name, ref := range refs {
		require.Equal(t, [2]int{1200, 800}, [2]int{ref.Width, ref.Height}, name)
		require.Len(t, ref.Variants, 9, name)

		for _, v := range ref.Variants {
			rel, ok := strings.CutPrefix(v.URL, "/static/photos/")
			require.True(t, ok, v.URL)
			w, _ := dims(t, filepath.Join(out, rel))
			require.Equal(t, v.Width, w, v.URL)
		}
	}

	entries, err := os.ReadDir(out)
	require.NoError(t, err)
	require.Len(t, entries, 2, "variants and the manifest, no work files")
}

func TestBuildStaticIncremental(t *testing.T) {
	t.Parallel()

	src, out := t.TempDir(), t.TempDir()
	copyFixture(t, "upright.jpg", filepath.Join(src, "a.jpg"))
	copyFixture(t, "orient6.jpg", filepath.Join(src, "b.jpg"))

	refs, err := BuildStatic(context.Background(), src, out, "/p")
	require.NoError(t, err)

	first := filepath.Join(out, strings.TrimPrefix(refs["a"].Variants[0].URL, "/p/"))
	old := time.Unix(1, 0)
	require.NoError(t, os.Chtimes(first, old, old))

	junk := filepath.Join(out, "v0", "x", "stale.avif")
	require.NoError(t, os.MkdirAll(filepath.Dir(junk), 0o750))
	require.NoError(t, os.WriteFile(junk, nil, 0o600))
	require.NoError(t, os.Remove(filepath.Join(src, "b.jpg")))

	refs2, err := BuildStatic(context.Background(), src, out, "/p")
	require.NoError(t, err)
	require.Equal(t, map[string]Ref{"a": refs["a"]}, refs2)

	st, err := os.Stat(first)
	require.NoError(t, err)
	require.Equal(t, old, st.ModTime(), "a current variant is not rebuilt")

	var files []string
	require.NoError(t, filepath.WalkDir(out, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(out, p)
			files = append(files, filepath.ToSlash(rel))
		}

		return err
	}))

	want := append(make([]string, 0, 1+len(refs["a"].Variants)), ManifestFile)
	for _, v := range refs["a"].Variants {
		want = append(want, strings.TrimPrefix(v.URL, "/p/"))
	}

	require.ElementsMatch(t, want, files, "b's variants and the junk are gone")
	require.NoDirExists(t, filepath.Join(out, "v0"))
}

func TestBuildStaticRejects(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	copyFixture(t, "upright.jpg", filepath.Join(src, "a.jpg"))
	copyFixture(t, "alpha.png", filepath.Join(src, "a.png"))

	_, err := BuildStatic(context.Background(), src, t.TempDir(), "/x")
	require.ErrorContains(t, err, `two masters are named "a"`)

	src = t.TempDir()
	copyFixture(t, "truncated.jpg", filepath.Join(src, "broken.jpg"))

	_, err = BuildStatic(context.Background(), src, t.TempDir(), "/x")
	require.ErrorIs(t, err, ErrBroken)
}
