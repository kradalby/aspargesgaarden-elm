package images

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// build writes specs, all of group g of photo p, from the original at orig.
// It decodes the original once (stage 1), cuts the crop window (stage 2) and
// encodes each file from that small master (stage 3). Every file is checked,
// synced and renamed into place, so a reader never sees a partial file.
func (v *vips) build(ctx context.Context, p Photo, orig string, g group, specs []VariantSpec, dst func(VariantSpec) string) error {
	hdr, err := v.preflight(ctx, orig, p.MIME)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}

	// Crops are in the frame the catalogue recorded; a different original
	// would put them in the wrong place.
	if hdr.width != p.Width || hdr.height != p.Height {
		return fmt.Errorf("original is %dx%d, the catalogue says %dx%d", hdr.width, hdr.height, p.Width, p.Height)
	}

	wmax := 0
	for _, s := range specs {
		wmax = max(wmax, s.Width)
	}

	r := g.rect
	hmax := g.kind.height(r, wmax)

	// Scale so the crop covers wmax×hmax. The extra pixel absorbs vips'
	// rounding of the scaled height; the window loses at most that.
	sw := wmax
	if g.kind != Full {
		scale := max(float64(wmax)/float64(r.W), float64(hmax)/float64(r.H))
		sw = min(p.Width, int(math.Ceil(float64(p.Width)*scale))+1)
	}

	dir, err := os.MkdirTemp(v.tmp, "build-")
	if err != nil {
		return fmt.Errorf("build dir: %w", err)
	}
	defer os.RemoveAll(dir)

	master := filepath.Join(dir, "scaled.v")
	if err := v.decode(ctx, orig, master, sw); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	kw, kh, err := v.size(ctx, master)
	if err != nil {
		return err
	}

	if g.kind == Full {
		hmax = kh
	} else {
		cx := min(int(math.Round(float64(r.X)*float64(kw)/float64(p.Width))), kw-wmax)
		cy := min(int(math.Round(float64(r.Y)*float64(kh)/float64(p.Height))), kh-hmax)
		if cx < 0 || cy < 0 {
			return fmt.Errorf("crop %+v does not fit %dx%d scaled to %dx%d", r, p.Width, p.Height, kw, kh)
		}

		cropped := filepath.Join(dir, "crop.v")
		if err := v.crop(ctx, master, cropped, cx, cy, wmax, hmax); err != nil {
			return fmt.Errorf("crop: %w", err)
		}

		master = cropped
	}

	var errs []error
	dirs := map[string]bool{}
	for _, s := range specs {
		out := dst(s)
		err := v.encodeOne(ctx, master, dir, s, float64(hmax)*float64(s.Width)/float64(wmax), out)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
			continue
		}

		dirs[filepath.Dir(out)] = true
	}

	for d := range dirs {
		if err := syncDir(d); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// encodeOne writes one variant to out via a work file, checking its size
// against what the spec promises before it becomes visible.
func (v *vips) encodeOne(ctx context.Context, master, dir string, s VariantSpec, wantH float64, out string) error {
	work := filepath.Join(dir, s.file())
	if err := v.encode(ctx, master, work, s.Width, s.Format); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	w, h, err := v.size(ctx, work)
	if err != nil {
		return fmt.Errorf("check output: %w", err)
	}

	if w != s.Width || math.Abs(float64(h)-wantH) > 1 {
		return fmt.Errorf("output is %dx%d, want %dx%.0f", w, h, s.Width, wantH)
	}

	if err := syncFile(work); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(out), dirPerm); err != nil {
		return fmt.Errorf("variant dir: %w", err)
	}

	//nolint:gosec // both paths are derived from a VariantSpec, never from input
	if err := os.Rename(work, out); err != nil {
		return fmt.Errorf("move into place: %w", err)
	}

	return nil
}

func syncFile(path string) error {
	f, err := os.Open(path) //nolint:gosec // our own file or folder
	if err != nil {
		return fmt.Errorf("sync %s: %w", path, err)
	}
	defer f.Close()

	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", path, err)
	}

	return nil
}

// syncDir makes renames into dir durable.
func syncDir(dir string) error { return syncFile(dir) }
