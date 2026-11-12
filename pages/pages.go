// Package pages builds the public pages of aspargesgaarden.no from the ui
// kit. HTML is only ever produced through elem-go, never a template
// language, so every value is escaped by construction.
//
// The fixed pages keep their words and photos here, next to the code that
// lays them out ("content in code").
package pages

import (
	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/booking"
	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/ui"
)

// Site is what every page needs from the running service.
type Site struct {
	// BaseURL is the public origin, without a trailing slash.
	BaseURL string
	// Menu is the main menu; build it with Menu.
	Menu []ui.MenuItem
	// Umami is the analytics site; the zero value loads no tracker.
	Umami ui.Umami
	// NoIndex is the beta site (--noindex); see seo.Meta.
	NoIndex bool
	// Calendar is the contact page's booking calendar; the zero value
	// shows none.
	Calendar booking.Calendar
}

// photoRef is what ui.Picture needs to show an uploaded photo as kind. A
// photo with no variants of that kind (a stored crop today's rules refuse)
// is shown whole rather than not at all; Intake refuses photos too small
// for that.
func photoRef(p site.Photo, kind images.Kind) images.Ref {
	ref, err := images.RefFor(p.Image(), kind)
	if err != nil {
		ref, _ = images.RefFor(p.Image(), images.Full)
	}

	return ref
}

func (s Site) page(meta ui.Meta, body ...elem.Node) *elem.Element {
	meta.NoIndex = s.NoIndex

	return ui.Page(ui.PageProps{BaseURL: s.BaseURL, Meta: meta, Menu: s.Menu, Umami: s.Umami}, body...)
}

// Titles are the site's own pages' titles by address: their <title> before
// the site name, and their names in llms.txt. Menu labels are shorter.
var Titles = map[string]string{
	"/":              "Bryllups- og selskapslokale i " + seo.Venue.Area + ", " + seo.Venue.Municipality,
	"/lokaler":       "Selskapslokaler: låven, fjøset og hagen",
	"/arrangementer": "Arrangementer i " + seo.Venue.Area + ", " + seo.Venue.Municipality,
	"/galleri":       "Galleri – bilder fra gården i " + seo.Venue.Area,
	"/om-oss":        "Om gården og vertskapet",
	"/kontakt":       "Kontakt oss – " + seo.Venue.Area + ", " + seo.Venue.Municipality,
}

// meta is the head of the page at path, titled from Titles.
func meta(path, description string) ui.Meta {
	return ui.Meta{Title: Titles[path], Description: description, Path: path}
}

// Menu is the main menu. events are the entries of events marked "Vis i
// menyen" (site.Menu); they follow Arrangementer, where Julemarked has
// always been.
func Menu(events ...site.MenuEntry) []ui.MenuItem {
	items := []ui.MenuItem{
		{Label: "Lokaler", Href: "/lokaler"},
		{Label: "Arrangementer", Href: "/arrangementer"},
	}
	for _, e := range events {
		items = append(items, ui.MenuItem{Label: e.Label, Href: e.Path})
	}

	return append(items,
		ui.MenuItem{Label: "Galleri", Href: "/galleri"},
		ui.MenuItem{Label: "Om gården", Href: "/om-oss"},
		ui.MenuItem{Label: "Kontakt", Href: "/kontakt"},
	)
}

// Fixed are the pages whose content is code, by their address. The server
// routes them from here, and each page's canonical path is its key.
var Fixed = map[string]func(Site) *elem.Element{
	"/":        Front,
	"/lokaler": Lokaler,
	"/om-oss":  OmOss,
	"/kontakt": Kontakt,
}
