package images

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIntakeAccepts(t *testing.T) {
	t.Parallel()

	tests := []struct{ file, mime, ext string }{
		{"upright.jpg", "image/jpeg", "jpg"},
		{"orient6.jpg", "image/jpeg", "jpg"},
		{"rotated.heic", "image/heic", "heic"},
		{"p3.jpg", "image/jpeg", "jpg"},
		{"alpha.png", "image/png", "png"},
		{"orient6.webp", "image/webp", "webp"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			s := newTestStore(t, newFakeCatalog())

			b, err := os.ReadFile(fixture(t, tt.file))
			require.NoError(t, err)

			sum := sha256.Sum256(b)
			want := Original{SHA256: hex.EncodeToString(sum[:]), MIME: tt.mime, Width: 1200, Height: 800}
			pv := VariantSpec{Photo: want.SHA256, Kind: Full, Crop: Rect{W: 1200, H: 800}.ID(), Width: 1200, Format: JPEG}
			want.Preview = pv.URL()

			got, err := s.Intake(context.Background(), bytes.NewReader(b))
			require.NoError(t, err)
			require.Equal(t, want, got)

			// The preview and the thumb are there before the row is.
			thumb := pv
			thumb.Width = 480
			require.FileExists(t, filepath.Join(s.root, pv.Path()))
			require.FileExists(t, filepath.Join(s.root, thumb.Path()))

			// Stored byte for byte, under its content address.
			stored, err := os.ReadFile(filepath.Join(s.root, "originals", want.SHA256[:2], want.SHA256+"."+tt.ext))
			require.NoError(t, err)
			require.Equal(t, b, stored)

			tmp, err := os.ReadDir(filepath.Join(s.root, "tmp"))
			require.NoError(t, err)
			require.Empty(t, tmp)
		})
	}
}

func TestIntakeRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file   string
		want   error
		reason Reason
	}{
		{"empty.jpg", ErrUnsupported, ReasonNotPhoto},
		{"text.jpg", ErrUnsupported, ReasonNotPhoto},
		{"matrix.jpg", ErrUnsupported, ReasonNotPhoto},
		{"animated.gif", ErrUnsupported, ReasonNotPhoto},
		{"animated.webp", ErrUnsupported, ReasonNotPhoto},
		{"truncated.jpg", ErrBroken, ReasonDamaged},
		{"bomb-dims.jpg", ErrTooManyPixels, ReasonTooManyPixels},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			s := newTestStore(t, newFakeCatalog())

			f, err := os.Open(fixture(t, tt.file))
			require.NoError(t, err)
			defer f.Close()

			_, err = s.Intake(context.Background(), f)
			require.ErrorIs(t, err, tt.want)
			require.Equal(t, tt.reason, ReasonOf(err))

			for _, d := range []string{"originals", "variants", "tmp"} {
				entries, err := os.ReadDir(filepath.Join(s.root, d))
				require.NoError(t, err)
				require.Empty(t, entries, d)
			}
		})
	}
}

func TestIntakeLimits(t *testing.T) {
	t.Parallel()
	s := newTestStore(t, newFakeCatalog())

	b, err := os.ReadFile(fixture(t, "upright.jpg"))
	require.NoError(t, err)

	s.maxUpload = int64(len(b)) - 1
	_, err = s.Intake(context.Background(), bytes.NewReader(b))
	require.ErrorIs(t, err, ErrTooLarge)

	s.maxUpload = int64(len(b))
	_, err = s.Intake(context.Background(), bytes.NewReader(b))
	require.NoError(t, err)

	small := filepath.Join(t.TempDir(), "small.png")
	out, err := exec.Command("vips", "black", small, "479", "800").CombinedOutput()
	require.NoError(t, err, string(out))

	f, err := os.Open(small)
	require.NoError(t, err)
	defer f.Close()

	_, err = s.Intake(context.Background(), f)
	require.ErrorIs(t, err, ErrTooSmall)
}

func TestIntakeDedup(t *testing.T) {
	t.Parallel()
	s := newTestStore(t, newFakeCatalog())

	a := intake(t, s, "upright.jpg")
	path := s.originalPath(a.SHA256, a.MIME)
	old := time.Now().Add(-6 * 24 * time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))

	b := intake(t, s, "upright.jpg")
	require.Equal(t, a, b)

	// The second upload marks the original fresh for the row to come.
	st, err := os.Stat(path)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now(), st.ModTime(), time.Minute)

	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestSniff(t *testing.T) {
	t.Parallel()

	ftyp := func(brand string) []byte { return []byte("\x00\x00\x00\x18ftyp" + brand + "\x00\x00\x00\x00") }
	tests := []struct {
		in   []byte
		want string
	}{
		{[]byte{0xff, 0xd8, 0xff, 0xe0}, "image/jpeg"},
		{[]byte("\x89PNG\r\n\x1a\n...."), "image/png"},
		{[]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "image/webp"},
		{[]byte("RIFF\x00\x00\x00\x00WAVEfmt "), ""},
		{ftyp("heic"), "image/heic"},
		{ftyp("mif1"), "image/heic"},
		{ftyp("avif"), "image/avif"},
		{ftyp("mp42"), ""},
		{[]byte("GIF89a"), ""},
		{[]byte("2 2\n0 0\n"), ""},
		{nil, ""},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, sniff(tt.in), "%q", tt.in)
	}
}
