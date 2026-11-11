package ui

import (
	"strings"
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

// The bar never catches a tap meant for the photo's link, its name only a
// mouse's, and it shows when a keyboard reaches its link.
func TestPhotoCreditBarLetsTapsThrough(t *testing.T) {
	got := PhotoCredit(Photographer{Name: "Aurora Hovland", Website: "https://www.aurorahovland.com"}, elem.Span(nil)).Render()

	require.Contains(t, got, `<a class="pointer-events-none pointer-fine:group-hover:pointer-events-auto"`)
	require.Contains(t, got, `<div class="opacity-0 pointer-events-none `)
	require.Contains(t, got, "group-focus-within:opacity-100")
	require.Equal(t, 3, strings.Count(got, "pointer-events-"), "only a mouse over the name turns them on")
}

// The old site showed an empty "av " bar when nobody was credited.
func TestPhotoCreditWithoutPhotographer(t *testing.T) {
	got := PhotoCredit(Photographer{}, elem.Span(nil)).Render()

	require.NotContains(t, got, "Foto:")
	require.NotContains(t, got, "group-hover")
}
