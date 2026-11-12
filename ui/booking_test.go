package ui

import (
	"testing"

	"github.com/chasefleming/elem-go"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

var testMonth = BookingMonth{Name: "November 2026", Weeks: [][7]BookingDay{
	{{}, {}, {}, {}, {}, {}, {Day: 1, Past: true, Taken: true}},
	{{Day: 2, Past: true}, {Day: 3, Today: true}, {Day: 4}, {Day: 5}, {Day: 6, Taken: true}, {Day: 7, Taken: true}, {Day: 8}},
}}

func TestBookingCalendar(t *testing.T) {
	cal := BookingCalendar(BookingCalendarProps{
		Phone: "+47 90 91 98 12", Tel: "tel:+4790919812",
		Range:  "November 2026",
		Months: []BookingMonth{testMonth},
		Edge:   "Kalenderen går til september 2028. Spør oss om datoer etter det.",
		Data: BookingData{
			Today: "2026-11-03", Last: "2028-09-30", Taken: []string{"2026-11-01", "2026-11-06"},
			Phone: "+47 90 91 98 12", Tel: "tel:+4790919812", MaxAge: 1800,
		},
		Weekends:  []BookingWeekendMonth{{Name: "November 2026", Weekends: []BookingWeekend{{Dates: "6.–8. november", Status: "Ledig søndag"}}}},
		ListLabel: "Helgene det neste året",
	})
	golden.HTML(t, "bookingcalendar", cal.Render())

	// The app loads where the calendar is, and only there.
	page := func(body elem.Node) string {
		return Page(PageProps{BaseURL: "https://x.no", Meta: Meta{Title: "x", Path: "/"}}, body).Render()
	}
	require.Contains(t, page(cal), "/static/calendar.js")
	require.NotContains(t, page(elem.P(nil)), "/static/calendar.js")
}

func TestBookingUnavailable(t *testing.T) {
	golden.HTML(t, "bookingunavailable", BookingUnavailable("+47 90 91 98 12", "tel:+4790919812").Render())
}

func TestBookingWeekends(t *testing.T) {
	golden.HTML(t, "bookingweekends", BookingWeekends(
		BookingWeekendMonth{Name: "November 2026", Weekends: []BookingWeekend{
			{Dates: "13.–15. november", Status: "Opptatt", Taken: true},
			{Dates: "20.–22. november", Status: "Ledig søndag"},
		}},
		BookingWeekendMonth{Name: "Januar – mars 2027", Note: "Alle helgene er ledige."},
	).Render())
}
