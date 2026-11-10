package ui

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
)

// Layout is where a picture sits on the page. It decides the sizes hint, so
// the browser fetches the smallest variant that is still sharp.
type Layout int

// The layouts. The zero value is invalid so a forgotten Layout panics in
// tests instead of guessing a size.
const (
	_ Layout = iota
	// LayoutFullscreen covers the viewport (the front page backdrop).
	LayoutFullscreen
	// LayoutRowPhoto is the photo column of PhotoRows: full width on a
	// phone, half from md, a third from lg.
	LayoutRowPhoto
	// LayoutBanner is the photo at the top of an event page, as wide as
	// EventColumn.
	LayoutBanner
	// LayoutCardGrid is a photo in EventCards: half the width on a phone,
	// a third from md, a quarter from lg.
	LayoutCardGrid
	// LayoutHalfColumn is a photo in a two-column grid inside EventColumn.
	LayoutHalfColumn
	// LayoutGallery is a Gallery thumbnail: full width on a phone, 20rem
	// from md, 27rem from lg.
	LayoutGallery
)

func (l Layout) sizes(photo images.Ref) string {
	switch l {
	case LayoutFullscreen:
		// object-cover on a portrait screen is as wide as the screen is tall
		// times the photo's aspect, far wider than 100vw on a phone.
		if photo.Height == 0 {
			return "100vw"
		}

		return "max(100vw, " + strconv.Itoa((100*photo.Width+photo.Height-1)/photo.Height) + "vh)"
	case LayoutRowPhoto:
		return "(min-width: 64rem) 33vw, (min-width: 48rem) 50vw, 100vw"
	case LayoutBanner:
		return "(min-width: 56rem) 56rem, 100vw"
	case LayoutCardGrid:
		return "(min-width: 64rem) 25vw, (min-width: 48rem) 33vw, 50vw"
	case LayoutHalfColumn:
		return "(min-width: 56rem) 28rem, 50vw"
	case LayoutGallery:
		return "(min-width: 64rem) 27rem, (min-width: 48rem) 20rem, 100vw"
	}

	panic("ui: Picture needs a Layout")
}

// Priority says whether a picture is the page's main image.
type Priority int

// The priorities. Lazy is the zero value: a picture is only fetched early
// when a caller says so.
const (
	// Lazy loads when the picture nears the viewport.
	Lazy Priority = iota
	// Hero loads at once with high priority: the largest image above the fold.
	Hero
)

// PictureProps configures a Picture.
type PictureProps struct {
	Photo images.Ref
	// Alt describes the photo. Leave it empty only with Decorative set.
	Alt string
	// Decorative marks a photo that adds nothing a reader would miss.
	Decorative bool
	Layout     Layout
	Priority   Priority
	// Class styles the <img>; for kit components, not pages.
	Class string
}

// formats in the order the browser should try them; the last is the <img>.
var formats = []images.Format{images.AVIF, images.WebP, images.JPEG}

// fallbackWidth is the <img src> width for browsers without srcset support.
const fallbackWidth = 1200

// Picture is the site's only way to show a photo: AVIF and WebP sources and
// an <img> with src, srcset, sizes, intrinsic width and height, and alt.
func Picture(p PictureProps) elem.Node {
	switch {
	case p.Alt == "" && !p.Decorative:
		panic("ui: Picture needs Alt, or Decorative for a purely decorative photo")
	case p.Alt != "" && p.Decorative:
		panic("ui: a Decorative Picture has no Alt")
	}

	sizes := p.Layout.sizes(p.Photo)
	sources, img := encodings(p.Photo)

	kids := make([]elem.Node, 0, len(sources)+1)
	for _, s := range sources {
		kids = append(kids, elem.Source(attrs.Props{
			attrs.Type:  s.mime(),
			"srcset":    srcset(s.variants),
			attrs.Sizes: sizes,
		}))
	}

	props := attrs.Props{
		attrs.Src:    fallback(img.variants).URL,
		"srcset":     srcset(img.variants),
		attrs.Sizes:  sizes,
		attrs.Width:  strconv.Itoa(p.Photo.Width),
		attrs.Height: strconv.Itoa(p.Photo.Height),
		attrs.Alt:    p.Alt,
	}
	if p.Class != "" {
		props[attrs.Class] = p.Class
	}

	switch p.Priority {
	case Hero:
		props["fetchpriority"] = "high"
	case Lazy:
		props[attrs.Loading] = "lazy"
		props["decoding"] = "async"
	}

	return elem.Picture(nil, append(kids, elem.Img(props))...)
}

// encoding is one format of a photo with its variants.
type encoding struct {
	format   images.Format
	variants []images.Variant
}

func (e encoding) mime() string { return "image/" + string(e.format) }

// encodings splits a photo's variants by format: the <source>s in order of
// preference, and the least preferred format present, which is the <img>.
func encodings(photo images.Ref) (sources []encoding, img encoding) {
	byFormat := map[images.Format][]images.Variant{}
	for _, v := range photo.Variants {
		byFormat[v.Format] = append(byFormat[v.Format], v)
	}

	var present []encoding

	for _, f := range formats {
		if len(byFormat[f]) > 0 {
			present = append(present, encoding{format: f, variants: byFormat[f]})
		}
	}

	if len(present) == 0 {
		panic("ui: Picture of a photo without variants")
	}

	return present[:len(present)-1], present[len(present)-1]
}

func srcset(vs []images.Variant) string {
	vs = slices.SortedFunc(slices.Values(vs), func(a, b images.Variant) int { return cmp.Compare(a.Width, b.Width) })

	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = v.URL + " " + strconv.Itoa(v.Width) + "w"
	}

	return strings.Join(parts, ", ")
}

func fallback(vs []images.Variant) images.Variant {
	return slices.MinFunc(vs, func(a, b images.Variant) int {
		return cmp.Compare(abs(a.Width-fallbackWidth), abs(b.Width-fallbackWidth))
	})
}

func abs(n int) int { return max(n, -n) }
