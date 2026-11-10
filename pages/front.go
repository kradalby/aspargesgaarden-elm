package pages

import (
	"fmt"

	"github.com/chasefleming/elem-go"

	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

// frontDescription says what the venue is for, where, and for how many.
var frontDescription = fmt.Sprintf(
	"Låvebryllup, konfirmasjon, dåp, minnestund og firmafest på en gård i %s, %s, ved kysten i Vestfold. Låven tar %d gjester, fjøset %d.",
	seo.Venue.Area, seo.Venue.Municipality, seo.Venue.Guests("Låven"), seo.Venue.Guests("Fjøset"))

// Front is the front page: the farm photo, filling the screen.
func Front(s Site) *elem.Element {
	return s.page(ui.Meta{
		Title:       "Bryllups- og selskapslokale i " + seo.Venue.Area + ", " + seo.Venue.Municipality,
		Description: frontDescription,
		Path:        "/",
		JSONLD:      []any{seo.VenueNode(s.BaseURL), seo.WebSiteNode(s.BaseURL)},
	},
		ui.Backdrop("Aspargesgården – selskapslokaler ved kysten i Vestfold", static.Photo("forside")),
	)
}
