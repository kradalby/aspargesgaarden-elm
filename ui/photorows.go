package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
)

// RowKind is the list a row belongs to; the lists differ in photo height
// and text padding.
type RowKind int

// The row kinds. The zero value is invalid.
const (
	_ RowKind = iota
	// RowFacility is a room on /lokaler.
	RowFacility
	// RowEvent is an event in the event list.
	RowEvent
)

func (k RowKind) classes() (photo, text string) {
	switch k {
	case RowFacility:
		return "object-cover h-48 w-full md:h-auto", "w-full h-full p-12 md:w-1/2 lg:w-2/3"
	case RowEvent:
		return "object-cover h-1/2 w-full md:h-auto", "w-full h-full px-12 py-8 break-words md:w-1/2 lg:w-2/3"
	}

	panic("ui: PhotoRows needs a RowKind")
}

// Row is one photo with its text beside it.
type Row struct {
	Photo images.Ref
	Alt   string
	By    Photographer
	Body  []elem.Node
}

// PhotoRows lists rows of photo and text, alternating sides and tint so a
// caller cannot get the rhythm wrong. The first photo is the page's hero.
func PhotoRows(kind RowKind, rows ...Row) elem.Node {
	photoClass, textClass := kind.classes()

	out := make([]elem.Node, 0, len(rows))

	for i, r := range rows {
		even := i%2 == 0
		priority := Lazy

		if i == 0 {
			priority = Hero
		}

		pic := Picture(PictureProps{Photo: r.Photo, Alt: r.Alt, Layout: LayoutRowPhoto, Priority: priority, Class: photoClass})

		out = append(out, elem.Div(attrs.Props{
			attrs.Class: cls("w-full flex flex-wrap",
				onClass(even, "bg-brown/20 flex-row-reverse md:pl-12"),
				onClass(!even, "flex-row")),
		},
			elem.Div(attrs.Props{attrs.Class: "w-full md:w-1/2 lg:w-1/3"},
				elem.Div(attrs.Props{attrs.Class: cls("w-full md:h-auto", onClass(!even, "float-right"))},
					PhotoCredit(r.By, pic))),
			elem.Div(attrs.Props{attrs.Class: textClass}, r.Body...),
		))
	}

	return elem.Div(attrs.Props{attrs.Class: "w-full flex flex-col"}, out...)
}
