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

	if len(e.ExtraPhotos) > 0 {
		kids = append(kids, extraPhotos(e.ExtraPhotos))
	}

	return s.page(ui.Meta{Title: e.Title, Description: teaser(e), Path: e.Path()}, ui.EventColumn(kids...))
}

// extraPhotos shows an event's further photos, whole and uncropped.
//
// shortcut: a plain grid until the gallery component (grid and lightbox)
// lands; then this returns ui.Gallery for the same photos.
func extraPhotos(photos []site.Photo) elem.Node {
	cells := make([]elem.Node, len(photos))
	for i, p := range photos {
		cells[i] = ui.PhotoCredit(credit(p.Photographer), ui.Picture(ui.PictureProps{
			Photo: photoRef(p, images.Full), Alt: p.Alt, Layout: ui.LayoutHalfColumn, Class: "w-full h-auto",
		}))
	}

	return elem.Div(attrs.Props{attrs.Class: "grid grid-cols-2 gap-4 my-12 md:gap-6"}, cells...)
}
