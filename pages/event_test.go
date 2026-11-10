package pages

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestEvent(t *testing.T) {
	got := Event(testSite, testEvents()[0], testNow).Render()

	golden.HTML(t, "event", got)
	require.Contains(t, got, `<link href="https://aspargesgaarden.no/arrangementer/2026/julemarked" rel="canonical">`)
	require.Contains(t, got, `<meta content="14. november 2026 på Aspargesgården i Tjølling, Larvik: Mer enn 30 utstillere, kafé med hjemmebakst og bålfat på tunet." name="description">`)
	require.Contains(t, got, `href="https://fabnite.com/julemarked"`)
	require.Contains(t, got, "/"+strings.Repeat("a", 64)+"/banner-")
	require.Contains(t, got, "/"+strings.Repeat("b", 64)+"/full-")
	require.Contains(t, got, "av Marianne Ous")
	require.Contains(t, got, `<p class="pb-4 text-xl text-sort">Mer enn 30`)
	require.NotContains(t, got, "Arrangementet er")
}

// Further photos are a gallery with its lightbox; an event without them
// loads no script.
func TestEventGallery(t *testing.T) {
	events := testEvents()
	require.NotEmpty(t, events[0].ExtraPhotos)

	got := Event(testSite, events[0], testNow).Render()
	require.Contains(t, got, `data-gallery="`)
	require.Contains(t, got, "/static/gallery.js")

	events[0].ExtraPhotos = nil
	require.NotContains(t, Event(testSite, events[0], testNow).Render(), "gallery.js")
}

func TestEventWithoutSummaryHasNoLead(t *testing.T) {
	got := Event(testSite, testEvents()[1], testNow).Render()

	require.NotContains(t, got, "text-xl")
	require.Contains(t, got, `<meta content="31. oktober 2026 på Aspargesgården i Tjølling, Larvik: Invitasjon til takkebakstkurs`)
	require.Contains(t, got, "<strong class=\"font-bold\">Datoer:</strong> 31. oktober 2026, 28. november 2026, 29. november 2026 kl 10:00 - 15:00")
}

// A past event stays at its address, says it is over and sells nothing.
func TestEventOver(t *testing.T) {
	e := testEvents()[0]

	during := Event(testSite, e, time.Date(2026, 11, 15, 23, 59, 0, 0, testNow.Location())).Render()
	require.NotContains(t, during, "Arrangementet er over")
	require.Contains(t, during, "fabnite.com/julemarked")

	after := Event(testSite, e, time.Date(2026, 11, 16, 0, 0, 0, 0, testNow.Location())).Render()
	require.Contains(t, after, "Arrangementet er over.")
	require.NotContains(t, after, "fabnite.com/julemarked")
	require.Contains(t, after, "Gratis inngang", "the price stays for the record")
}

func TestEventCancelled(t *testing.T) {
	e := testEvents()[0]
	e.Cancelled = true

	got := Event(testSite, e, testNow).Render()
	require.Contains(t, got, "Arrangementet er avlyst.")
	require.NotContains(t, got, "fabnite.com/julemarked")
	require.Equal(t, 1, strings.Count(got, "Arrangementet er"))
}
