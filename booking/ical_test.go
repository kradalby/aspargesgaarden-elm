package booking

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/site"
)

// feed wraps VEVENT bodies in a calendar in Oslo, with CRLF as Google
// sends it.
func feed(events ...string) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-TIMEZONE:Europe/Oslo\r\n")

	for _, e := range events {
		b.WriteString("BEGIN:VEVENT\r\n" + strings.ReplaceAll(strings.TrimSpace(e), "\n", "\r\n") + "\r\nEND:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")

	return b.String()
}

func day(s string) site.Date {
	d, err := site.ParseDate(s)
	if err != nil {
		panic(err)
	}

	return d
}

// The tests' window, 2026–2028.
var lo, hi = midnight(day("2026-01-01"), 0), midnight(day("2028-12-31"), 1)

func parseAll(ics string) ([]span, int, error) {
	return parse(strings.NewReader(ics), lo, hi)
}

// takenIn parses ics and returns the taken days of 2026–2028, sorted.
func takenIn(t *testing.T, ics string) []string {
	t.Helper()

	spans, _, err := parseAll(ics)
	require.NoError(t, err)

	days := taken(spans, day("2026-01-01"), day("2028-12-31"))

	out := make([]string, 0, len(days))
	for d := range days {
		out = append(out, d.String())
	}

	if len(out) == 0 {
		return nil
	}

	slices.Sort(out)

	return out
}

func TestGoogleFeed(t *testing.T) {
	f, err := os.Open("testdata/google.ics")
	require.NoError(t, err)

	defer f.Close()

	spans, bookings, err := parse(f, lo, hi)
	require.NoError(t, err)
	require.Len(t, spans, 82)
	require.Equal(t, 82, bookings)

	days := taken(spans, day("2026-01-01"), day("2028-12-31"))

	// A whole weekend, Friday 10:00 to Sunday 17:00.
	for _, d := range []string{"2026-09-11", "2026-09-12", "2026-09-13"} {
		assert.True(t, days[day(d)], d)
	}

	// Monday 16:00–21:00 is five hours: a meeting, not a booking.
	assert.False(t, days[day("2026-09-21")])
	// Four one-hour slots on a Thursday.
	assert.False(t, days[day("2026-09-17")])
}

func TestTakenRule(t *testing.T) {
	tests := []struct {
		name   string
		events []string
		want   []string
	}{
		{
			name:   "weekend",
			events: []string{"DTSTART;TZID=Europe/Oslo:20270611T100000\nDTEND;TZID=Europe/Oslo:20270613T170000"},
			want:   []string{"2027-06-11", "2027-06-12", "2027-06-13"},
		},
		{
			name:   "party past midnight takes the evening",
			events: []string{"DTSTART:20270611T180000\nDTEND:20270612T020000"},
			want:   []string{"2027-06-11"},
		},
		{
			name:   "long party with no six hours on either day takes its first",
			events: []string{"DTSTART:20270611T200000\nDTEND:20270612T030000"},
			want:   []string{"2027-06-11"},
		},
		{
			name:   "long night mostly after midnight",
			events: []string{"DTSTART:20270611T230000\nDTEND:20270612T070000"},
			want:   []string{"2027-06-12"},
		},
		{
			name:   "one-hour meeting",
			events: []string{"DTSTART:20270614T100000\nDTEND:20270614T110000"},
		},
		{
			name: "two short bookings that add up",
			events: []string{
				"DTSTART:20270614T090000\nDTEND:20270614T120000",
				"DTSTART:20270614T150000\nDTEND:20270614T180000",
			},
			want: []string{"2027-06-14"},
		},
		{
			name: "the same hours twice count once",
			events: []string{
				"DTSTART:20270614T090000\nDTEND:20270614T140000",
				"DTSTART:20270614T090000\nDTEND:20270614T140000",
			},
		},
		{
			name:   "all-day event",
			events: []string{"DTSTART;VALUE=DATE:20270614\nDTEND;VALUE=DATE:20270616"},
			want:   []string{"2027-06-14", "2027-06-15"},
		},
		{
			name:   "all-day event without an end is one day",
			events: []string{"DTSTART;VALUE=DATE:20270614"},
			want:   []string{"2027-06-14"},
		},
		{
			name:   "all-day event over the spring DST change",
			events: []string{"DTSTART;VALUE=DATE:20270327\nDURATION:P2D"},
			want:   []string{"2027-03-27", "2027-03-28"},
		},
		{
			// Midnight to 06:00 on the spring-forward day is five hours.
			name:   "short DST night",
			events: []string{"DTSTART;TZID=Europe/Oslo:20270328T000000\nDTEND;TZID=Europe/Oslo:20270328T060000"},
		},
		{
			// Midnight to 05:00 on the fall-back day is six hours.
			name:   "long DST night",
			events: []string{"DTSTART;TZID=Europe/Oslo:20271031T000000\nDTEND;TZID=Europe/Oslo:20271031T050000"},
			want:   []string{"2027-10-31"},
		},
		{
			name:   "UTC times are read in Oslo",
			events: []string{"DTSTART:20270614T220000Z\nDTEND:20270615T060000Z"},
			want:   []string{"2027-06-15"},
		},
		{
			name:   "duration",
			events: []string{"DTSTART:20270614T100000\nDURATION:PT6H"},
			want:   []string{"2027-06-14"},
		},
		{
			name:   "cancelled",
			events: []string{"DTSTART:20270614T100000\nDTEND:20270614T200000\nSTATUS:CANCELLED"},
		},
		{
			name:   "an alarm's properties are not the event's",
			events: []string{"DTSTART:20270614T100000\nDTEND:20270614T200000\nBEGIN:VALARM\nDTSTART:20270101T000000\nEND:VALARM"},
			want:   []string{"2027-06-14"},
		},
		{
			name:   "outside the window",
			events: []string{"DTSTART;VALUE=DATE:00010101\nDTEND;VALUE=DATE:99991231"},
			want:   nil, // checked by count below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := takenIn(t, feed(tt.events...))
			if tt.name == "outside the window" {
				assert.Len(t, got, 3*365+1)
				return
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFoldedLines(t *testing.T) {
	ics := "BEGIN:VCALENDAR\r\nX-WR-TIMEZONE:Europe/Oslo\r\nBEGIN:VEVENT\r\nDTSTART;TZID=\"Europe/\r\n Oslo\":2027061\r\n\t4T100000\r\nDTEND:20270614T\r\n 200000\r\nEND:VEVENT\r\nEND:VCALENDAR"
	assert.Equal(t, []string{"2027-06-14"}, takenIn(t, ics))
}

// Every event dated without a guess, or the read fails and says which.
func TestUncertainEventsFailTheRead(t *testing.T) {
	for name, ev := range map[string]string{
		"unknown zone":          "UID:z\nDTSTART;TZID=W. Europe Standard Time:20270614T100000\nDTEND;TZID=W. Europe Standard Time:20270614T160000",
		"not a date":            "UID:z\nDTSTART:not a date\nDTEND:20270614T200000",
		"no such month":         "UID:z\nDTSTART:20271399T100000\nDTEND:20270614T200000",
		"no start":              "UID:z\nDTEND:20270614T200000",
		"ends before it starts": "UID:z\nDTSTART:20270614T200000\nDTEND:20270614T100000",
		"bad duration":          "UID:z\nDTSTART:20270614T100000\nDURATION:P1Y",
		"negative duration":     "UID:z\nDTSTART:20270614T100000\nDURATION:-PT6H",
		"day and time mixed":    "UID:z\nDTSTART;VALUE=DATE:20270614\nDTEND:20270615T100000",
		"markup":                "UID:z\n<script>alert(1)</script>",
		"orphan RECURRENCE-ID":  "UID:z\nRECURRENCE-ID:20270614T100000Z\nDTSTART:20270615T100000Z\nDTEND:20270615T200000Z",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := parseAll(feed("UID:ok\nDTSTART:20270615T100000\nDTEND:20270615T200000", ev))
			require.ErrorIs(t, err, errUnreadable)
			assert.Contains(t, err.Error(), "event UID z")
		})
	}

	// A time without a zone needs the calendar's.
	_, _, err := parseAll(strings.Replace(feed("DTSTART:20270615T100000\nDTEND:20270615T200000"), "X-WR-TIMEZONE:Europe/Oslo\r\n", "", 1))
	require.ErrorIs(t, err, errUnreadable)
	assert.Contains(t, err.Error(), "event 1")
}

func TestJunk(t *testing.T) {
	refused := map[string]string{
		"empty":     "",
		"html":      "<!DOCTYPE html>\n<html><head><meta charset=\"utf-8\"></head><body>Not found</body></html>",
		"cut off":   strings.TrimSuffix(feed("DTSTART:20270614T100000\nDTEND:20270614T200000"), "END:VCALENDAR\r\n"),
		"stray end": "BEGIN:VCALENDAR\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n",
		"huge line": "BEGIN:VCALENDAR\r\nX-JUNK:" + strings.Repeat("x", maxLine+1) + "\r\nEND:VCALENDAR\r\n",
		"huge fold": "BEGIN:VCALENDAR\r\nX-JUNK:x" + strings.Repeat("\r\n "+strings.Repeat("x", 1000), maxLine/1000+1) + "\r\nEND:VCALENDAR\r\n",
	}

	for name, body := range refused {
		t.Run(name, func(t *testing.T) {
			_, _, err := parseAll(body)
			require.Error(t, err)
		})
	}

	// Bookings are counted; cancelled events are none.
	spans, bookings, err := parseAll(feed(
		"DTSTART:20270615T100000\nDTEND:20270615T200000",
		"DTSTART:20270616T100000\nDTEND:20270616T200000\nSTATUS:CANCELLED",
	))
	require.NoError(t, err)
	assert.Equal(t, 1, bookings)
	require.Len(t, spans, 1)
	assert.Equal(t, time.Date(2027, 6, 15, 10, 0, 0, 0, site.Oslo), spans[0].start)
}

func TestDuration(t *testing.T) {
	for in, want := range map[string]time.Duration{
		"PT6H":      6 * time.Hour,
		"P1D":       24 * time.Hour,
		"P2W":       14 * 24 * time.Hour,
		"P1DT2H30M": 26*time.Hour + 30*time.Minute,
		"+PT90S":    90 * time.Second,
	} {
		got, err := duration(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	for _, in := range []string{"", "P", "PT", "P1M", "PT6", "-PT1H", "P1X", "PT99999999H"} {
		_, err := duration(in)
		assert.Error(t, err, in)
	}
}
