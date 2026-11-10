package images

import (
	"errors"
	"fmt"
	"math"
)

var (
	ErrCropOutside = errors.New("crop is not inside the photo")
	ErrCropAspect  = errors.New("crop has the wrong aspect ratio")
	ErrTooSmall    = errors.New("photo is too small")
	ErrKind        = errors.New("kind takes no crop")
)

// MinWidth is the narrowest crop, in source pixels, a kind accepts. Card and
// Full need at least the smallest ladder step, or there is no variant at all.
// A banner spans the content column, and under 800 pixels it is soft on
// every screen. Share is always 1200×630 and is never upscaled, so a photo
// narrower than 1200 pixels (or shorter than 630) gets no share image; the
// admin learns this from DefaultCrop returning ErrTooSmall at upload.
func (k Kind) MinWidth() int {
	switch k {
	case Banner:
		return 800
	case Share:
		return shareWidth
	case Card, Full:
		return widths[0]
	default:
		return widths[0]
	}
}

// The cropper rounds to whole pixels, so a crop may miss the exact ratio. The
// pipeline cuts a window of the exact ratio from it, so the slack only
// decides how much of the edge is lost, never stretches the picture.
const (
	aspectSlack   = 0.01
	aspectSlackPx = 2
)

// ValidateCrop checks a crop the admin sends for kind k on a w×h (upright)
// photo: inside the photo, the kind's aspect ratio, and at least MinWidth.
// It is the trust boundary for crops; the pipeline assumes it passed.
func ValidateCrop(k Kind, w, h int, r Rect) error {
	an, ad := k.aspect()
	if an == 0 {
		return fmt.Errorf("%w: %q", ErrKind, k)
	}

	if r.X < 0 || r.Y < 0 || r.W <= 0 || r.H <= 0 || r.X+r.W > w || r.Y+r.H > h {
		return fmt.Errorf("%w: %+v in %dx%d", ErrCropOutside, r, w, h)
	}

	want := float64(r.W) * float64(ad) / float64(an)
	if math.Abs(float64(r.H)-want) > max(aspectSlackPx, want*aspectSlack) {
		return fmt.Errorf("%w: %dx%d for %s", ErrCropAspect, r.W, r.H, k)
	}

	// The exact-ratio window can be the slack taller than the crop; it must
	// still fit the photo.
	if int(math.Round(want)) > h {
		return fmt.Errorf("%w: a %s %d wide needs %.0f rows, the photo has %d", ErrCropOutside, k, r.W, want, h)
	}

	if r.W < k.MinWidth() {
		return fmt.Errorf("%w: %s needs a crop at least %d pixels wide, got %d", ErrTooSmall, k, k.MinWidth(), r.W)
	}

	return nil
}

// DefaultCrop is the largest centred crop of kind k on a w×h photo: the
// starting point in the admin and the crop the import uses. ErrTooSmall
// means the photo cannot be used for that kind at all.
func DefaultCrop(k Kind, w, h int) (Rect, error) {
	if an, _ := k.aspect(); an == 0 && k != Full {
		return Rect{}, fmt.Errorf("%w: %q", ErrKind, k)
	}

	r := CentredCrop(k, w, h)
	if r.W < k.MinWidth() {
		return Rect{}, fmt.Errorf("%w: %s needs at least %d pixels across, the photo gives %d", ErrTooSmall, k, k.MinWidth(), r.W)
	}

	return r, nil
}

// CentredCrop is the largest centred rectangle of kind k's shape on a w×h
// photo, however small: where the admin's crop tool starts for a kind the
// photo has no crop of. Full, and a kind without a shape, is the whole photo.
func CentredCrop(k Kind, w, h int) Rect {
	r := Rect{W: w, H: h}

	an, ad := k.aspect()
	switch {
	case an == 0:
	case w*ad > h*an:
		r.W, r.H = h*an/ad, h
	default:
		r.W, r.H = w, w*ad/an
	}

	r.X, r.Y = (w-r.W)/2, (h-r.H)/2

	return r
}
