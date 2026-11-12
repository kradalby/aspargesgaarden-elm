package booking

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/site"
)

func days(ds ...string) Days {
	out := Days{}
	for _, d := range ds {
		out[day(d)] = true
	}

	return out
}

func TestCalendarRange(t *testing.T) {
	assert.False(t, NewCalendar(nil, day("2026-11-11")).On())

	// Nothing booked: a year, this month included.
	c := NewCalendar(Days{}, day("2026-11-11"))
	require.True(t, c.On())
	assert.Equal(t, day("2026-11-01"), c.First())
	assert.Equal(t, day("2027-10-31"), c.Last)

	// To the end of the month of the last booking.
	c = NewCalendar(days("2026-09-12", "2026-11-14", "2028-09-02"), day("2026-11-11"))
	assert.Equal(t, day("2028-09-30"), c.Last)
	assert.Equal(t, []string{"2026-11-14", "2028-09-02"}, isos(c.Upcoming()))
}

func isos(ds []site.Date) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.String()
	}

	return out
}

func TestWeekendsAhead(t *testing.T) {
	c := NewCalendar(days("2026-11-13", "2026-11-14", "2026-11-15", "2026-11-28", "2026-11-29", "2027-04-30"), day("2026-11-14"))
	got := c.WeekendsAhead(MinMonths)

	// The weekend under way comes first; a month whose weekends are all
	// free on either side of a run folds with it.
	require.Len(t, got, 4)
	assert.Equal(t, "November 2026", got[0].Name())
	assert.Equal(t, []string{"13.–15. november", "20.–22. november", "27.–29. november"},
		[]string{got[0].Weekends[0].Dates(), got[0].Weekends[1].Dates(), got[0].Weekends[2].Dates()})
	assert.Equal(t, []string{"Opptatt", "Ledig", "Ledig fredag"},
		[]string{got[0].Weekends[0].Status(), got[0].Weekends[1].Status(), got[0].Weekends[2].Status()})

	assert.True(t, got[1].Folded())
	assert.Equal(t, "Desember 2026 – april 2027", got[1].Name())

	// 30 April – 2 May belongs to May, the month of its Saturday.
	assert.Equal(t, "Mai 2027", got[2].Name())
	assert.Equal(t, "Ledig lørdag og søndag", got[2].Weekends[0].Status())
	assert.Equal(t, "Juni – oktober 2027", got[3].Name())
	assert.True(t, got[3].Folded())
}

func TestWeekendsAheadOnTheFirst(t *testing.T) {
	// Sunday 1 August 2027: the weekend under way has its Saturday in July.
	got := NewCalendar(Days{}, day("2027-08-01")).WeekendsAhead(2)
	require.Len(t, got, 1)
	assert.Equal(t, "August – september 2027", got[0].Name())
}

func TestWeekendDates(t *testing.T) {
	assert.Equal(t, "30. april – 2. mai", Weekend{Friday: day("2027-04-30")}.Dates())
	assert.Equal(t, "Ledig fredag og søndag", Weekend{Free: [3]bool{true, false, true}}.Status())
}

func TestMonthRange(t *testing.T) {
	assert.Equal(t, "November 2026", MonthRange(day("2026-11-01"), day("2026-11-01")))
	assert.Equal(t, "April – juni 2027", MonthRange(day("2027-04-01"), day("2027-06-01")))
	assert.Equal(t, "November 2026 – januar 2027", MonthRange(day("2026-11-01"), day("2027-01-01")))
}
