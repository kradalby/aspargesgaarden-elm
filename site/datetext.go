package site

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

var norwegianMonths = [...]string{
	"januar", "februar", "mars", "april", "mai", "juni",
	"juli", "august", "september", "oktober", "november", "desember",
}

// NorwegianDate is d as the site writes it: "1. desember 2025".
func NorwegianDate(d Date) string {
	return dayMonth(d) + " " + strconv.Itoa(d.Year)
}

func dayMonth(d Date) string {
	return fmt.Sprintf("%d. %s", d.Day, norwegianMonths[d.Month-1])
}

// Days names the event's first day, or its run of days when it spans
// several: "14.–15. november 2026", "30. november – 1. desember 2026". The
// extra days of a course are separate events and left out.
func (e Event) Days() string {
	switch {
	case e.End.IsZero() || e.End == e.Start:
		return NorwegianDate(e.Start)
	case e.Start.Year != e.End.Year:
		return NorwegianDate(e.Start) + " – " + NorwegianDate(e.End)
	case e.Start.Month != e.End.Month:
		return dayMonth(e.Start) + " – " + NorwegianDate(e.End)
	}

	return strconv.Itoa(e.Start.Day) + ".–" + NorwegianDate(e.End)
}

// DateLabel is the bold word in front of DateLine.
func (e Event) DateLabel() string {
	if len(e.ExtraDates) > 0 {
		return "Datoer:"
	}

	return "Dato:"
}

// DateLine is the event's when-and-where sentence:
//
//  19. mai 2026 kl. 19.30–22.00 på Aspargesgården
//  14. november kl. 12.00 – 15. november 2026 kl. 16.00 på Aspargesgården
//  31. oktober, 28. november og 29. november 2026 kl. 10.00–15.00 på Aspargesgården
//
// A year is written once, after the last day in it. The dash is tight
// between hours and spaced between days, which hold spaces themselves.
// An end time with no end day reads as the same day.
func (e Event) DateLine() string {
	if e.Start.IsZero() {
		return ""
	}

	var line string

	switch {
	case len(e.ExtraDates) > 0:
		line = dayList(slices.SortedFunc(slices.Values(e.days()), Date.Compare)) + hours(e.StartTime, e.EndTime)
	case e.End.IsZero() || e.End == e.Start:
		line = NorwegianDate(e.Start) + hours(e.StartTime, e.EndTime)
	default:
		first := NorwegianDate(e.Start)
		if e.Start.Year == e.End.Year {
			first = dayMonth(e.Start)
		}

		line = first + hours(e.StartTime, "") + " – " + NorwegianDate(e.End) + hours(e.EndTime, "")
	}

	return line + " på Aspargesgården"
}

// dayList is sorted days as "28. desember 2026, 4. januar og 11. januar 2027".
func dayList(days []Date) string {
	var b strings.Builder

	for i, d := range days {
		switch {
		case i == 0:
		case i == len(days)-1:
			b.WriteString(" og ")
		default:
			b.WriteString(", ")
		}

		b.WriteString(dayMonth(d))

		if i == len(days)-1 || days[i+1].Year != d.Year {
			b.WriteString(" " + strconv.Itoa(d.Year))
		}
	}

	return b.String()
}

// hours is " kl. 12.00–15.00", " kl. 12.00", " til kl. 15.00" or "" for
// HH:MM clocks.
func hours(from, to string) string {
	dot := func(clock string) string { return strings.Replace(clock, ":", ".", 1) }

	switch {
	case from != "" && to != "":
		return " kl. " + dot(from) + "–" + dot(to)
	case from != "":
		return " kl. " + dot(from)
	case to != "":
		return " til kl. " + dot(to)
	}

	return ""
}
