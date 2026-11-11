package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestEventPoster(t *testing.T) {
	got := EventPoster(testPhoto(), "Plakat: julemarked 14.–15. november", Photographer{}).Render()

	golden.HTML(t, "eventposter", got)
	require.Contains(t, got, `href="/img/x-1600.jpeg"`, "links the largest file")
	require.Contains(t, got, `loading="lazy"`)
}
