package site

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

var norwegianMonths = [...]string{
	"januar", "februar", "mars", "april", "mai", "juni",
	"juli", "august", "september", "oktober", "november", "desember",
}

// MonthName is m in Norwegian, lowercase: "juni".
func MonthName(m time.Month) string { return norwegianMonths[m-1] }

// DayMonth is d without its year: "12. juni".
func DayMonth(d Date) string { return dayMonth(d) }

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

// DateLine is the event's when-and-where sentence: its days, then the hours
// of each, as in "14.–15. november 2026 kl. 12.00–16.00 på Aspargesgården",
// or for a course "31. oktober, 28. november og 29. november 2026 kl.
// 10.00–15.00 på Aspargesgården". A year is written once, after the last day
// in it.
func (e Event) DateLine() string {
	if e.Start.IsZero() {
		return ""
	}

	days := e.Days()
	if len(e.ExtraDates) > 0 {
		days = dayList(slices.SortedFunc(slices.Values(e.days()), Date.Compare))
	}

	return days + hours(e.StartTime, e.EndTime) + " på Aspargesgården"
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
