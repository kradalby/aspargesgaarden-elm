package ui

import (
	"slices"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
)

// EventPoster is an event's poster in its text: whole, at most the column's
// width and never wider than its own pixels, so its words stay sharp. It
// links to the largest file, which a phone can zoom into.
func EventPoster(photo images.Ref, alt string, by Photographer) elem.Node {
	_, img := encodings(photo)
	largest := slices.MaxFunc(img.variants, func(a, b images.Variant) int { return a.Width - b.Width })

	return elem.Div(attrs.Props{attrs.Class: "my-8"},
		PhotoCredit(by, elem.A(attrs.Props{attrs.Href: largest.URL, attrs.Class: "block"},
			Picture(PictureProps{
				Photo: photo, Alt: alt, Layout: LayoutBanner,
				Class: "block mx-auto h-auto max-w-full",
			}))))
}
