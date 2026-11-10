package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
)

func TestPictureOrPlaceholder(t *testing.T) {
	draft := testEventItem()
	draft.Photo, draft.Alt, draft.By = images.Ref{}, "", Photographer{}

	for name, got := range map[string]string{
		"rows":  EventRows(draft).Render(),
		"cards": EventCards(draft).Render(),
	} {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, got, `aspect-4/5 bg-brown/20`)
			require.NotContains(t, got, "<picture>")
			require.NotContains(t, got, "<img")
		})
	}

	require.Contains(t, EventRows(testEventItem()).Render(), "<picture>", "a photo is still a picture")
}
