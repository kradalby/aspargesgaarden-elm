package pages

import (
	"time"

	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/markdown"
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
		kids = append(kids, ui.EventBanner(photoRef(*p, images.Banner), p.Alt, credit(p.Photographer)))
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

	return s.page(ui.Meta{Title: e.Title, Description: teaser(e), Path: e.Path()}, ui.EventColumn(kids...))
}
