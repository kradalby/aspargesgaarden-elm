package booking

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/site"
)

// A Google feed's repeats: a Friday-evening series over the spring DST
// change with a deleted and a moved evening, and an endless monthly
// all-day repeat with one time cancelled.
func TestGoogleRepeats(t *testing.T) {
	body, err := os.ReadFile("testdata/google-repeats.ics")
	require.NoError(t, err)

	got := takenIn(t, string(body))

	// 18:00–24:00 is six hours on Oslo's clock before and after 28 March;
	// 26 March moved to Saturday the 27th, 2 April deleted.
	for _, d := range []string{"2027-03-19", "2027-03-27", "2027-04-09"} {
		assert.Contains(t, got, d)
	}

	for _, d := range []string{"2027-03-26", "2027-04-02", "2027-07-10"} {
		assert.NotContains(t, got, d)
	}

	// The second Saturday of every month from June 2027 to the end of the
	// window, July cancelled.
	var second []string

	for m := day("2027-06-01"); m.Year < 2029; m = m.MonthStart(1) {
		sat := m
		for sat.Weekday() != time.Saturday {
			sat = sat.AddDays(1)
		}

		if d := sat.AddDays(7).String(); d != "2027-07-10" {
			second = append(second, d)
		}
	}

	for _, d := range second {
		assert.Contains(t, got, d)
	}

	assert.Len(t, got, 3+len(second))
}

func TestRepeats(t *testing.T) {
	const weekly = "UID:a@google.com\nDTSTART;TZID=Europe/Oslo:20270108T100000\nDTEND;TZID=Europe/Oslo:20270108T200000\n"

	tests := []struct {
		name   string
		events []string
		want   []string
	}{
		{
			name:   "weekly until",
			events: []string{weekly + "RRULE:FREQ=WEEKLY;UNTIL=20270129T225959Z;BYDAY=FR"},
			want:   []string{"2027-01-08", "2027-01-15", "2027-01-22", "2027-01-29"},
		},
		{
			name:   "weekly count",
			events: []string{weekly + "RRULE:FREQ=WEEKLY;COUNT=2"},
			want:   []string{"2027-01-08", "2027-01-15"},
		},
		{
			name:   "every other week",
			events: []string{weekly + "RRULE:FREQ=WEEKLY;INTERVAL=2;COUNT=3;BYDAY=FR"},
			want:   []string{"2027-01-08", "2027-01-22", "2027-02-05"},
		},
		{
			name:   "monthly on the last Friday",
			events: []string{"UID:b\nDTSTART;TZID=Europe/Oslo:20270129T100000\nDTEND;TZID=Europe/Oslo:20270129T200000\nRRULE:FREQ=MONTHLY;BYDAY=-1FR;COUNT=3"},
			want:   []string{"2027-01-29", "2027-02-26", "2027-03-26"},
		},
		{
			name:   "monthly on a day of the month",
			events: []string{"UID:c\nDTSTART;VALUE=DATE:20270131\nDTEND;VALUE=DATE:20270201\nRRULE:FREQ=MONTHLY;BYMONTHDAY=31;COUNT=3"},
			want:   []string{"2027-01-31", "2027-03-31", "2027-05-31"},
		},
		{
			name:   "yearly",
			events: []string{"UID:d\nDTSTART;VALUE=DATE:20260620\nDTEND;VALUE=DATE:20260621\nRRULE:FREQ=YEARLY"},
			want:   []string{"2026-06-20", "2027-06-20", "2028-06-20"},
		},
		{
			name:   "yearly in given months",
			events: []string{"UID:e\nDTSTART;VALUE=DATE:20270605\nDTEND;VALUE=DATE:20270606\nRRULE:FREQ=YEARLY;BYMONTH=6,8;BYDAY=1SA;UNTIL=20271231"},
			want:   []string{"2027-06-05", "2027-08-07"},
		},
		{
			name:   "daily for a long weekend",
			events: []string{"UID:f\nDTSTART;VALUE=DATE:20270604\nDTEND;VALUE=DATE:20270605\nRRULE:FREQ=DAILY;COUNT=3"},
			want:   []string{"2027-06-04", "2027-06-05", "2027-06-06"},
		},
		{
			name:   "all-day weekly",
			events: []string{"UID:g\nDTSTART;VALUE=DATE:20270605\nDTEND;VALUE=DATE:20270606\nRRULE:FREQ=WEEKLY;BYDAY=SA;COUNT=3"},
			want:   []string{"2027-06-05", "2027-06-12", "2027-06-19"},
		},
		{
			name:   "a deleted time",
			events: []string{weekly + "RRULE:FREQ=WEEKLY;COUNT=3\nEXDATE;TZID=Europe/Oslo:20270115T100000"},
			want:   []string{"2027-01-08", "2027-01-22"},
		},
		{
			name: "a moved time: the old day free, the new one taken",
			events: []string{
				weekly + "RRULE:FREQ=WEEKLY;COUNT=3",
				"UID:a@google.com\nRECURRENCE-ID;TZID=Europe/Oslo:20270115T100000\nDTSTART;TZID=Europe/Oslo:20270116T100000\nDTEND;TZID=Europe/Oslo:20270116T200000",
			},
			want: []string{"2027-01-08", "2027-01-16", "2027-01-22"},
		},
		{
			name: "a cancelled time",
			events: []string{
				weekly + "RRULE:FREQ=WEEKLY;COUNT=3",
				"UID:a@google.com\nRECURRENCE-ID;TZID=Europe/Oslo:20270115T100000\nDTSTART;TZID=Europe/Oslo:20270115T100000\nDTEND;TZID=Europe/Oslo:20270115T200000\nSTATUS:CANCELLED",
			},
			want: []string{"2027-01-08", "2027-01-22"},
		},
		{
			// Six hours to midnight on Oslo's clock either side of both
			// changes; adding 7×24 hours in UTC would make some five.
			name: "evenings across both DST changes",
			events: []string{
				"UID:h\nDTSTART;TZID=Europe/Oslo:20270319T180000\nDTEND;TZID=Europe/Oslo:20270320T000000\nRRULE:FREQ=WEEKLY;COUNT=3",
				"UID:i\nDTSTART;TZID=Europe/Oslo:20271022T180000\nDTEND;TZID=Europe/Oslo:20271023T000000\nRRULE:FREQ=WEEKLY;COUNT=3",
			},
			want: []string{"2027-03-19", "2027-03-26", "2027-04-02", "2027-10-22", "2027-10-29", "2027-11-05"},
		},
		{
			name:   "a short weekly slot takes no day",
			events: []string{"UID:k\nDTSTART;TZID=Europe/Oslo:20270104T100000\nDTEND;TZID=Europe/Oslo:20270104T120000\nRRULE:FREQ=WEEKLY"},
		},
		{
			name:   "RDATE adds a time",
			events: []string{weekly + "RRULE:FREQ=WEEKLY;COUNT=1\nRDATE;TZID=Europe/Oslo:20270301T100000"},
			want:   []string{"2027-01-08", "2027-03-01"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, takenIn(t, feed(tt.events...)))
		})
	}
}

// RFC 5545: a rule from a UTC start repeats on UTC's clock, so after 28
// March the evening runs 19:00 to 01:00 in Oslo.
func TestRepeatInUTC(t *testing.T) {
	spans, _, err := parseAll(feed("UID:j\nDTSTART:20270319T170000Z\nDTEND:20270319T230000Z\nRRULE:FREQ=WEEKLY;COUNT=3"))
	require.NoError(t, err)
	require.Len(t, spans, 3)
	assert.Equal(t, 18, spans[0].start.In(site.Oslo).Hour())
	assert.Equal(t, 19, spans[2].start.In(site.Oslo).Hour())
}

// An endless rule costs only its times inside the window: from far back,
// every day, or every week.
func TestEndlessRepeat(t *testing.T) {
	got := takenIn(t, feed("UID:x\nDTSTART;VALUE=DATE:20200104\nDTEND;VALUE=DATE:20200105\nRRULE:FREQ=WEEKLY;BYDAY=SA"))

	var want []string

	for d := day("2026-01-03"); d.Year < 2029; d = d.AddDays(7) {
		want = append(want, d.String())
	}

	assert.Equal(t, want, got)

	got = takenIn(t, feed("UID:y\nDTSTART;VALUE=DATE:00010101\nDTEND;VALUE=DATE:00010102\nRRULE:FREQ=DAILY"))
	assert.Len(t, got, 3*365+1)
}

func TestUnsupportedRepeats(t *testing.T) {
	const start = "DTSTART:20270108T100000Z\nDTEND:20270108T200000Z\n"

	for name, tt := range map[string]struct{ event, says string }{
		"BYSETPOS":               {start + "RRULE:FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1", "part BYSETPOS=-1"},
		"hourly":                 {start + "RRULE:FREQ=HOURLY;COUNT=5", "part FREQ=HOURLY"},
		"BYWEEKNO":               {start + "RRULE:FREQ=YEARLY;BYWEEKNO=20", "part BYWEEKNO=20"},
		"EXRULE":                 {start + "RRULE:FREQ=WEEKLY\nEXRULE:FREQ=MONTHLY", "EXRULE"},
		"RDATE periods":          {start + "RDATE;VALUE=PERIOD:20270301T100000Z/PT2H", "RDATE;VALUE=PERIOD"},
		"this and future":        {start + "RRULE:FREQ=WEEKLY\nRECURRENCE-ID;RANGE=THISANDFUTURE:20270115T100000Z", "RANGE=THISANDFUTURE"},
		"wrong FREQ":             {start + "RRULE:FREQ=FORTNIGHTLY", "part FREQ=FORTNIGHTLY"},
		"twice":                  {start + "RRULE:FREQ=WEEKLY;FREQ=DAILY", "part FREQ=DAILY"},
		"COUNT and UNTIL":        {start + "RRULE:FREQ=WEEKLY;COUNT=3;UNTIL=20270301T000000Z", "both COUNT and UNTIL"},
		"weekly 2nd Sat":         {"DTSTART;VALUE=DATE:20270109\nDTEND;VALUE=DATE:20270110\nRRULE:FREQ=WEEKLY;BYDAY=2SA", "numbered BYDAY"},
		"yearly weekdays":        {"DTSTART;VALUE=DATE:20270109\nDTEND;VALUE=DATE:20270110\nRRULE:FREQ=YEARLY;BYDAY=SA", "BYDAY in a yearly rule"},
		"UNTIL a day for a time": {start + "RRULE:FREQ=WEEKLY;UNTIL=20270301", "UNTIL 20270301"},
		// The 31st of February never comes: RFC 5545 leaves a DTSTART off
		// its own rule undefined, so it is refused before any stepping.
		"never matches": {"DTSTART;VALUE=DATE:20270228\nDTEND;VALUE=DATE:20270301\nRRULE:FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=31", "DTSTART 2027-02-28 is not one of its days"},
		// Leap days, counted from year 4: the walk to the window would be
		// the whole calendar since then, past the read's budget.
		"too much work": {"DTSTART;VALUE=DATE:00040229\nDTEND;VALUE=DATE:00040301\nRRULE:FREQ=DAILY;BYMONTH=2;BYMONTHDAY=29;COUNT=100000", "too many times"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := parseAll(feed("UID:u\n" + tt.event))
			require.ErrorIs(t, err, errUnsupported)
			assert.Contains(t, err.Error(), "event UID u")
			assert.Contains(t, err.Error(), tt.says)
		})
	}
}

// What Kristine did wrong must be in the journal, and the calendar stays.
func TestUnsupportedRepeatKeepsTheCopyAndSaysWhy(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	body, status := weekend, 0

	var log bytes.Buffer

	f := Open(Config{
		URL: google(t, &body, &status), Path: path,
		Now: func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, site.Oslo) },
		Log: slog.New(slog.NewTextHandler(&log, nil)),
	})
	f.Refresh(context.Background())

	body = strings.Replace(weekend, "DTSTART:", "RRULE:FREQ=MONTHLY;BYSETPOS=2;BYDAY=SA\r\nDTSTART:", 1)
	f.Refresh(context.Background())

	days, ok := f.Shown()
	assert.True(t, ok)
	assert.Len(t, days, 3)
	assert.Contains(t, log.String(), "event 1: RRULE:FREQ=MONTHLY;BYSETPOS=2;BYDAY=SA")
	assert.Contains(t, log.String(), "part BYSETPOS=2")
	assert.Contains(t, log.String(), "result=refused")
}
