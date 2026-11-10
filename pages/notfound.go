package pages

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/markdown"
	"github.com/kradalby/asparges/ui"
)

// NotFound answers every address the site does not have. It has no
// canonical address of its own.
func NotFound(s Site) *elem.Element {
	return s.page(ui.Meta{Title: "Fant ikke siden", Description: "Siden du leter etter finnes ikke, eller den har flyttet."},
		ui.Container(
			elem.Div(nil, ui.Headline("Fant ikke siden")),
			elem.Div(attrs.Props{attrs.Class: "px-5 md:px-0"}, markdown.Render(
				"Siden du leter etter finnes ikke, eller den har flyttet. "+
					"Gå til [forsiden](/), se [arrangementene våre](/arrangementer) eller [ta kontakt](/kontakt).")...),
		),
	)
}
