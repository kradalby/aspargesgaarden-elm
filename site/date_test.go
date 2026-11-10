package site

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// d is a Date from a literal known to be valid.
func d(s string) Date {
	date, err := ParseDate(s)
	if err != nil {
		panic(err)
	}

	return date
}

func TestParseDate(t *testing.T) {
	got, err := ParseDate("2026-11-14")
	require.NoError(t, err)
	require.Equal(t, Date{Year: 2026, Month: time.November, Day: 14}, got)
	require.Equal(t, "2026-11-14", got.String())

	for _, bad := range []string{"", "2026-2-3", "2026-02-30", "14.11.2026", "2026-11-14 12:00"} {
		_, err := ParseDate(bad)
		require.ErrorIs(t, err, ErrInvalidDate, bad)
	}
}

func TestDateZero(t *testing.T) {
	require.True(t, Date{}.IsZero())
	require.Empty(t, Date{}.String())
	require.False(t, d("2026-01-01").IsZero())
}

func TestDateCompare(t *testing.T) {
	a, b := d("2025-12-31"), d("2026-01-01")
	require.True(t, a.Before(b))
	require.True(t, b.After(a))
	require.False(t, a.After(d("2025-12-31")))
	require.Equal(t, 0, a.Compare(d("2025-12-31")))
	require.Equal(t, -1, d("2026-01-31").Compare(d("2026-02-01")))
}

func TestToday(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want Date
	}{
		{"winter, before Oslo midnight", time.Date(2026, 11, 15, 22, 59, 59, 0, time.UTC), d("2026-11-15")},
		{"winter, Oslo midnight", time.Date(2026, 11, 15, 23, 0, 0, 0, time.UTC), d("2026-11-16")},
		{"summer, Oslo midnight", time.Date(2026, 6, 30, 22, 0, 0, 0, time.UTC), d("2026-07-01")},
		{"summer, before Oslo midnight", time.Date(2026, 6, 30, 21, 59, 0, 0, time.UTC), d("2026-06-30")},
		{"other zone", time.Date(2026, 11, 15, 18, 0, 0, 0, time.FixedZone("EST", -5*3600)), d("2026-11-16")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Today(tt.now))
		})
	}
}
