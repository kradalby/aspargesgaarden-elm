package pages

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

// testNow is a Tuesday before the 2026 Christmas market.
var testNow = time.Date(2026, 11, 10, 12, 0, 0, 0, site.Oslo)

func date(s string) site.Date {
	d, err := site.ParseDate(s)
	if err != nil {
		panic(err)
	}

	return d
}

// testPhoto is a 1600×1200 photo with the import's centred crops.
func testPhoto(c byte, alt string) site.Photo {
	p := site.Photo{
		ID: int64(c), SHA256: strings.Repeat(string(c), 64), MIME: "image/jpeg", Width: 1600, Height: 1200, Alt: alt,
		Crops: map[images.Kind]images.Rect{},
	}

	for _, k := range []images.Kind{images.Card, images.Banner} {
		r, err := images.DefaultCrop(k, p.Width, p.Height)
		if err != nil {
			panic(err)
		}

		p.Crops[k] = r
	}

	return p
}

// testEvents covers what pages show differently: a coming event with every
// field, one without a summary, a past one with a ticket link, and a
// cancelled one.
func testEvents() []site.Event {
	main := testPhoto('a', "Bålfat på gårdsplassen")
	main.Photographer = &site.Photographer{ID: 1, Name: "Marianne Ous", URL: "https://www.instagram.com/marianneous"}
	extra := testPhoto('b', "Julepyntet låve")
	kurs := testPhoto('c', "Anita Saga ved bakebordet")
	kurs.Crops = nil // shown whole
	bettan := testPhoto('d', "Bettan synger")
	gourmet := testPhoto('e', "Dekket langbord")

	return []site.Event{
		{
			ID: 1, Year: 2026, Slug: "julemarked", Title: "Julemarked på Aspargesgården",
			Summary: "Mer enn 30 utstillere, kafé med hjemmebakst og bålfat på tunet.",
			BodyMD:  "# Velkommen til Julemarked på Aspargesgården!\n\nBli med oss.\n\n## Høydepunkter\n\n- Kafé\n- Bålfat\n\n> **Velkommen!**",
			Start:   date("2026-11-14"), StartTime: "12:00", End: date("2026-11-15"), EndTime: "16:00",
			PriceText: "Gratis inngang", TicketURL: "https://fabnite.com/julemarked",
			ShowInMenu: true, Status: site.Published, MainPhoto: &main, ExtraPhotos: []site.Photo{extra},
		},
		{
			ID: 2, Year: 2026, Slug: "takkebakstkurs-med-anita-saga", Title: "Takkebakstkurs med Anita Saga",
			BodyMD: "Invitasjon til takkebakstkurs på Aspargesgården i Tjølling.\n\n**Kursdatoer:**\n\n- 31. oktober\n\n**Sted:** Bryggerhuset på Aspargesgården.",
			Start:  date("2026-10-31"), StartTime: "10:00", EndTime: "15:00",
			ExtraDates: []site.Date{date("2026-11-28"), date("2026-11-29")},
			Status:     site.Published, MainPhoto: &kurs,
		},
		{
			ID: 3, Year: 2024, Slug: "varkonsert-med-bettan", Title: "Vårkonsert med Bettan",
			BodyMD: "Velkommen til en herlig vårkonsert.", Start: date("2024-05-29"), StartTime: "18:30",
			TicketURL: "https://fabnite.com/bettan", Status: site.Published, MainPhoto: &bettan,
		},
		{
			ID: 4, Year: 2024, Slug: "gourmetaften", Title: "Gourmetaften",
			BodyMD: "Landlig sjarm.", Start: date("2024-10-01"), Cancelled: true,
			Status: site.Published, MainPhoto: &gourmet,
		},
	}
}

func TestEvents(t *testing.T) {
	got := Events(testSite, testEvents(), testNow).Render()

	golden.HTML(t, "events", got)

	upcoming, past, ok := strings.Cut(got, "Tidligere arrangementer")
	require.True(t, ok)
	require.Contains(t, upcoming, "Julemarked på Aspargesgården")
	require.Contains(t, upcoming, "Takkebakstkurs med Anita Saga")
	require.Contains(t, past, "Vårkonsert med Bettan")
	require.Contains(t, past, "Gourmetaften")
	require.Less(t, strings.Index(upcoming, "Julemarked på"), strings.Index(upcoming, "Takkebakstkurs"), "soonest first: the course's next day is 28 November")
	require.NotContains(t, got, "Ingen kommende arrangementer")

	require.Contains(t, got, "/"+strings.Repeat("a", 64)+"/card-")
	require.Contains(t, got, "/"+strings.Repeat("c", 64)+"/card-", "no card crop: its centred one")
}

func TestEventsTeaserFallsBackToTheText(t *testing.T) {
	got := Events(testSite, testEvents(), testNow).Render()

	require.Contains(t, got, ">Mer enn 30 utstillere, kafé med hjemmebakst og bålfat på tunet.</p>")
	require.Contains(t, got, ">Invitasjon til takkebakstkurs på Aspargesgården i Tjølling. Sted: Bryggerhuset på Aspargesgården.</p>")
}

// The admin's "i listen" preview: a fresh draft has no title, date or
// photo yet, and is shown first among what is coming.
func TestEventsDraftPreview(t *testing.T) {
	draft := site.Event{ID: 99, Summary: "Ny kladd uten noe annet"}

	got := Events(testSite, append(testEvents(), draft), testNow).Render()

	upcoming, _, ok := strings.Cut(got, "Tidligere arrangementer")
	require.True(t, ok)
	require.Less(t, strings.Index(upcoming, "Ny kladd uten noe annet"), strings.Index(upcoming, "Julemarked på"))
	require.Contains(t, upcoming, `aspect-4/5 bg-brown/20`, "a placeholder holds the photo's place")
}

func TestEventsWithNothingComing(t *testing.T) {
	past := testEvents()[2:]

	require.Contains(t, Events(testSite, past, testNow).Render(),
		">Ingen kommende arrangementer for øyeblikket. Se tidligere arrangementer nedenfor for et inntrykk av hva som kan komme.</p>")

	empty := Events(testSite, nil, testNow).Render()
	require.Contains(t, empty, ">Ingen kommende arrangementer for øyeblikket.</p>")
	require.NotContains(t, empty, "Tidligere arrangementer")
}
