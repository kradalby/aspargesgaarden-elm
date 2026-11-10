package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// pictureOrPlaceholder is Picture, or a plain tinted box where a list has
// no photo to show. Only the admin's preview of a draft gets the box:
// published events always have their photo.
func pictureOrPlaceholder(p PictureProps) elem.Node {
	if len(p.Photo.Variants) == 0 {
		return elem.Div(attrs.Props{attrs.Class: cls(p.Class, "aspect-4/5 bg-brown/20"), attrs.AriaHidden: "true"})
	}

	return Picture(p)
}
