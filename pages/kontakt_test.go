package pages

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/booking"
	"github.com/kradalby/asparges/golden"
)

// Without the calendar (not configured, or never read) the page has no
// calendar section at all.
func TestKontakt(t *testing.T) {
	got := Kontakt(testSite).Render()
	golden.HTML(t, "kontakt", got)
	require.NotContains(t, got, "kalender")
	require.NotContains(t, got, "<iframe")
}

// A calendar the site may not vouch for is a sentence and the number to
// call: no months, no list, no app.
func TestKontaktCalendarUnavailable(t *testing.T) {
	s := testSite
	s.Calendar = booking.Calendar{Unavailable: true}

	got := Kontakt(s).Render()
	require.Contains(t, got, "Kalenderen er ikke tilgjengelig akkurat nå. Ring")
	require.Contains(t, got, `href="tel:+4790919812"`)
	require.NotContains(t, got, "<table")
	require.NotContains(t, got, "data-calendar")
	require.NotContains(t, got, "calendar.js")
	require.NotContains(t, got, "Helgene det neste året")
}

func TestKontaktCalendar(t *testing.T) {
	s := testSite
	s.Calendar = booking.NewCalendar(booking.Days{
		date("2026-11-13"): true, date("2026-11-14"): true, date("2026-11-15"): true,
		date("2026-12-05"): true, date("2027-06-12"): true,
	}, date("2026-11-11"))

	got := Kontakt(s).Render()
	golden.HTML(t, "kontakt-calendar", got)

	// Three months drawn, the year's weekends as text, the app loaded.
	require.Contains(t, got, "November 2026 – januar 2027")
	require.Equal(t, 3, strings.Count(got, "<table"))
	require.Contains(t, got, "13.–15. november")
	require.Contains(t, got, "/static/calendar.js")
}
