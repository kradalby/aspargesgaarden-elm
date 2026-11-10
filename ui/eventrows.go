package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
)

// EventItem is an event as the event lists show it: enough to tempt a
// reader into the event's own page.
type EventItem struct {
	Href  string
	Title string
	// DateLabel and DateLine are the event's "Dato:" sentence; Date is the
	// short form for the quiet past list.
	DateLabel string
	DateLine  string
	Date      string
	Summary   string
	Cancelled bool
	Photo     images.Ref
	Alt       string
	By        Photographer
}

// EventRows is the list of coming events: big photo rows in the old event
// list's rhythm, each with the title, when, the gist and a way in.
func EventRows(items ...EventItem) elem.Node {
	rows := make([]Row, len(items))
	for i, it := range items {
		rows[i] = Row{Photo: it.Photo, Alt: it.Alt, By: it.By, Href: it.Href, Body: eventTeaser(it)}
	}

	return PhotoRows(RowEvent, rows...)
}

func eventTeaser(it EventItem) []elem.Node {
	body := []elem.Node{}
	if it.Cancelled {
		body = append(body, cancelledBadge())
	}

	body = append(body,
		elem.H2(attrs.Props{attrs.Class: "pt-2 pb-4 text-4xl font-display text-tre md:text-5xl"},
			elem.A(attrs.Props{attrs.Href: it.Href, attrs.Class: "hover:text-rust"}, elem.Text(it.Title))),
		EventDetail(it.DateLabel, it.DateLine),
	)

	if it.Summary != "" {
		body = append(body, elem.P(attrs.Props{attrs.Class: "pb-3 text-lg text-sort"}, elem.Text(it.Summary)))
	}

	return append(body, ButtonLink(ButtonLinkProps{Href: it.Href, Label: "Les mer", AriaLabel: "Les mer om " + it.Title}))
}

func cancelledBadge() elem.Node {
	return elem.P(attrs.Props{attrs.Class: "inline-block px-2 py-1 bg-rust text-white text-xs font-bold uppercase tracking-wide"}, elem.Text("Avlyst"))
}
