package ui

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
)

// GalleryPhoto is one photo of a Gallery.
type GalleryPhoto struct {
	// Photo is the whole, uncropped photo: grid and lightbox show it as taken.
	Photo images.Ref
	Alt   string
	// Caption is shown under the photo in the lightbox; "" shows none.
	Caption string
	By      Photographer
}

// galleryAttr marks a gallery's grid. Page loads the lightbox when a page
// holds one, and the lightbox's boot.js finds the grids by it.
const galleryAttr = "data-gallery"

// GalleryLayout is where a Gallery sits; it decides the grid and the sizes
// hint of the thumbnails. The zero value is invalid.
type GalleryLayout int

// The gallery layouts.
const (
	_ GalleryLayout = iota
	// GalleryPage fills a page, as the old /gallery did: photos full width
	// on a phone, 20rem from md and 27rem from lg, wrapping.
	GalleryPage
	// GalleryColumn closes an EventColumn: two columns of photos with
	// rounded corners, as the old Christmas page had.
	GalleryColumn
)

func (l GalleryLayout) classes() (grid, item string, picture Layout) {
	switch l {
	case GalleryPage:
		return "w-full flex flex-row flex-wrap", "md:w-80 md:mr-6 md:mb-6 lg:w-108", LayoutGallery
	case GalleryColumn:
		return "grid grid-cols-2 gap-4 my-12 md:gap-6", "self-start overflow-hidden rounded-lg", LayoutHalfColumn
	}

	panic("ui: Gallery needs a GalleryLayout")
}

// Gallery is a grid of photos, each a link to the large photo, so it works
// with JavaScript off. On a page that shows one, Page loads the lightbox
// (elm/gallery), which opens over the grid instead of following the link.
// Any number of galleries may share a page. No photos renders nothing.
//
// The lightbox's contract: the grid carries data-gallery, a JSON list of
// the photos (lightboxPhoto) in grid order, and every photo link carries
// data-index, its place in that list.
func Gallery(layout GalleryLayout, photos ...GalleryPhoto) elem.Node {
	grid, item, pictureLayout := layout.classes()

	if len(photos) == 0 {
		return elem.None()
	}

	items := make([]elem.Node, len(photos))
	data := make([]lightboxPhoto, len(photos))

	for i, p := range photos {
		sources, img := encodings(p.Photo)
		largest := slices.MaxFunc(img.variants, func(a, b images.Variant) int { return a.Width - b.Width })

		pic := PictureProps{Photo: p.Photo, Alt: p.Alt, Layout: pictureLayout, Class: "w-full"}
		// A page-filling gallery's first photo sits under the headline,
		// above the fold: the page's main image.
		if layout == GalleryPage && i == 0 {
			pic.Priority = Hero
		}

		// The focus ring goes inside, where a rounded item cannot clip it.
		link := elem.A(attrs.Props{attrs.Href: largest.URL, "data-index": strconv.Itoa(i), attrs.Class: "block focus-visible:-outline-offset-4"}, Picture(pic))
		items[i] = PhotoCredit(p.By, link)
		if item != "" {
			items[i] = elem.Div(attrs.Props{attrs.Class: item}, items[i])
		}

		// Never nil: the decoder wants a list, not null.
		srcs := make([]lightboxSource, len(sources))
		for j, s := range sources {
			srcs[j] = lightboxSource{Type: s.mime(), Srcset: srcset(s.variants)}
		}

		data[i] = lightboxPhoto{
			Alt:     p.Alt,
			Caption: strings.TrimSpace(p.Caption),
			Width:   p.Photo.Width,
			Height:  p.Photo.Height,
			Src:     fallback(img.variants).URL,
			Srcset:  srcset(img.variants),
			Sources: srcs,
			By:      p.By.Name,
			Link:    p.By.Website,
		}
	}

	// Plain structs of strings and ints cannot fail to marshal.
	js, err := json.Marshal(data)
	if err != nil {
		panic("ui: gallery data: " + err.Error())
	}

	return elem.Div(attrs.Props{attrs.Class: grid, galleryAttr: string(js)}, items...)
}

// lightboxPhoto is what the lightbox knows of a photo; Lightbox.elm's
// photoDecoder reads it. Sources and the <img> mirror Picture's, so the
// lightbox offers the browser the same formats.
type lightboxPhoto struct {
	Alt     string           `json:"alt"`
	Caption string           `json:"caption,omitempty"`
	Width   int              `json:"width"`
	Height  int              `json:"height"`
	Src     string           `json:"src"`
	Srcset  string           `json:"srcset"`
	Sources []lightboxSource `json:"sources"`
	By      string           `json:"by,omitempty"`
	Link    string           `json:"link,omitempty"`
}

type lightboxSource struct {
	Type   string `json:"type"`
	Srcset string `json:"srcset"`
}

// holdsGallery reports whether a Gallery is among nodes or their children.
func holdsGallery(nodes []elem.Node) bool {
	return holds(nodes, func(e *elem.Element) bool {
		_, gallery := e.Attrs[galleryAttr]

		return gallery
	})
}

// holds reports whether an element among nodes or their children matches.
func holds(nodes []elem.Node, match func(*elem.Element) bool) bool {
	return slices.ContainsFunc(nodes, func(n elem.Node) bool {
		e, ok := n.(*elem.Element)

		return ok && (match(e) || holds(e.Children, match))
	})
}
