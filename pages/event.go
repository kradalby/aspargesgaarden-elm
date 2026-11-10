package pages

import (
	"fmt"
	"time"

	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/markdown"
	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/ui"
)

// Event is an event's own page: banner, the quick facts with the ticket
// button, then the whole text and the further photos. A past event keeps
// its page, marked as over, so links and search results still land.
func Event(s Site, e site.Event, now time.Time) *elem.Element {
	over := e.IsOver(now)

	kids := []elem.Node{ui.Headline(e.Title)}

	switch {
	case e.Cancelled:
		kids = append(kids, ui.EventNotice(ui.NoticeCancelled))
	case over:
		kids = append(kids, ui.EventNotice(ui.NoticeOver))
	}

	if p := e.MainPhoto; p != nil {
		kids = append(kids, ui.EventBanner(photoRef(*p, images.Banner), altOf(*p), credit(p.Photographer)))
	}

	facts := ui.EventFactsProps{DateLabel: e.DateLabel(), DateLine: e.DateLine(), Price: e.PriceText}
	if !over && !e.Cancelled {
		facts.TicketURL = e.TicketURL
	}

	kids = append(kids, ui.EventFacts(facts))

	if e.Summary != "" {
		kids = append(kids, ui.Lead(e.Summary))
	}

	kids = append(kids, markdown.RenderEvent(e.BodyMD)...)

	// The further photos, whole, with the lightbox over them; nothing
	// when there are none.
	kids = append(kids, ui.Gallery(ui.GalleryColumn, galleryPhotos(e.ExtraPhotos)...))

	return s.page(eventMeta(s, e, facts.TicketURL), ui.EventColumn(kids...))
}

// descriptionRunes is about what search results show of a description.
const descriptionRunes = 160

// eventMeta names the event with its first day, since a yearly event keeps
// its title, and opens the description with when and where.
func eventMeta(s Site, e site.Event, ticketURL string) ui.Meta {
	m := ui.Meta{
		Title:       e.Title,
		Description: seo.Clip(teaser(e), descriptionRunes),
		Path:        e.Path(),
		JSONLD:      seo.EventNodes(s.BaseURL, seo.EventInput{Event: e, Description: teaser(e), TicketURL: ticketURL}),
	}

	// Only a draft in the admin's preview has no date yet.
	if !e.Start.IsZero() {
		day := site.NorwegianDate(e.Start)
		m.Title += " – " + day
		m.Description = seo.Clip(fmt.Sprintf("%s på %s i %s, %s: %s",
			day, seo.Venue.Name, seo.Venue.Area, seo.Venue.Municipality, teaser(e)), descriptionRunes)
	}

	if e.MainPhoto != nil {
		m.Image, _ = seo.PhotoImage(*e.MainPhoto)
	}

	return m
}
