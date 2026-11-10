package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// EventFactsProps configures EventFacts. Empty fields are left out.
type EventFactsProps struct {
	DateLabel, DateLine string
	Price               string
	// TicketURL leads to the ticket shop; pages leave it empty once the
	// event is over or cancelled.
	TicketURL string
}

// EventFacts is the quick information at the top of an event page: when,
// what it costs, and the ticket button.
func EventFacts(p EventFactsProps) elem.Node {
	kids := []elem.Node{EventDetail(p.DateLabel, p.DateLine)}

	if p.Price != "" {
		kids = append(kids, EventDetail("Pris:", p.Price))
	}

	if p.TicketURL != "" {
		kids = append(kids, ButtonLink(ButtonLinkProps{Href: p.TicketURL, Label: "Kjøp billett", External: true}))
	}

	return elem.Div(attrs.Props{attrs.Class: "bg-brown/20 px-6 pt-6 pb-4 my-6"}, kids...)
}
