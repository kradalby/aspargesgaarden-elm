package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// BookingWeekend is one weekend, Friday to Sunday.
type BookingWeekend struct {
	// Dates is "13.–15. november".
	Dates string
	// Status is "Ledig", "Opptatt", or the free days of a weekend partly
	// taken, "Ledig søndag".
	Status string
	// Taken is the whole weekend taken.
	Taken bool
}

// BookingWeekendMonth is a month's weekends, or a run of months whose
// weekends are all free: no Weekends, and Note says so.
type BookingWeekendMonth struct {
	Name     string
	Weekends []BookingWeekend
	Note     string
}

// BookingWeekends is a list of weekends by month, each free or taken in
// words: the booking calendar as text.
func BookingWeekends(months ...BookingWeekendMonth) elem.Node {
	kids := make([]elem.Node, len(months))
	for i, m := range months {
		kids[i] = weekendMonth(m)
	}

	return elem.Div(nil,
		elem.P(attrs.Props{attrs.Class: "pb-6 text-sort"}, elem.Text("Helgene er fredag til søndag. Spør oss om hverdager.")),
		elem.Div(attrs.Props{attrs.Class: "grid gap-x-10 gap-y-8 md:grid-cols-2 lg:grid-cols-3"}, kids...))
}

func weekendMonth(m BookingWeekendMonth) elem.Node {
	heading := elem.H3(attrs.Props{attrs.Class: "pb-1 font-display text-2xl text-tre"}, elem.Text(m.Name))

	if len(m.Weekends) == 0 {
		return elem.Div(nil, heading,
			elem.P(attrs.Props{attrs.Class: "py-2 border-t border-brown-border text-sort"}, elem.Text(m.Note)))
	}

	items := make([]elem.Node, len(m.Weekends))
	for i, w := range m.Weekends {
		status := "text-mose"
		if w.Taken {
			status = "px-2 bg-rust text-white"
		}

		items[i] = elem.Li(attrs.Props{attrs.Class: "flex items-baseline justify-between gap-4 py-2 border-t border-brown-border"},
			elem.Span(attrs.Props{attrs.Class: "text-sort tabular-nums"}, elem.Text(w.Dates)),
			elem.Span(attrs.Props{attrs.Class: "py-0.5 text-xs font-bold uppercase tracking-wide whitespace-nowrap " + status}, elem.Text(w.Status)),
		)
	}

	return elem.Div(nil, heading, elem.Ul(nil, items...))
}
