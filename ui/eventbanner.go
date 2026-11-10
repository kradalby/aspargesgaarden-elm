package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
)

// EventBanner is the wide photo at the top of an event page, the page's
// main image, with its credit.
func EventBanner(photo images.Ref, alt string, by Photographer) elem.Node {
	return elem.Div(attrs.Props{attrs.Class: "my-4"},
		PhotoCredit(by, Picture(PictureProps{
			Photo: photo, Alt: alt, Layout: LayoutBanner, Priority: Hero,
			Class: "w-full h-auto",
		})))
}
