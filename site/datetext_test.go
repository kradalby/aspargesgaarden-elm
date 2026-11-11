package site

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDateLineLegacy(t *testing.T) {
	for _, l := range legacyEvents {
		t.Run(l.file, func(t *testing.T) {
			require.Equal(t, l.label, l.event.DateLabel())
			require.Equal(t, l.line, l.event.DateLine())
		})
	}
}

func TestDays(t *testing.T) {
	for want, e := range map[string]Event{
		"1. oktober 2024":                    {Start: d("2024-10-01")},
		"19. mai 2026":                       {Start: d("2026-05-19"), End: d("2026-05-19")},
		"14.–15. november 2026":              {Start: d("2026-11-14"), End: d("2026-11-15"), StartTime: "12:00", EndTime: "16:00"},
		"30. november – 1. desember 2026":    {Start: d("2026-11-30"), End: d("2026-12-01")},
		"31. desember 2026 – 1. januar 2027": {Start: d("2026-12-31"), End: d("2027-01-01")},
		"31. oktober 2026":                   {Start: d("2026-10-31"), ExtraDates: []Date{d("2026-11-28")}},
	} {
		require.Equal(t, want, e.Days())
	}
}

func TestDateLine(t *testing.T) {
	tests := []struct {
		name  string
		event Event
		want  string
	}{
		{
			name:  "draft without date",
			event: Event{},
			want:  "",
		},
		{
			name:  "day only",
			event: Event{Start: d("2024-10-01")},
			want:  "1. oktober 2024 på Aspargesgården",
		},
		{
			name:  "start time",
			event: Event{Start: d("2026-05-19"), StartTime: "09:05"},
			want:  "19. mai 2026 kl. 09.05 på Aspargesgården",
		},
		{
			name:  "end time without end day is the same day",
			event: Event{Start: d("2026-05-19"), StartTime: "19:30", EndTime: "22:00"},
			want:  "19. mai 2026 kl. 19.30–22.00 på Aspargesgården",
		},
		{
			name:  "end time alone",
			event: Event{Start: d("2026-05-19"), EndTime: "22:00"},
			want:  "19. mai 2026 til kl. 22.00 på Aspargesgården",
		},
		{
			name:  "end day equal to the start is one day",
			event: Event{Start: d("2026-05-19"), End: d("2026-05-19"), StartTime: "19:30", EndTime: "22:00"},
			want:  "19. mai 2026 kl. 19.30–22.00 på Aspargesgården",
		},
		// Over several days, the hours are each day's.
		{
			name:  "several days with both times",
			event: Event{Start: d("2026-11-14"), StartTime: "12:00", End: d("2026-11-15"), EndTime: "16:00"},
			want:  "14.–15. november 2026 kl. 12.00–16.00 på Aspargesgården",
		},
		{
			name:  "several days without times",
			event: Event{Start: d("2026-11-14"), End: d("2026-11-15")},
			want:  "14.–15. november 2026 på Aspargesgården",
		},
		{
			name:  "several days across a month",
			event: Event{Start: d("2026-11-30"), End: d("2026-12-01"), StartTime: "12:00", EndTime: "16:00"},
			want:  "30. november – 1. desember 2026 kl. 12.00–16.00 på Aspargesgården",
		},
		{
			name:  "several days across a new year",
			event: Event{Start: d("2026-12-31"), End: d("2027-01-01")},
			want:  "31. desember 2026 – 1. januar 2027 på Aspargesgården",
		},
		{
			name:  "several days with the start time only",
			event: Event{Start: d("2026-11-14"), StartTime: "12:00", End: d("2026-11-15")},
			want:  "14.–15. november 2026 kl. 12.00 på Aspargesgården",
		},
		{
			name:  "several days with the end time only",
			event: Event{Start: d("2026-11-14"), End: d("2026-11-15"), EndTime: "16:00"},
			want:  "14.–15. november 2026 til kl. 16.00 på Aspargesgården",
		},
		{
			name:  "two days",
			event: Event{Start: d("2026-01-05"), ExtraDates: []Date{d("2026-02-02")}, StartTime: "18:00"},
			want:  "5. januar og 2. februar 2026 kl. 18.00 på Aspargesgården",
		},
		{
			name: "days across a new year",
			event: Event{
				Start: d("2026-12-28"), ExtraDates: []Date{d("2027-01-04"), d("2027-01-11")},
				StartTime: "18:00", EndTime: "20:00",
			},
			want: "28. desember 2026, 4. januar og 11. januar 2027 kl. 18.00–20.00 på Aspargesgården",
		},
		{
			name:  "extra days ignore the end day, as the old site did",
			event: Event{Start: d("2026-01-05"), End: d("2026-01-06"), ExtraDates: []Date{d("2026-02-02")}},
			want:  "5. januar og 2. februar 2026 på Aspargesgården",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.event.DateLine())
		})
	}
}

func TestNorwegianDateMonths(t *testing.T) {
	want := []string{
		"1. januar 2026", "1. februar 2026", "1. mars 2026", "1. april 2026",
		"1. mai 2026", "1. juni 2026", "1. juli 2026", "1. august 2026",
		"1. september 2026", "1. oktober 2026", "1. november 2026", "1. desember 2026",
	}
	for i, w := range want {
		require.Equal(t, w, NorwegianDate(Date{Year: 2026, Month: time.Month(i + 1), Day: 1}))
	}

	require.Equal(t, "31. desember 2026", NorwegianDate(Date{Year: 2026, Month: time.December, Day: 31}))
}
