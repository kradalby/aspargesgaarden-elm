package site

import (
	"fmt"
	"time"
)

// CropKind names one of the fixed crops every photo can carry. The full,
// uncropped photo is not a kind: it needs no rectangle.
type CropKind string

const (
	CropCard   CropKind = "card"   // 4:5, event list
	CropBanner CropKind = "banner" // 16:9, event page
	CropShare  CropKind = "share"  // 1200:630, Open Graph
)

// CropKinds lists every kind, in a fixed order.
var CropKinds = []CropKind{CropCard, CropBanner, CropShare}

// Rect is a crop rectangle in source pixels.
type Rect struct {
	X, Y, W, H int
}

// Photo is one uploaded original and what is known about it.
type Photo struct {
	ID               int64
	SHA256           string
	MIME             string
	Width, Height    int
	Alt              string
	PhotographerName string
	PhotographerURL  string
	// Crops holds the rectangles set so far; a missing kind means images/
	// falls back to a centred crop.
	Crops     map[CropKind]Rect
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CheckCrop checks that r is a known kind and lies inside the photo. Aspect
// ratio and minimum size are images/' rules, which know the output sizes.
func (p Photo) CheckCrop(kind CropKind, r Rect) error {
	switch kind {
	case CropCard, CropBanner, CropShare:
	default:
		return fmt.Errorf("%w: %q", ErrCropKind, kind)
	}

	if r.X < 0 || r.Y < 0 || r.W <= 0 || r.H <= 0 || r.X+r.W > p.Width || r.Y+r.H > p.Height {
		return fmt.Errorf("%w: %+v in %dx%d", ErrCropOutside, r, p.Width, p.Height)
	}

	return nil
}
