package images

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestVips(t *testing.T) *vips {
	t.Helper()

	v, err := newVips(t.TempDir())
	require.NoError(t, err)

	return v
}

func TestPreflightAccepts(t *testing.T) {
	t.Parallel()
	v := newTestVips(t)

	tests := []struct {
		file, mime, loader string
	}{
		{"upright.jpg", "image/jpeg", "jpegload"},
		{"orient2.jpg", "image/jpeg", "jpegload"},
		{"orient3.jpg", "image/jpeg", "jpegload"},
		{"orient4.jpg", "image/jpeg", "jpegload"},
		{"orient5.jpg", "image/jpeg", "jpegload"},
		{"orient6.jpg", "image/jpeg", "jpegload"},
		{"orient7.jpg", "image/jpeg", "jpegload"},
		{"orient8.jpg", "image/jpeg", "jpegload"},
		{"rotated.heic", "image/heic", "heifload"},
		{"p3.jpg", "image/jpeg", "jpegload"},
		{"gps.jpg", "image/jpeg", "jpegload"},
		{"cmyk.jpg", "image/jpeg", "jpegload"},
		{"alpha.png", "image/png", "pngload"},
		{"rgb16.png", "image/png", "pngload"},
		{"upright.webp", "image/webp", "webpload"},
		{"orient6.webp", "image/webp", "webpload"},
		// The header is fine; the decode is what fails (see TestDecodeFails).
		{"truncated.jpg", "image/jpeg", "jpegload"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			hdr, err := v.preflight(context.Background(), fixture(t, tt.file), tt.mime)
			require.NoError(t, err)
			// Every fixture is the same 1200×800 scene, however it is stored.
			require.Equal(t, header{loader: tt.loader, width: 1200, height: 800}, hdr)
		})
	}
}

func TestPreflightRejects(t *testing.T) {
	t.Parallel()
	v := newTestVips(t)

	tests := []struct {
		file, mime string
		want       error
	}{
		{"empty.jpg", "image/jpeg", ErrUnsupported},
		{"text.jpg", "image/jpeg", ErrUnsupported},
		// Both get past VIPS_BLOCK_UNTRUSTED; the allow-list stops them even
		// when the magic-byte sniff is fooled.
		{"matrix.jpg", "image/jpeg", ErrUnsupported},
		{"animated.gif", "image/jpeg", ErrUnsupported},
		// A real PNG claimed as JPEG: the loader must match the sniff.
		{"alpha.png", "image/jpeg", ErrUnsupported},
		{"animated.webp", "image/webp", ErrUnsupported},
		{"bomb-dims.jpg", "image/jpeg", ErrTooManyPixels},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			_, err := v.preflight(context.Background(), fixture(t, tt.file), tt.mime)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestDecodeFails(t *testing.T) {
	v := newTestVips(t)
	out := t.TempDir() + "/out.v"

	err := v.decode(context.Background(), fixture(t, "truncated.jpg"), out, 480)
	require.ErrorIs(t, err, ErrBroken)
	require.ErrorContains(t, err, "premature end of JPEG")
}

// A vips the machine killed, or one stopped with its context, says nothing
// about the photo: never "damaged" or "not a photo".
func TestInterruptedIsNoVerdict(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	upright := fixture(t, "upright.jpg")

	killed := filepath.Join(t.TempDir(), "killed")
	require.NoError(t, os.WriteFile(killed, []byte("#!/bin/sh\nkill -KILL $$\n"), 0o700)) //nolint:gosec // a test's own script

	v := newTestVips(t)
	v.vips, v.header = killed, killed

	err := v.decode(ctx, upright, filepath.Join(t.TempDir(), "out.v"), 480)
	require.ErrorIs(t, err, errInterrupted)
	require.Equal(t, ReasonInternal, ReasonOf(err))

	_, err = v.preflight(ctx, upright, "image/jpeg")
	require.ErrorIs(t, err, errInterrupted)
	require.Equal(t, ReasonInternal, ReasonOf(err))

	stopped, cancel := context.WithCancel(ctx)
	cancel()

	err = newTestVips(t).decode(stopped, fixture(t, "truncated.jpg"), filepath.Join(t.TempDir(), "out.v"), 480)
	require.NotErrorIs(t, err, ErrBroken)

	// The file's own fault still is one.
	err = newTestVips(t).decode(ctx, fixture(t, "truncated.jpg"), filepath.Join(t.TempDir(), "out.v"), 480)
	require.ErrorIs(t, err, ErrBroken)
	require.NotErrorIs(t, err, errInterrupted)
}

func TestRunEnvironment(t *testing.T) {
	v := newTestVips(t)
	ctx := context.Background()

	// Untrusted processes cannot read our .v files; trusted ones can.
	out := t.TempDir() + "/scaled.v"
	require.NoError(t, v.decode(ctx, fixture(t, "upright.jpg"), out, 480))

	_, err := v.run(ctx, true, v.header, "-f", "width", out)
	require.Error(t, err)

	w, h, err := v.size(ctx, out)
	require.NoError(t, err)
	require.Equal(t, [2]int{480, 320}, [2]int{w, h})
}
