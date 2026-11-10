package site

import (
	"errors"
	"fmt"
	"strings"
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

// Photographer is a person credited on photos. One value serves every photo
// they took, so the admin picks rather than retypes, and a corrected link
// fixes every credit.
type Photographer struct {
	ID   int64
	Name string
	URL  string // optional
}

// Normalize returns p with the name's spaces collapsed and the link trimmed,
// and what is wrong with it.
func (p Photographer) Normalize() (Photographer, error) {
	p.Name = strings.Join(strings.Fields(p.Name), " ")
	p.URL = strings.TrimSpace(p.URL)

	var errs []error
	if p.Name == "" {
		errs = append(errs, ErrPhotographerName)
	}

	if p.URL != "" && !validLink(p.URL) {
		errs = append(errs, fmt.Errorf("%w: %q", ErrPhotographerURL, p.URL))
	}

	return p, errors.Join(errs...)
}

// nameKey is what makes two names one photographer: case and spacing are
// typing accidents, not different people.
func nameKey(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// Photo is one uploaded original and what is known about it.
type Photo struct {
	ID            int64
	SHA256        string
	MIME          string
	Width, Height int
	Alt           string
	// Photographer is the credit, nil for none; only its ID is read on writes.
	Photographer *Photographer
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
