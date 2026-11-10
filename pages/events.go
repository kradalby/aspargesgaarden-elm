package pages

import (
	"time"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/markdown"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/ui"
)

// teaserRunes bounds the gist an event list shows when an event has no
// summary: about three lines beside its photo.
const teaserRunes = 200

// Events is the event list (old /events): what is coming, big, each leading
// to its own page, then what has been, quietly. events must all have a
// main photo, as published ones do.
func Events(s Site, events []site.Event, now time.Time) *elem.Element {
	upcoming, past := site.Split(events, now)

	body := []elem.Node{ui.Container(elem.Div(nil, ui.Headline("Arrangementer")))}

	if len(upcoming) > 0 {
		body = append(body, ui.EventRows(eventItems(upcoming)...))
	} else {
		empty := "Ingen kommende arrangementer for øyeblikket."
		if len(past) > 0 {
			empty += " Se noen av våre tidligere arrangementer nedenfor for hva som kan komme i fremtiden!"
		}

		body = append(body, ui.Container(elem.P(attrs.Props{attrs.Class: "text-center py-12 text-gray-600"}, elem.Text(empty))))
	}

	if len(past) > 0 {
		body = append(body, ui.Container(ui.SectionHeading("Tidligere arrangementer"), ui.EventCards(eventItems(past)...)))
	}

	return s.page(ui.Meta{
		Title:       "Arrangementer i Tjølling, Larvik",
		Description: "Julemarked, kurs, konserter og andre arrangementer på Aspargesgården i Tjølling, Larvik. Se hva som skjer, og hva som har vært.",
		Path:        "/arrangementer",
	}, body...)
}

func eventItems(events []site.Event) []ui.EventItem {
	items := make([]ui.EventItem, len(events))
	for i, e := range events {
		items[i] = ui.EventItem{
			Href:      e.Path(),
			Title:     e.Title,
			DateLabel: e.DateLabel(),
			DateLine:  e.DateLine(),
			Date:      site.NorwegianDate(e.Start),
			Summary:   teaser(e),
			Cancelled: e.Cancelled,
			Photo:     photoRef(*e.MainPhoto, images.Card),
			Alt:       e.MainPhoto.Alt,
			By:        credit(e.MainPhoto.Photographer),
		}
	}

	return items
}

// teaser is the event's gist: its summary, or for an event written without
// one (all those from the old site) the opening of its text.
func teaser(e site.Event) string {
	if e.Summary != "" {
		return e.Summary
	}

	return markdown.Excerpt(e.BodyMD, teaserRunes)
}

func credit(p *site.Photographer) ui.Photographer {
	if p == nil {
		return ui.Photographer{}
	}

	return ui.Photographer{Name: p.Name, Website: p.URL}
}
