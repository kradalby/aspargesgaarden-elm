package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// EventCards is the quiet list of past events: a grid of small photos with
// title and date, each leading to the event's page.
func EventCards(items ...EventItem) elem.Node {
	cards := make([]elem.Node, len(items))
	for i, it := range items {
		pic := pictureOrPlaceholder(PictureProps{
			Photo: it.Photo, Alt: it.Alt, Layout: LayoutCardGrid,
			Class: "w-full aspect-4/5 object-cover",
		})

		kids := []elem.Node{
			PhotoCredit(it.By, hiddenLink(it.Href, pic)),
			elem.H3(attrs.Props{attrs.Class: "pt-2 font-display text-xl text-tre break-words hyphens-auto"},
				elem.A(attrs.Props{attrs.Href: it.Href, attrs.Class: "hover:text-rust"}, elem.Text(it.Title))),
			elem.P(attrs.Props{attrs.Class: "text-sm text-brown-text"}, elem.Text(it.Date)),
		}
		if it.Cancelled {
			kids = append(kids, cancelledBadge())
		}

		cards[i] = elem.Li(nil, kids...)
	}

	return elem.Ul(attrs.Props{attrs.Class: "grid grid-cols-2 gap-4 px-4 pb-8 md:grid-cols-3 md:gap-6 md:px-0 lg:grid-cols-4"}, cards...)
}
