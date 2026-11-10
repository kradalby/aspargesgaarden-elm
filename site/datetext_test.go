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
			name:  "end time without end day is the same day",
			event: Event{Start: d("2026-05-19"), StartTime: "19:30", EndTime: "22:00"},
			want:  "19. mai 2026 kl 19:30 - 22:00 på Aspargesgården",
		},
		{
			name:  "end day without times",
			event: Event{Start: d("2026-12-31"), End: d("2027-01-01")},
			want:  "31. desember 2026 - 01. januar 2027 på Aspargesgården",
		},
		{
			name:  "extra days ignore the end day, as the old site did",
			event: Event{Start: d("2026-01-05"), End: d("2026-01-06"), ExtraDates: []Date{d("2026-02-02")}},
			want:  "05. januar 2026, 02. februar 2026 på Aspargesgården",
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
		"01. januar 2026", "01. februar 2026", "01. mars 2026", "01. april 2026",
		"01. mai 2026", "01. juni 2026", "01. juli 2026", "01. august 2026",
		"01. september 2026", "01. oktober 2026", "01. november 2026", "01. desember 2026",
	}
	for i, w := range want {
		require.Equal(t, w, NorwegianDate(Date{Year: 2026, Month: time.Month(i + 1), Day: 1}))
	}
}
