package pages

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/ui"
)

// calendar is the farm's public booking calendar.
const calendar = ui.CalendarOrigin + "/calendar/embed?src=klatrerosen.no_hgf7kiksq8hrmilogq5nd6f4rs%40group.calendar.google.com&ctz=Europe%2FOslo"

// Kontakt has the contact details and the booking calendar (old /contact).
func Kontakt(s Site) *elem.Element {
	return s.page(ui.Meta{Title: "Kontakt oss", Description: description, Path: "/kontakt"},
		ui.Container(
			elem.Div(nil, ui.Headline("Kontakt oss")),
			elem.Div(attrs.Props{attrs.Class: "flex flex-row flex-wrap"},
				elem.Div(attrs.Props{attrs.Class: "w-full flex flex-col flex-wrap md:w-1/2"},
					elem.Div(attrs.Props{attrs.Class: "md:pt-2"},
						ui.InfoLine(ui.IconPhone, "Telefon", "+47 90 91 98 12"),
						ui.InfoLine(ui.IconMail, "E-post", "kristine (at) aspargesgaarden (dot) no"),
						ui.InfoLine(ui.IconLocation, "Adresse", "Østbyveien 75, 3280 Tjodalyng"),
					)),
				elem.Div(attrs.Props{attrs.Class: "w-full md:w-1/2"},
					elem.IFrame(attrs.Props{
						attrs.Class:   "p-2 pt-4 h-96 w-full",
						attrs.Src:     calendar,
						attrs.Title:   "Kalender for Aspargesgården",
						attrs.Loading: "lazy",
					})),
			),
		),
	)
}
