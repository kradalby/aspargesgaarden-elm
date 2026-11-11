package site

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/kradalby/asparges/images"
)

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

// nameKey is what makes two names one photographer: case, spacing and how
// a keyboard spells "å" (one letter, or a and a ring) are typing accidents,
// not different people.
func nameKey(name string) string {
	return strings.ToLower(norm.NFC.String(strings.Join(strings.Fields(name), " ")))
}

// Photo is one uploaded original and what is known about it.
type Photo struct {
	ID            int64
	SHA256        string
	MIME          string
	Width, Height int
	Alt           string
	// Caption is shown under the photo in the lightbox; "" for none.
	Caption string
	// Photographer is the credit, nil for none; only its ID is read on writes.
	Photographer *Photographer
	// Crops holds the rectangles set so far; a missing kind is cut centred
	// (images.CentredCrop), soft where the photo is too small for it.
	Crops     map[images.Kind]images.Rect
	CreatedAt time.Time
	// UpdatedAt is the last change. In an event being saved it is the
	// version the editor loaded; see writePhotoMeta.
	UpdatedAt time.Time
}

// checkCrop is the rule every stored crop keeps, whoever writes it: a kind
// that takes a crop, inside the photo. Aspect ratio and minimum size are
// images.ValidateCrop's, for the crops the admin sends.
func (p Photo) checkCrop(kind images.Kind, r images.Rect) error {
	if !slices.Contains(images.CropKinds, kind) {
		return fmt.Errorf("%w: %q", images.ErrKind, kind)
	}

	if r.X < 0 || r.Y < 0 || r.W <= 0 || r.H <= 0 || r.X+r.W > p.Width || r.Y+r.H > p.Height {
		return fmt.Errorf("%w: %+v in %dx%d", images.ErrCropOutside, r, p.Width, p.Height)
	}

	return nil
}
