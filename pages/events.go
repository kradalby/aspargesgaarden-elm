package pages

import (
	"strings"
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
// to its own page, then what has been, quietly. The admin previews a draft
// in it, which may lack a title, a date or a photo.
func Events(s Site, events []site.Event, now time.Time) *elem.Element {
	upcoming, past := site.Split(events, now)

	body := []elem.Node{ui.Container(elem.Div(nil, ui.Headline("Arrangementer")))}

	if len(upcoming) > 0 {
		body = append(body, ui.EventRows(eventItems(upcoming)...))
	} else {
		empty := "Ingen kommende arrangementer for øyeblikket."
		if len(past) > 0 {
			empty += " Se tidligere arrangementer nedenfor for et inntrykk av hva som kan komme."
		}

		body = append(body, ui.Container(elem.P(attrs.Props{attrs.Class: "text-center py-12 text-gray-600"}, elem.Text(empty))))
	}

	if len(past) > 0 {
		body = append(body, ui.Container(ui.SectionHeading("Tidligere arrangementer"), ui.EventCards(eventItems(past)...)))
	}

	return s.page(meta("/arrangementer",
		"Julemarked, kurs, konserter og andre arrangementer på Aspargesgården i Tjølling, Larvik. Se hva som skjer, og hva som har vært."),
		body...)
}

func eventItems(events []site.Event) []ui.EventItem {
	items := make([]ui.EventItem, len(events))
	for i, e := range events {
		items[i] = ui.EventItem{
			Href:      e.Path(),
			Title:     e.Title,
			DateLabel: e.DateLabel(),
			DateLine:  e.DateLine(),
			Summary:   teaser(e),
			Cancelled: e.Cancelled,
		}

		if !e.Start.IsZero() {
			items[i].Date, items[i].Day = site.NorwegianDate(e.Start), e.Start.String()
		}

		// A draft in the admin's preview may have no photo yet; the list
		// then holds its place.
		if p := e.MainPhoto; p != nil {
			items[i].Photo, items[i].Alt, items[i].By = photoRef(*p, images.Card), altOf(*p), credit(p.Photographer)
		}
	}

	return items
}

// altOf is a photo's description. Of an event's photos only a draft's can lack one (the
// publish rules require it), and the admin's preview must still render it.
func altOf(p site.Photo) string {
	if strings.TrimSpace(p.Alt) == "" {
		return fallbackAlt
	}

	return p.Alt
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
