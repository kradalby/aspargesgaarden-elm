package booking

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kradalby/asparges/site"
)

// MinMonths is how far ahead the calendar always goes, this month included.
const MinMonths = 12

// Calendar is the booking calendar as the site shows it: today's month to
// Last. The zero Calendar is "no calendar".
type Calendar struct {
	// Unavailable is a calendar the site has but may not show: never read
	// whole, or not recently enough (Feed.Shown).
	Unavailable bool

	Taken Days
	Today site.Date
	// Last is the end of the month of the last booking, or of MinMonths
	// from today if that is later. The feed lists bookings, not free days:
	// past the last booking nothing says the farm is taking any, so the
	// calendar does not call those days free.
	Last site.Date
}

// NewCalendar is the calendar of taken on today; nil taken is no calendar.
func NewCalendar(taken Days, today site.Date) Calendar {
	if taken == nil {
		return Calendar{}
	}

	last := today.MonthStart(MinMonths).AddDays(-1)
	for d := range taken {
		if end := d.MonthStart(1).AddDays(-1); end.After(last) {
			last = end
		}
	}

	return Calendar{Taken: taken, Today: today, Last: last}
}

// On reports whether there is a calendar to show.
func (c Calendar) On() bool { return c.Taken != nil && !c.Unavailable }

// First is the first day the calendar covers: the 1st of today's month.
func (c Calendar) First() site.Date { return c.Today.MonthStart(0) }

// Upcoming are the taken days from First to Last, sorted.
func (c Calendar) Upcoming() []site.Date {
	out := slices.SortedFunc(maps.Keys(c.Taken), site.Date.Compare)
	return slices.DeleteFunc(out, func(d site.Date) bool { return d.Before(c.First()) || d.After(c.Last) })
}

// Weekend is Friday to Sunday.
type Weekend struct {
	Friday site.Date
	// Free is Friday, Saturday and Sunday: not taken.
	Free [3]bool
}

// AllFree reports whether no day of the weekend is taken.
func (w Weekend) AllFree() bool { return w.Free == [3]bool{true, true, true} }

// Dates is "13.–15. november", or "30. april – 2. mai" across a month.
func (w Weekend) Dates() string {
	sun := w.Friday.AddDays(2)
	if w.Friday.Month != sun.Month {
		return site.DayMonth(w.Friday) + " – " + site.DayMonth(sun)
	}

	return strconv.Itoa(w.Friday.Day) + ".–" + site.DayMonth(sun)
}

// Status is "Ledig", "Opptatt", or the free days: "Ledig fredag og søndag".
func (w Weekend) Status() string {
	var free []string

	for i, f := range w.Free {
		if f {
			free = append(free, [3]string{"fredag", "lørdag", "søndag"}[i])
		}
	}

	switch len(free) {
	case 0:
		return "Opptatt"
	case 3:
		return "Ledig"
	}

	return "Ledig " + strings.Join(free, " og ")
}

// WeekendMonths is a month's weekends, or a run of months From to To whose
// weekends are all free (Weekends then empty).
type WeekendMonths struct {
	From, To site.Date
	Weekends []Weekend
}

// Folded reports whether m is a run of free months.
func (m WeekendMonths) Folded() bool { return len(m.Weekends) == 0 }

// Name is "November 2026", or "Januar – mars 2027" for a folded run.
func (m WeekendMonths) Name() string { return MonthRange(m.From, m.To) }

// WeekendsAhead are the weekends of the next months months, from the one
// under way, by the month of their Saturday. Two or more months in a row
// with every weekend free are folded into one.
func (c Calendar) WeekendsAhead(months int) []WeekendMonths {
	fri := c.Today
	for fri.Weekday() != time.Friday {
		fri = fri.AddDays(-1)
	}

	if fri.AddDays(2).Before(c.Today) {
		fri = fri.AddDays(7)
	}

	byMonth := make([]WeekendMonths, months)
	for i := range byMonth {
		byMonth[i].From = c.First().MonthStart(i)
		byMonth[i].To = byMonth[i].From
	}

	for ; ; fri = fri.AddDays(7) {
		i := c.First().MonthsUntil(fri.AddDays(1))
		if i >= months {
			break
		}

		// A weekend under way whose Saturday was last month.
		if i < 0 {
			continue
		}

		w := Weekend{Friday: fri}
		for j := range w.Free {
			w.Free[j] = !c.Taken[fri.AddDays(j)]
		}

		byMonth[i].Weekends = append(byMonth[i].Weekends, w)
	}

	free := func(m WeekendMonths) bool {
		return !slices.ContainsFunc(m.Weekends, func(w Weekend) bool { return !w.AllFree() })
	}

	var out []WeekendMonths

	for i := 0; i < len(byMonth); {
		j := i
		for j < len(byMonth) && free(byMonth[j]) {
			j++
		}

		if j-i >= 2 {
			out = append(out, WeekendMonths{From: byMonth[i].From, To: byMonth[j-1].From})
			i = j

			continue
		}

		out = append(out, byMonth[i])
		i++
	}

	return out
}

// MonthRange names the months from's to to's: "November 2026", "April –
// juni 2027", "November 2026 – januar 2027".
func MonthRange(from, to site.Date) string {
	name := func(d site.Date) string { return site.MonthName(d.Month) }
	first := strings.ToUpper(name(from)[:1]) + name(from)[1:]

	switch {
	case from.Year == to.Year && from.Month == to.Month:
		return fmt.Sprintf("%s %d", first, from.Year)
	case from.Year == to.Year:
		return fmt.Sprintf("%s – %s %d", first, name(to), to.Year)
	}

	return fmt.Sprintf("%s %d – %s %d", first, from.Year, name(to), to.Year)
}
