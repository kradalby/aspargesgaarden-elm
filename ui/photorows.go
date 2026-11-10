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
		return "object-cover h-48 w-full md:h-auto", "w-full h-full p-12 md:w-1/2 lg:flex-1"
	case RowEvent:
		// A teaser is shorter than its photo; centred, it reads as one unit.
		return "object-cover h-1/2 w-full md:h-auto", "w-full px-12 py-8 break-words md:self-center md:w-1/2 lg:flex-1"
	}

	panic("ui: PhotoRows needs a RowKind")
}

// Row is one photo with its text beside it.
type Row struct {
	Photo images.Ref
	Alt   string
	By    Photographer
	Body  []elem.Node
	// Href makes the photo a link too, for rows that lead to a page; the
	// body must hold a visible link to the same place, since this one is
	// hidden from keyboards and screen readers to avoid saying it twice.
	Href string
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

		pic := pictureOrPlaceholder(PictureProps{Photo: r.Photo, Alt: r.Alt, Layout: LayoutRowPhoto, Priority: priority, Class: photoClass})
		if r.Href != "" {
			pic = hiddenLink(r.Href, pic)
		}

		out = append(out, elem.Div(attrs.Props{
			attrs.Class: cls("w-full flex flex-wrap",
				onClass(even, "bg-brown/20 flex-row-reverse md:pl-12"),
				onClass(!even, "flex-row")),
		},
			// The cap keeps an event's card within images.Card's SharpWidth.
			elem.Div(attrs.Props{attrs.Class: "w-full md:w-1/2 lg:w-1/3 lg:max-w-192"},
				elem.Div(attrs.Props{attrs.Class: cls("w-full md:h-auto", onClass(!even, "float-right"))},
					PhotoCredit(r.By, pic))),
			elem.Div(attrs.Props{attrs.Class: textClass}, r.Body...),
		))
	}

	return elem.Div(attrs.Props{attrs.Class: "w-full flex flex-col"}, out...)
}

// hiddenLink makes a photo clickable for pointer users only; the visible
// title link next to it is the one keyboards and screen readers get.
func hiddenLink(href string, picture elem.Node) elem.Node {
	return elem.A(attrs.Props{attrs.Href: href, attrs.Class: "block", attrs.Tabindex: "-1", attrs.AriaHidden: "true"}, picture)
}
