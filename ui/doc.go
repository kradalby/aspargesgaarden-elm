// Package ui is the component kit of aspargesgaarden.no: elem-go components
// that mirror the old Elm views (src/View/*.elm) class for class, so the site
// keeps its look. Pages compose these instead of writing markup, and every
// photo goes through Picture.
//
// A page:
//
//	func Kontakt(s Site) *elem.Element {
//	    return ui.Page(ui.PageProps{
//	        BaseURL: s.BaseURL,
//	        Menu:    s.Menu,
//	        Meta:    ui.Meta{Title: "Kontakt oss", Description: "…", Path: "/kontakt"},
//	    },
//	        ui.Container(
//	            ui.Headline("Kontakt oss"),
//	            ui.InfoLine(ui.IconPhone, "Telefon", "+47 90 91 98 12"),
//	        ),
//	    )
//	}
//
// Conventions:
//
//   - Class strings are Tailwind v4 utilities, written whole in the Go
//     source so the Tailwind scanner finds them (static/input.css @source).
//     Never build a class name from parts.
//   - One utility per CSS property per variant on an element: v4 orders
//     utilities itself, so a second one is not "the later one wins".
//   - Enum-like props (Layout, Priority, RowKind, Icon) are typed constants
//     whose zero value panics or is the safe choice; tests render every page,
//     so a mistake fails there and not in front of a visitor.
//   - Only origins in external.go may be loaded from; CSP enforces it.
//
// Files: page (document, head), header (logo, menu), footer (ContactPoint),
// container, headline, sectionheading, lead, picture, photocredit,
// photorows, backdrop, icon, infoline, contactlink, buttonlink, external
// (outside origins, CSP). Events: eventrows (coming, with EventItem),
// eventcards (past), eventcolumn, eventbanner, eventfacts, eventdetail,
// eventnotice.
package ui
