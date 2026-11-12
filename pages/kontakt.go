package pages

import (
	"fmt"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/booking"
	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/ui"
)

// Kontakt has the contact details and, when the server reads the booking
// calendar, the taken days (old /contact).
func Kontakt(s Site) *elem.Element {
	m := meta("/kontakt", fmt.Sprintf("Ring %s eller send e-post for å leie låven eller fjøset. Aspargesgården ligger i %s i %s.",
		seo.Venue.Phone, seo.Venue.AddressLine(), seo.Venue.Municipality))
	m.JSONLD = []any{seo.VenueNode(s.BaseURL)}

	return s.page(m,
		ui.Container(
			elem.Div(nil, ui.Headline("Kontakt oss")),
			elem.Div(attrs.Props{attrs.Class: "md:pt-2"},
				ui.InfoLine(ui.IconPhone, "Telefon", seo.Venue.Phone, "tel:"+seo.Venue.PhoneE164),
				ui.InfoLine(ui.IconMail, "E-post", "kristine (at) aspargesgaarden (dot) no", ""),
				ui.InfoLine(ui.IconLocation, "Adresse", seo.Venue.AddressLine(), ""),
			),
			calendar(s.Calendar),
		),
	)
}

// shownMonths is how many months the calendar draws at once; Booking.elm's
// shown must match.
const shownMonths = 3

// calendar is the booking calendar, or nothing when there is none.
func calendar(c booking.Calendar) elem.Node {
	if !c.On() {
		return elem.None()
	}

	first := c.First()

	months := make([]ui.BookingMonth, shownMonths)
	for i := range months {
		months[i] = bookingMonth(c, first.MonthStart(i))
	}

	upcoming := c.Upcoming()
	taken := make([]string, len(upcoming))

	for i, d := range upcoming {
		taken[i] = d.String()
	}

	return ui.BookingCalendar(ui.BookingCalendarProps{
		Phone:  seo.Venue.Phone,
		Tel:    "tel:" + seo.Venue.PhoneE164,
		Range:  booking.MonthRange(first, first.MonthStart(shownMonths-1)),
		Months: months,
		Edge:   "Kalenderen går til " + site.MonthName(c.Last.Month) + fmt.Sprintf(" %d. Spør oss om datoer etter det.", c.Last.Year),
		Data: ui.BookingData{
			Today: c.Today.String(),
			Last:  c.Last.String(),
			Taken: taken,
			Phone: seo.Venue.Phone,
			Tel:   "tel:" + seo.Venue.PhoneE164,
		},
		Weekends:  weekendList(c),
		ListLabel: "Helgene det neste året",
	})
}

func bookingMonth(c booking.Calendar, first site.Date) ui.BookingMonth {
	m := ui.BookingMonth{Name: booking.MonthRange(first, first)}
	week := [7]ui.BookingDay{}
	col := (int(first.Weekday()) + 6) % 7

	for d := first; d.Month == first.Month; d = d.AddDays(1) {
		week[col] = ui.BookingDay{Day: d.Day, Taken: c.Taken[d], Past: d.Before(c.Today), Today: d == c.Today}
		col++

		if col == 7 {
			m.Weeks = append(m.Weeks, week)
			week, col = [7]ui.BookingDay{}, 0
		}
	}

	if col > 0 {
		m.Weeks = append(m.Weeks, week)
	}

	return m
}

// weekendList is the coming year's weekends as text: for visitors without
// script, and for search engines and AI assistants.
func weekendList(c booking.Calendar) []ui.BookingWeekendMonth {
	months := c.WeekendsAhead(booking.MinMonths)
	out := make([]ui.BookingWeekendMonth, len(months))

	for i, m := range months {
		out[i] = ui.BookingWeekendMonth{Name: m.Name(), Note: "Alle helgene er ledige."}
		for _, w := range m.Weekends {
			out[i].Weekends = append(out[i].Weekends, ui.BookingWeekend{Dates: w.Dates(), Status: w.Status(), Taken: w.Status() == "Opptatt"})
		}
	}

	return out
}
