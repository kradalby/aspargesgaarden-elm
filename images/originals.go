package images

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MaxUpload is the largest original Intake accepts.
const MaxUpload = 40 << 20

var (
	ErrTooLarge      = errors.New("file is larger than the upload limit")
	ErrUnsupported   = errors.New("not a supported image (JPEG, PNG, WebP, HEIC or AVIF)")
	ErrBroken        = errors.New("image is damaged or incomplete")
	ErrTooManyPixels = errors.New("image has too many pixels")
)

// Original is what Intake learned about an upload: what the photos row
// needs.
type Original struct {
	SHA256 string
	MIME   string
	// Upright size, after EXIF orientation: the frame crops are given in.
	Width, Height int
	// Preview is the URL of the crop tool's picture. It is built already and
	// is served once the photos row exists and Enqueue has been called.
	Preview string
}

// Intake stores an upload as received, after checking it is an image the
// pipeline will build: size limit, magic bytes, the libvips loader allow-list,
// a pixel cap from the header, and a full decode (which builds the preview),
// so a broken file is rejected now and not at its first build. The same
// content twice is one file. Errors map to a Reason with ReasonOf.
//
// The caller then writes the photos row.
func (s *Store) Intake(ctx context.Context, r io.Reader) (Original, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case <-s.stopped:
			cancel()
		case <-ctx.Done():
		}
	}()

	tmp := filepath.Join(s.root, "tmp")

	f, err := os.CreateTemp(tmp, "upload-")
	if err != nil {
		return Original{}, fmt.Errorf("upload file: %w", err)
	}

	// After the rename there is nothing to remove; a crash leaves it for
	// the next start (see clean).
	defer os.Remove(f.Name())
	defer f.Close()

	h := sha256.New()

	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(r, s.maxUpload+1))
	if err != nil {
		return Original{}, fmt.Errorf("read upload: %w", err)
	}

	if n > s.maxUpload {
		return Original{}, fmt.Errorf("%w of %d MB", ErrTooLarge, s.maxUpload>>20)
	}

	head := make([]byte, 32)
	if _, err := f.ReadAt(head, 0); err != nil && !errors.Is(err, io.EOF) {
		return Original{}, fmt.Errorf("read upload: %w", err)
	}

	mime := sniff(head)
	if mime == "" {
		return Original{}, ErrUnsupported
	}

	hdr, err := s.vips.preflight(ctx, f.Name(), mime)
	if err != nil {
		return Original{}, err
	}

	if hdr.width < Full.MinWidth() {
		return Original{}, fmt.Errorf("%w: %d pixels wide, at least %d needed", ErrTooSmall, hdr.width, Full.MinWidth())
	}

	o := Original{SHA256: hex.EncodeToString(h.Sum(nil)), MIME: mime, Width: hdr.width, Height: hdr.height}
	p := Photo{SHA256: o.SHA256, MIME: o.MIME, Width: o.Width, Height: o.Height}

	// Even its narrowest size would be too tall for WebP and AVIF.
	pv, ok := previewSpec(p)
	if !ok {
		return Original{}, fmt.Errorf("%w: %dx%d is too tall and narrow", ErrUnsupported, o.Width, o.Height)
	}

	// The preview is the full decode that proves the file is sound, and what
	// the admin shows next, with the thumb its lists show. They are built
	// from the upload where it lies, so an upload that fails or is cut off
	// never touches originals/, which another upload of the same bytes may
	// rely on.
	specs := []VariantSpec{pv}
	if th, ok := thumbSpec(p); ok && th != pv {
		specs = append(specs, th)
	}

	work, err := os.MkdirTemp(tmp, "intake-")
	if err != nil {
		return Original{}, fmt.Errorf("intake dir: %w", err)
	}
	defer os.RemoveAll(work)

	built := func(sp VariantSpec) string { return filepath.Join(work, sp.file()) }
	if err := s.buildGroup(ctx, p, f.Name(), groups(p)[0], specs, built); err != nil {
		return Original{}, err
	}

	if err := f.Sync(); err != nil {
		return Original{}, fmt.Errorf("sync upload: %w", err)
	}

	if err := s.store(f.Name(), o, specs, built); err != nil {
		return Original{}, err
	}

	o.Preview = pv.URL()

	return o, nil
}

// store moves a checked upload to its content address, unless the same
// content is there already, and its built files into place. The photo is
// fresh until the worker finds its row: the caller is about to write it,
// and a forget queued by an earlier delete of the same bytes must not take
// the original from under it.
func (s *Store) store(upload string, o Original, specs []VariantSpec, built func(VariantSpec) string) error {
	s.files.Lock()
	defer s.files.Unlock()

	s.fresh[o.SHA256] = true

	dst := s.originalPath(o.SHA256, o.MIME)
	if !exists(dst) {
		if err := os.MkdirAll(filepath.Dir(dst), dirPerm); err != nil {
			return fmt.Errorf("originals dir: %w", err)
		}

		if err := os.Rename(upload, dst); err != nil {
			return fmt.Errorf("store original: %w", err)
		}

		// Originals cannot be rebuilt: make the rename and a new prefix
		// folder durable before the caller records the photo.
		for _, d := range []string{filepath.Dir(dst), filepath.Join(s.root, "originals")} {
			if err := syncDir(d); err != nil {
				return err
			}
		}
	}

	for _, sp := range specs {
		if err := os.MkdirAll(filepath.Dir(s.path(sp)), dirPerm); err != nil {
			return fmt.Errorf("variant dir: %w", err)
		}

		//nolint:gosec // both paths are derived from a VariantSpec, never from input
		if err := os.Rename(built(sp), s.path(sp)); err != nil {
			return fmt.Errorf("move into place: %w", err)
		}
	}

	return nil
}

// sniff names the type from magic bytes. libvips must later agree (see
// loaders), so a file cannot pass as one type and be decoded as another.
func sniff(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xff, 0xd8, 0xff}):
		return "image/jpeg"
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP":
		return "image/webp"
	case len(b) >= 12 && string(b[4:8]) == "ftyp":
		switch string(b[8:12]) {
		case "heic", "heix", "heim", "heis", "hevc", "hevx", "mif1", "msf1":
			return "image/heic"
		case "avif", "avis":
			return "image/avif"
		}
	}

	return ""
}

// exts names an original's file by its type.
var exts = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
	"image/heic": "heic",
	"image/avif": "avif",
}

func extOf(mime string) string { return exts[mime] }
