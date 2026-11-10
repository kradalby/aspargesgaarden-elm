package images

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
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

	for _, tt := range []struct {
		w, h string
		want error
	}{
		{"479", "800", ErrTooSmall},
		// 480 wide is the narrowest size, and WebP and AVIF stop at 16383.
		{"480", "16384", ErrUnsupported},
	} {
		path := filepath.Join(t.TempDir(), "black.png")
		out, err := exec.Command("vips", "black", path, tt.w, tt.h).CombinedOutput()
		require.NoError(t, err, string(out))

		f, err := os.Open(path)
		require.NoError(t, err)

		_, err = s.Intake(context.Background(), f)
		require.ErrorIs(t, err, tt.want, "%sx%s", tt.w, tt.h)
		require.NoError(t, f.Close())
	}
}

func TestIntakeDedup(t *testing.T) {
	t.Parallel()
	s := newTestStore(t, newFakeCatalog())

	a := intake(t, s, "upright.jpg")
	path := s.originalPath(a.SHA256, a.MIME)

	b := intake(t, s, "upright.jpg")
	require.Equal(t, a, b)

	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// Two uploads of the same bytes; the first is cut off in its decode after
// the second has finished. The second's original must stay.
func TestIntakeCutOffKeepsOthersOriginal(t *testing.T) {
	t.Parallel()
	s := newTestStore(t, newFakeCatalog())
	ctx := t.Context()

	dir := t.TempDir()
	first, fifo := filepath.Join(dir, "first"), filepath.Join(dir, "go")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))

	mkdir, err := exec.LookPath("mkdir")
	require.NoError(t, err)

	// The first decode waits for the test, then dies the way a request's end
	// or an OOM kill ends it; every other call is the real vips.
	script := filepath.Join(dir, "vips")
	require.NoError(t, os.WriteFile(script, fmt.Appendf(nil,
		"#!/bin/sh\nif [ \"$1\" = thumbnail ] && %s %s 2>/dev/null; then read x < %s; kill -KILL $$; fi\nexec %s \"$@\"\n",
		mkdir, first, fifo, s.vips.vips), 0o700)) //nolint:gosec // a test's own script
	s.vips.vips = script

	b, err := os.ReadFile(fixture(t, "upright.jpg"))
	require.NoError(t, err)

	cut := make(chan error, 1)

	go func() {
		_, err := s.Intake(ctx, bytes.NewReader(b))
		cut <- err
	}()

	require.Eventually(t, func() bool { return exists(first) }, time.Minute, 10*time.Millisecond)

	o, err := s.Intake(ctx, bytes.NewReader(b))
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(fifo, []byte("go\n"), 0o600))
	require.Error(t, <-cut)
	require.FileExists(t, s.originalPath(o.SHA256, o.MIME))
}

// A photo whose original went missing heals when the same file is uploaded
// again.
func TestIntakeHealsMissingOriginal(t *testing.T) {
	t.Parallel()
	cat := newFakeCatalog()
	s := newTestStore(t, cat)
	ctx := t.Context()

	p := photoOf(intake(t, s, "upright.jpg"))
	cat.put(p)
	require.NoError(t, os.Remove(s.originalPath(p.SHA256, p.MIME)))

	s.Enqueue(p.SHA256)
	require.True(t, s.step(ctx))

	st, err := s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, ReasonMissingOriginal, st.Reason)

	intake(t, s, "upright.jpg")
	s.Enqueue(p.SHA256) // the upload's Changed
	require.True(t, s.step(ctx))

	st, err = s.Status(ctx, p.SHA256)
	require.NoError(t, err)
	require.Equal(t, StateReady, st.State)
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
