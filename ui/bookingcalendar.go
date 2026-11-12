package ui

import (
	"encoding/json"
	"strconv"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// BookingDay is a day as the booking calendar draws it.
type BookingDay struct {
	// Day is the day of the month; 0 is a blank before the 1st or after
	// the last.
	Day                int
	Taken, Past, Today bool
}

// BookingMonth is one month of days, Monday first.
type BookingMonth struct {
	// Name is the caption, "November 2026".
	Name  string
	Weeks [][7]BookingDay
}

// BookingData is what the calendar app (elm/calendar) is booted with;
// Calendar.elm's flags decoder reads it. Dates are YYYY-MM-DD.
type BookingData struct {
	// Today is the day in Oslo the page was rendered.
	Today string `json:"today"`
	// Last is the last day the calendar covers.
	Last string `json:"last"`
	// Taken are the taken days from the 1st of Today's month to Last.
	Taken []string `json:"taken"`
	// Phone and Tel are the number to call and its tel: link.
	Phone string `json:"phone"`
	Tel   string `json:"tel"`
}

// BookingCalendarProps configures BookingCalendar.
type BookingCalendarProps struct {
	Phone, Tel string
	// Range names the months shown, "November 2026 – januar 2027".
	Range  string
	Months []BookingMonth
	// Edge says how far the calendar goes.
	Edge string
	Data BookingData
	// Weekends is the coming year's weekends as text, under ListLabel.
	Weekends  []BookingWeekendMonth
	ListLabel string
}

// calendarAttr marks the part of the calendar the app takes over. Page
// loads the app where a page holds one, and its boot.js finds it by this.
const calendarAttr = "data-calendar"

// Calendar.elm draws the same markup from these; a change here is a change
// there, or the page jumps when the app boots.
const (
	calendarRangeClass = "flex items-center justify-between gap-2 h-12 mb-2"
	calendarGridClass  = "grid items-start gap-x-10 gap-y-8 sm:grid-cols-2 lg:grid-cols-3"
	// The question's box: its rows are the label's line and the date
	// field's height, fixed, so the app's form fills exactly the box this
	// page leaves for it in every engine, whatever height their date
	// fields would take on their own.
	calendarAskClass  = "mb-8 p-6 bg-brown/20"
	calendarRowsClass = "grid grid-rows-[1.5rem_3rem] gap-1"
)

// BookingCalendar is the contact page's booking calendar: three months as
// tables and the coming year's weekends as a list, complete without
// script. The calendar app (elm/calendar) takes the months over and adds
// paging and a date question.
func BookingCalendar(p BookingCalendarProps) elem.Node {
	tables := make([]elem.Node, len(p.Months))
	for i, m := range p.Months {
		tables[i] = BookingMonthTable(m)
	}

	// Plain strings cannot fail to marshal.
	data, err := json.Marshal(p.Data)
	if err != nil {
		panic("ui: calendar data: " + err.Error())
	}

	return elem.Section(attrs.Props{attrs.ID: "kalender", "aria-labelledby": "kalender-tittel", attrs.Class: "px-2 pt-12 pb-16 md:px-0"},
		elem.H2(attrs.Props{attrs.ID: "kalender-tittel", attrs.Class: "pb-2 font-display text-4xl text-tre"}, elem.Text("Ledige datoer")),
		elem.P(attrs.Props{attrs.Class: "pb-6 max-w-2xl text-lg text-sort"},
			elem.Text("Kalenderen viser dagene gården er leid ut.")),
		elem.Div(attrs.Props{calendarAttr: string(data)},
			// The app puts its date question here; without it, the box
			// says how to ask.
			elem.Div(attrs.Props{attrs.Class: calendarAskClass},
				elem.Div(attrs.Props{attrs.Class: calendarRowsClass},
					elem.P(attrs.Props{attrs.Class: "row-start-2 self-center text-sm text-sort"},
						elem.Text("Vil du spørre om en dato? Ring "),
						elem.A(attrs.Props{attrs.Href: p.Tel, attrs.Class: "underline whitespace-nowrap"}, elem.Text(p.Phone)),
						elem.Text(".")))),
			bookingLegend(),
			elem.Div(attrs.Props{attrs.Class: calendarRangeClass},
				elem.P(attrs.Props{attrs.Class: "font-bold text-sort", "aria-live": "polite"}, elem.Text(p.Range))),
			elem.Div(attrs.Props{attrs.Class: calendarGridClass}, tables...),
			elem.P(attrs.Props{attrs.Class: "pt-6 text-sort"}, elem.Text(p.Edge)),
		),
		elem.Details(attrs.Props{attrs.Class: "mt-10 group"},
			elem.Summary(attrs.Props{attrs.Class: "inline-block cursor-pointer py-3 px-6 border border-brown-border text-sm font-bold uppercase tracking-wide text-sort hover:bg-brown/20 group-open:mb-8"},
				elem.Text(p.ListLabel)),
			BookingWeekends(p.Weekends...),
		),
	)
}

var weekdays = [7][2]string{
	{"ma", "mandag"},
	{"ti", "tirsdag"},
	{"on", "onsdag"},
	{"to", "torsdag"},
	{"fr", "fredag"},
	{"lø", "lørdag"},
	{"sø", "søndag"},
}

// BookingMonthTable is one month as a table: a caption, the weekdays, and a
// row per week.
func BookingMonthTable(m BookingMonth) elem.Node {
	heads := make([]elem.Node, 7)
	for i, w := range weekdays {
		heads[i] = elem.Th(attrs.Props{attrs.Scope: "col", "abbr": w[1], attrs.Class: "pb-1 text-xs font-bold uppercase text-gray-600"}, elem.Text(w[0]))
	}

	rows := make([]elem.Node, len(m.Weeks))
	for i, w := range m.Weeks {
		cells := make([]elem.Node, 7)
		for j, d := range w {
			cells[j] = bookingCell(d)
		}

		rows[i] = elem.Tr(nil, cells...)
	}

	return elem.Table(attrs.Props{attrs.Class: "w-full table-fixed border-separate border-spacing-1"},
		elem.Caption(attrs.Props{attrs.Class: "pb-1 text-left font-display text-2xl text-tre"}, elem.Text(m.Name)),
		elem.THead(nil, elem.Tr(nil, heads...)),
		elem.TBody(nil, rows...),
	)
}

// bookingCell is a day: taken days filled, today framed, past days quiet.
// The state is in words for screen readers, and in fill, not hue, for eyes.
func bookingCell(d BookingDay) elem.Node {
	if d.Day == 0 {
		return elem.Td(nil)
	}

	taken := d.Taken && !d.Past
	a := attrs.Props{attrs.Class: cls(
		"h-9 text-center text-sm tabular-nums",
		onClass(taken, "bg-rust font-bold text-white"),
		onClass(d.Past, "text-gray-500"),
		onClass(!taken && !d.Past, "text-sort"),
		onClass(d.Today, "font-bold outline-2 -outline-offset-2 outline-sort"),
	)}

	kids := []elem.Node{elem.Text(strconv.Itoa(d.Day))}
	if taken {
		kids = append(kids, srOnly(" opptatt"))
	}

	if d.Today {
		a["aria-current"] = "date"
		kids = append(kids, srOnly(" i dag"))
	}

	return elem.Td(a, kids...)
}

func srOnly(text string) elem.Node {
	return elem.Span(attrs.Props{attrs.Class: "sr-only"}, elem.Text(text))
}

// bookingLegend explains the marks; the swatches are the cells' own looks.
func bookingLegend() elem.Node {
	item := func(swatch, text string) elem.Node {
		return elem.Li(attrs.Props{attrs.Class: "flex items-center gap-2"},
			elem.Span(attrs.Props{attrs.Class: "inline-block size-5 " + swatch, attrs.AriaHidden: "true"}),
			elem.Text(text))
	}

	return elem.Ul(attrs.Props{attrs.Class: "flex flex-wrap gap-x-6 gap-y-2 pb-4 text-sm text-sort"},
		item("bg-rust", "Opptatt"),
		item("border border-brown-border", "Ledig"),
		item("outline-2 -outline-offset-2 outline-sort", "I dag"),
	)
}

// holdsCalendar reports whether a BookingCalendar is among nodes.
func holdsCalendar(nodes []elem.Node) bool {
	return holds(nodes, func(e *elem.Element) bool {
		_, ok := e.Attrs[calendarAttr]
		return ok
	})
}
