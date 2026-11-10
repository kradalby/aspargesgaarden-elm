// Package pages builds the public pages of aspargesgaarden.no from the ui
// kit. HTML is only ever produced through elem-go, never a template
// language, so every value is escaped by construction.
//
// The fixed pages keep their words and photos here, next to the code that
// lays them out ("content in code").
package pages

import (
	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/ui"
)

// Site is what every page needs from the running service.
type Site struct {
	// BaseURL is the public origin, without a trailing slash.
	BaseURL string
	// Menu is the main menu; build it with Menu.
	Menu []ui.MenuItem
}

func (s Site) page(meta ui.Meta, body ...elem.Node) *elem.Element {
	return ui.Page(ui.PageProps{BaseURL: s.BaseURL, Meta: meta, Menu: s.Menu}, body...)
}

// Menu is the main menu. events are the events marked "Vis i menyen"; they
// follow Arrangementer, where Julemarked has always been.
func Menu(events ...ui.MenuItem) []ui.MenuItem {
	return append(append([]ui.MenuItem{
		{Label: "Muligheter", Href: "/lokaler"},
		{Label: "Arrangementer", Href: "/arrangementer"},
	}, events...),
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

// description is the old site's one description for every page.
// shortcut: shared by all pages until the SEO phase writes one per page.
const description = "Aspargesgården byr på sjarmerende og unike lokaler i landlige omgivelser, ved kysten i Vestfold."
