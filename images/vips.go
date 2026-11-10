package images

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// vipsThreads pins libvips' worker threads per process. AVIF bytes
	// depend on it, so pinning keeps a build reproducible.
	vipsThreads = 2
	// processTimeout bounds one vips process; a hung decoder must not hold a
	// job slot forever.
	processTimeout = 2 * time.Minute
	// maxPixels is checked from the header, before anything is decoded.
	maxPixels = 100_000_000
	// noHeight makes the width alone decide a thumbnail's scale.
	noHeight = "10000000"
)

// ErrNoVips means the vips CLI is not on PATH. The service cannot build a
// single variant without it, so it refuses to start.
var ErrNoVips = errors.New("vips and vipsheader must be on PATH")

// loaders is the allow-list: the libvips loader each sniffed type may use.
// VIPS_BLOCK_UNTRUSTED alone still lets matrixload, TIFF and GIF through.
var loaders = map[string][]string{
	"image/jpeg": {"jpegload", "uhdrload"},
	"image/png":  {"pngload"},
	"image/webp": {"webpload"},
	"image/heic": {"heifload"},
	"image/avif": {"heifload"},
}

// vips runs the vips CLI. Every call is its own process: a crash or a hang
// in a decoder kills that child only.
type vips struct {
	vips, header string
	tmp          string // TMPDIR for the children: large decodes spill here
}

func newVips(tmp string) (*vips, error) {
	v, err := exec.LookPath("vips")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoVips, err)
	}

	h, err := exec.LookPath("vipsheader")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoVips, err)
	}

	return &vips{vips: v, header: h, tmp: tmp}, nil
}

// run executes bin with args. untrusted is for every process that reads an
// original; our own .v intermediates need vipsload, which the flag blocks.
func (v *vips) run(ctx context.Context, untrusted bool, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, processTimeout)
	defer cancel()

	// bin is from exec.LookPath; args are a vector, never a shell line.
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // see above
	cmd.Env = []string{"TMPDIR=" + v.tmp, "VIPS_CONCURRENCY=" + strconv.Itoa(vipsThreads)}
	if untrusted {
		cmd.Env = append(cmd.Env, "VIPS_BLOCK_UNTRUSTED=1")
	}

	cmd.WaitDelay = 5 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := start(cmd)
	if err == nil {
		err = cmd.Wait()
	}

	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("%w (%w)", err, ctx.Err())
		}

		if !refused(ctx, err) {
			err = fmt.Errorf("%w: %w", errInterrupted, err)
		}

		msg := stderr.Bytes()
		if len(msg) > 1024 {
			msg = msg[len(msg)-1024:]
		}

		return "", fmt.Errorf("%s %s: %w: %s", filepath.Base(bin), strings.Join(args, " "), err, bytes.TrimSpace(msg))
	}

	return stdout.String(), nil
}

// errInterrupted marks a vips process that did not finish by itself: killed
// by the OOM killer under the unit's MemoryMax, timed out, stopped with the
// service, or never started. It says nothing about the photo, so it must not
// become ErrBroken or ErrUnsupported.
var errInterrupted = errors.New("vips did not finish")

// refused reports whether vips ended by itself with an error: its verdict
// on the file. SIGKILL is the OOM killer's and the context's signal.
func refused(ctx context.Context, err error) bool {
	var ee *exec.ExitError
	if ctx.Err() != nil || !errors.As(err, &ee) {
		return false
	}

	ws, ok := ee.Sys().(syscall.WaitStatus)

	return !ok || !ws.Signaled() || ws.Signal() != syscall.SIGKILL
}

// header is what preflight learns about an original. Width and Height are
// upright: after EXIF orientation, the frame crops are given in.
type header struct {
	loader        string
	width, height int
}

// preflight reads only single header fields of an untrusted file. Never
// `vipsheader -a`: EXIF strings can hold newlines and forge lines.
func (v *vips) preflight(ctx context.Context, path, mime string) (header, error) {
	out, err := v.run(ctx, true, v.header, "-f", "vips-loader", "-f", "width", "-f", "height", path)
	if err != nil {
		return header{}, verdict(ErrUnsupported, err)
	}

	f := strings.Split(strings.TrimSpace(out), "\n")
	if len(f) != 3 {
		return header{}, fmt.Errorf("%w: vipsheader printed %q", ErrBroken, out)
	}

	hdr := header{loader: f[0]}
	if !slices.Contains(loaders[mime], hdr.loader) {
		return header{}, fmt.Errorf("%w: %s opens with %s", ErrUnsupported, mime, hdr.loader)
	}

	hdr.width, err = strconv.Atoi(f[1])
	if err != nil {
		return header{}, fmt.Errorf("%w: width %q", ErrBroken, f[1])
	}

	hdr.height, err = strconv.Atoi(f[2])
	if err != nil {
		return header{}, fmt.Errorf("%w: height %q", ErrBroken, f[2])
	}

	if hdr.width <= 0 || hdr.height <= 0 {
		return header{}, fmt.Errorf("%w: %dx%d", ErrBroken, hdr.width, hdr.height)
	}

	if int64(hdr.width)*int64(hdr.height) > maxPixels {
		return header{}, fmt.Errorf("%w: %dx%d", ErrTooManyPixels, hdr.width, hdr.height)
	}

	pages, err := v.intField(ctx, path, "n-pages")
	if err != nil {
		return header{}, err
	}

	// libvips would quietly use the first frame. HEIF is exempt: there extra
	// pages are other images in the container and libvips loads the primary.
	if pages > 1 && hdr.loader != "heifload" {
		return header{}, fmt.Errorf("%w: animated, %d frames", ErrUnsupported, pages)
	}

	o, err := v.intField(ctx, path, "orientation")
	if err != nil {
		return header{}, err
	}

	// vips reports the stored size; 5-8 turn the picture a quarter. HEIF has
	// no such field because libheif has already turned it.
	if o >= 5 && o <= 8 {
		hdr.width, hdr.height = hdr.height, hdr.width
	}

	return hdr, nil
}

// intField reads an optional integer header field; absent reads as 0.
func (v *vips) intField(ctx context.Context, path, field string) (int, error) {
	out, err := v.run(ctx, true, v.header, "-f", field, path)
	if err != nil {
		if strings.Contains(err.Error(), `field "`+field+`" not found`) {
			return 0, nil
		}

		return 0, verdict(ErrBroken, err)
	}

	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("%w: %s %q", ErrBroken, field, out)
	}

	return n, nil
}

// decode is stage 1, the only one that reads the original: shrink-on-load,
// autorotate, any ICC profile to sRGB, 16 to 8 bit, scaled to width. A
// truncated or corrupt file fails here instead of padding with grey.
func (v *vips) decode(ctx context.Context, orig, out string, width int) error {
	_, err := v.run(ctx, true, v.vips, "thumbnail", orig, out, strconv.Itoa(width),
		"--height", noHeight, "--size", "down", "--output-profile", "srgb", "--fail-on", "error")
	if err != nil {
		return verdict(ErrBroken, err)
	}

	return nil
}

// verdict wraps err in what it says about the file, unless vips was
// interrupted and so said nothing.
func verdict(about, err error) error {
	if errors.Is(err, errInterrupted) {
		return err
	}

	return fmt.Errorf("%w: %w", about, err)
}

// crop is stage 2, on our own intermediate.
func (v *vips) crop(ctx context.Context, in, out string, x, y, w, h int) error {
	_, err := v.run(ctx, false, v.vips, "crop", in, out,
		strconv.Itoa(x), strconv.Itoa(y), strconv.Itoa(w), strconv.Itoa(h))

	return err
}

// encode is stage 3: one file of the given width and format.
func (v *vips) encode(ctx context.Context, in, out string, width int, f Format) error {
	opts := map[Format]string{AVIF: avifOptions, WebP: webpOptions, JPEG: jpegOptions}[f]
	_, err := v.run(ctx, false, v.vips, "thumbnail", in, out+opts, strconv.Itoa(width),
		"--height", noHeight, "--size", "down")

	return err
}

// size reads the dimensions of a file we wrote.
func (v *vips) size(ctx context.Context, path string) (int, int, error) {
	out, err := v.run(ctx, false, v.header, "-f", "width", "-f", "height", path)
	if err != nil {
		return 0, 0, err
	}

	var w, h int
	if _, err := fmt.Sscan(out, &w, &h); err != nil {
		return 0, 0, fmt.Errorf("vipsheader printed %q: %w", out, err)
	}

	return w, h, nil
}
