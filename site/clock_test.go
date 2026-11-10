package site

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidClock(t *testing.T) {
	for _, ok := range []string{"00:00", "09:30", "19:00", "23:59"} {
		require.True(t, ValidClock(ok), ok)
	}
	for _, bad := range []string{"", "9:30", "24:00", "19:60", "19.00", "ca. 15:00", "19:00 "} {
		require.False(t, ValidClock(bad), bad)
	}
}

func TestParseLegacyTime(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		approx bool
	}{
		{"19:00", "19:00", false},
		{"ca. 15:00", "15:00", true},
		{"Ca 15", "15:00", true},
		{"ca.15:00", "15:00", true},
		{"cirka 15.30", "15:30", true},
		{"kl. 19.30", "19:30", false},
		{"kl 9", "09:00", false},
		{"ca. kl. 15", "15:00", true},
		{" 9:05 ", "09:05", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, approx, err := ParseLegacyTime(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.approx, approx)
			require.True(t, ValidClock(got))
		})
	}

	for _, bad := range []string{"", "15-16", "15:00-16:00", "etter middag", "24:00", "19:75", "ca."} {
		_, _, err := ParseLegacyTime(bad)
		require.ErrorIs(t, err, ErrInvalidTime, bad)
	}
}
