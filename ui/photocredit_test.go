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

// The old site showed an empty "av " bar when nobody was credited.
func TestPhotoCreditWithoutPhotographer(t *testing.T) {
	got := PhotoCredit(Photographer{}, elem.Span(nil)).Render()

	require.NotContains(t, got, "av ")
	require.NotContains(t, got, "group-hover")
}
