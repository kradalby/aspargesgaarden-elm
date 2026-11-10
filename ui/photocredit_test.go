package ui

import (
	"testing"

	"github.com/chasefleming/elem-go"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestPhotoCredit(t *testing.T) {
	pic := elem.Span(nil)

	golden.HTML(t, "photocredit", elem.Fragment(
		PhotoCredit(Photographer{Name: "Aurora Hovland", Website: "https://www.aurorahovland.com"}, pic),
		PhotoCredit(Photographer{Name: "Hanne Gulliksen"}, pic),
	).Render())
}

// The hidden bar must not catch a tap meant for the photo's link, and must
// show when a keyboard reaches its link.
func TestPhotoCreditHiddenBarLetsTapsThrough(t *testing.T) {
	got := PhotoCredit(Photographer{Name: "Aurora Hovland", Website: "https://www.aurorahovland.com"}, elem.Span(nil)).Render()

	for _, c := range []string{"opacity-0 pointer-events-none", "group-focus-within:opacity-100", "group-focus-within:pointer-events-auto", "group-hover:pointer-events-auto"} {
		require.Contains(t, got, c)
	}
}

// The old site showed an empty "av " bar when nobody was credited.
func TestPhotoCreditWithoutPhotographer(t *testing.T) {
	got := PhotoCredit(Photographer{}, elem.Span(nil)).Render()

	require.NotContains(t, got, "av ")
	require.NotContains(t, got, "group-hover")
}
