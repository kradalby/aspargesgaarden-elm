package images

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
