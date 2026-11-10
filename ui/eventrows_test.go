package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func testEventItem() EventItem {
	return EventItem{
		Href:      "/arrangementer/2026/julemarked",
		Title:     "Julemarked på Aspargesgården",
		DateLabel: "Dato:",
		DateLine:  "14. november kl. 12.00 – 15. november 2026 kl. 16.00 på Aspargesgården",
		Date:      "14. november 2026",
		Day:       "2026-11-14",
		Summary:   "Mer enn 30 utstillere, kafé og bålfat.",
		Photo:     testPhoto(),
		Alt:       "Bålfat på gårdsplassen",
		By:        Photographer{Name: "Marianne Ous"},
	}
}

func TestEventRows(t *testing.T) {
	cancelled := testEventItem()
	cancelled.Cancelled = true
	cancelled.Summary = ""

	got := EventRows(testEventItem(), cancelled).Render()

	golden.HTML(t, "eventrows", got)
	require.Equal(t, 1, strings.Count(got, ">Avlyst<"))
	require.Equal(t, 1, strings.Count(got, "Mer enn 30 utstillere"), "no empty summary paragraph")
}

// The photo links to the event for a mouse; keyboards and screen readers get
// the title and the button, not a third copy of the same link.
func TestEventRowsPhotoLinkIsHidden(t *testing.T) {
	got := EventRows(testEventItem()).Render()

	require.Contains(t, got, `<a aria-hidden="true" class="block" href="/arrangementer/2026/julemarked" tabindex="-1"><picture>`)
	require.Contains(t, got, `<a aria-label="Les mer om Julemarked på Aspargesgården"`)
}
