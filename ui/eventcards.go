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
			// At text-xl the title is normal-size text, which needs 4.5:1:
			// rust has 5.7 on white, the headings' tre only 3.5.
			elem.H3(attrs.Props{attrs.Class: "pt-2 font-display text-xl text-rust break-words hyphens-auto"},
				elem.A(attrs.Props{attrs.Href: it.Href, attrs.Class: "hover:text-sort"}, elem.Text(it.Title))),
			elem.P(attrs.Props{attrs.Class: "text-sm text-brown-text"}, cardDate(it)),
		}
		if it.Cancelled {
			kids = append(kids, cancelledBadge())
		}

		// Drawn first but last in the markup, so Tab reaches the title
		// before the photo's credit.
		kids = append(kids, elem.Div(attrs.Props{attrs.Class: "order-first w-full"}, PhotoCredit(it.By, hiddenLink(it.Href, pic))))

		// The cap keeps a card photo within images.Card's SharpWidth.
		cards[i] = elem.Li(attrs.Props{attrs.Class: "flex flex-col items-start max-w-192"}, kids...)
	}

	return elem.Ul(attrs.Props{attrs.Class: "grid grid-cols-2 gap-4 px-4 pb-8 md:grid-cols-3 md:gap-6 md:px-0 lg:grid-cols-4"}, cards...)
}

// cardDate is the day as written, with its machine-readable form when known.
func cardDate(it EventItem) elem.Node {
	if it.Day == "" {
		return elem.Text(it.Date)
	}

	return elem.Time(attrs.Props{attrs.Datetime: it.Day}, elem.Text(it.Date))
}
