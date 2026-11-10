package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestEventCards(t *testing.T) {
	cancelled := testEventItem()
	cancelled.Cancelled = true

	got := EventCards(testEventItem(), cancelled).Render()

	golden.HTML(t, "eventcards", got)
	require.Equal(t, 2, strings.Count(got, "<h3"))
	require.Equal(t, 1, strings.Count(got, ">Avlyst<"))
	require.NotContains(t, got, "fetchpriority", "past events are never the page's main image")
}
