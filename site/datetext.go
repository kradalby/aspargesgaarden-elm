package site

import (
	"fmt"
	"slices"
	"strings"
)

var norwegianMonths = [...]string{
	"januar", "februar", "mars", "april", "mai", "juni",
	"juli", "august", "september", "oktober", "november", "desember",
}

// NorwegianDate is d as the site writes it: "01. desember 2025". The
// zero-padded day is the old site's spelling, kept so pages read the same.
func NorwegianDate(d Date) string {
	return fmt.Sprintf("%02d. %s %d", d.Day, norwegianMonths[d.Month-1], d.Year)
}

// DateLabel is the bold word in front of DateLine.
func (e Event) DateLabel() string {
	if len(e.ExtraDates) > 0 {
		return "Datoer:"
	}

	return "Dato:"
}

// DateLine is the event's when-and-where sentence, ported from the old site:
//
//  14. november 2026 kl 12:00 - 15. november 2026 kl 16:00 på Aspargesgården
//  31. oktober 2026, 28. november 2026 kl 10:00 - 15:00 på Aspargesgården
//
// One difference: an end time with no end day reads as the same day, where
// the old site dropped it.
func (e Event) DateLine() string {
	if e.Start.IsZero() {
		return ""
	}

	var b strings.Builder

	if len(e.ExtraDates) > 0 {
		days := slices.SortedFunc(slices.Values(e.days()), Date.Compare)
		for i, d := range days {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(NorwegianDate(d))
		}
		writeClock(&b, " kl ", e.StartTime)
		writeClock(&b, " - ", e.EndTime)
	} else {
		b.WriteString(NorwegianDate(e.Start))
		writeClock(&b, " kl ", e.StartTime)
		if e.End.IsZero() || e.End == e.Start {
			writeClock(&b, " - ", e.EndTime)
		} else {
			b.WriteString(" - " + NorwegianDate(e.End))
			writeClock(&b, " kl ", e.EndTime)
		}
	}

	b.WriteString(" på Aspargesgården")

	return b.String()
}

func writeClock(b *strings.Builder, prefix, clock string) {
	if clock != "" {
		b.WriteString(prefix + clock)
	}
}
