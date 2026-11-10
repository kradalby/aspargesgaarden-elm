package seo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVenue(t *testing.T) {
	require.Equal(t, "Østbyveien 75, 3280 Tjodalyng", Venue.AddressLine())
	require.Equal(t, 100, Venue.Guests("Låven"))
	require.Equal(t, 68, Venue.Guests("Fjøset"))
	require.Panics(t, func() { Venue.Guests("Stabburet") })
}
