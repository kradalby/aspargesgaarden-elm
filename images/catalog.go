package images

import (
	"context"
	"fmt"
	"math"
	"regexp"
)

var shaRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Photo is what this package needs to know about one photo row.
type Photo struct {
	SHA256 string // of the original; also its file name
	MIME   string // as Intake returned it
	// Upright size, as Intake returned it. Crops are in this frame.
	Width, Height int
	// The current crop per kind. Full is implicit and never listed.
	Crops map[Kind]Rect
}

// Catalog is the database's view of photos: the source of truth for which
// photos and crops exist, and so for which files may exist.
type Catalog interface {
	// Photo returns the photo whose original has this sha256, or an error
	// wrapping ErrNotFound.
	Photo(ctx context.Context, sha256 string) (Photo, error)
	// Photos returns every photo. Reconcile deletes what this list does not
	// explain, so an incomplete answer must be an error, never a short list.
	Photos(ctx context.Context) ([]Photo, error)
}

// group is the variants of one kind of one photo: one decode, many files.
type group struct {
	kind  Kind
	rect  Rect
	specs []VariantSpec // in formats order, then by width
}

// height is the pixel height of a variant w pixels wide cut from r.
func (k Kind) height(r Rect, w int) int {
	an, ad := k.aspect()
	if an == 0 {
		return int(math.Round(float64(w) * float64(r.H) / float64(r.W)))
	}

	return w * ad / an
}

// groups lists the variants a photo should have, Full first because the
// admin preview shows it. A stored crop that no longer validates is skipped
// rather than built wrong; the admin must set it again.
func groups(p Photo) []group {
	if !shaRE.MatchString(p.SHA256) {
		return nil
	}

	var out []group

	for _, k := range []Kind{Full, Card, Banner, Share} {
		var (
			r   Rect
			err error
		)

		if k == Full {
			r, err = DefaultCrop(Full, p.Width, p.Height)
		} else {
			var ok bool
			if r, ok = p.Crops[k]; !ok {
				continue
			}

			err = ValidateCrop(k, p.Width, p.Height, r)
		}

		if err != nil {
			continue
		}

		g := group{kind: k, rect: r}
		ws, fs := widths, formats
		if k == Share {
			ws, fs = []int{shareWidth}, []Format{JPEG}
		}

		for _, f := range fs {
			for _, w := range ws {
				if w <= r.W {
					g.specs = append(g.specs, VariantSpec{Photo: p.SHA256, Kind: k, Crop: r.ID(), Width: w, Format: f})
				}
			}
		}

		out = append(out, g)
	}

	return out
}

// Expected lists every variant a photo should have.
func Expected(p Photo) []VariantSpec {
	var out []VariantSpec
	for _, g := range groups(p) {
		out = append(out, g.specs...)
	}

	return out
}

// RefFor builds the Ref a page uses to show photo p as kind k. It needs no
// files: names are derived, and the handler builds what is missing.
func RefFor(p Photo, k Kind) (Ref, error) {
	return refFor(p, k, VariantSpec.URL)
}

func refFor(p Photo, k Kind, url func(VariantSpec) string) (Ref, error) {
	for _, g := range groups(p) {
		if g.kind != k {
			continue
		}

		var ref Ref
		for _, s := range g.specs {
			ref.Variants = append(ref.Variants, Variant{Format: s.Format, Width: s.Width, URL: url(s)})
			if s.Width > ref.Width {
				ref.Width, ref.Height = s.Width, k.height(g.rect, s.Width)
			}
		}

		return ref, nil
	}

	return Ref{}, fmt.Errorf("photo %s has no %s variants: %w", p.SHA256, k, ErrNotFound)
}
