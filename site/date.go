package site

import (
	"cmp"
	"fmt"
	"time"
	_ "time/tzdata" // the service must not depend on the host's zoneinfo
)

// Oslo is the venue's time zone. Every "today" on the site is a day in Oslo,
// whatever zone the server runs in.
var Oslo = mustLoad("Europe/Oslo")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("loading %s from embedded tzdata: %v", name, err))
	}

	return loc
}

const dateLayout = "2006-01-02"

// Date is a calendar day with no time zone; events happen on days in Oslo.
// The zero Date means "no date".
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// ParseDate parses a strict YYYY-MM-DD date.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("%w: %q", ErrInvalidDate, s)
	}

	return DateOf(t), nil
}

// DateOf is the calendar day of t in t's own location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// Today is the day it is in Oslo at now.
func Today(now time.Time) Date {
	return DateOf(now.In(Oslo))
}

// IsZero reports whether d is "no date".
func (d Date) IsZero() bool { return d == Date{} }

// String is YYYY-MM-DD, or "" for the zero Date.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}

	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// Compare returns -1, 0 or +1 as d is before, equal to or after o.
func (d Date) Compare(o Date) int {
	return cmp.Or(
		cmp.Compare(d.Year, o.Year),
		cmp.Compare(d.Month, o.Month),
		cmp.Compare(d.Day, o.Day),
	)
}

// Before reports whether d is a day before o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// After reports whether d is a day after o.
func (d Date) After(o Date) bool { return d.Compare(o) > 0 }

// civil is d at noon UTC, for arithmetic no DST change can shift a day.
func (d Date) civil() time.Time { return time.Date(d.Year, d.Month, d.Day, 12, 0, 0, 0, time.UTC) }

// AddDays is the day n days after d (before, for a negative n).
func (d Date) AddDays(n int) Date { return DateOf(d.civil().AddDate(0, 0, n)) }

// Weekday is d's day of the week.
func (d Date) Weekday() time.Weekday { return d.civil().Weekday() }

// MonthStart is the first day of the month n months after d's.
func (d Date) MonthStart(n int) Date {
	return DateOf(time.Date(d.Year, d.Month+time.Month(n), 1, 12, 0, 0, 0, time.UTC))
}

// MonthsUntil is how many months o's month is after d's.
func (d Date) MonthsUntil(o Date) int { return (o.Year-d.Year)*12 + int(o.Month-d.Month) }
