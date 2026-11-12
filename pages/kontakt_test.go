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
