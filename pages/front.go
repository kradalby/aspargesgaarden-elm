package pages

import (
	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

// Front is the front page: the farm photo, filling the screen.
func Front(s Site) *elem.Element {
	return s.page(ui.Meta{Title: "Velkommen", Description: description, Path: "/"},
		ui.Backdrop("Aspargesgården – selskapslokaler ved kysten i Vestfold", static.Photo("forside")),
	)
}
