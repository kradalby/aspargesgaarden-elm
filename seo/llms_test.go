package seo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/booking"
	"github.com/kradalby/asparges/site"
)

func TestLLMs(t *testing.T) {
	kurs := site.Event{
		Year: 2026, Slug: "takkebakstkurs", Title: "Takkebakstkurs", Start: date("2026-11-28"),
		StartTime: "10:00", EndTime: "15:00", Cancelled: true,
	}

	got := LLMs(testBase,
		[]Link{{Title: "Selskapslokaler: låven, fjøset og hagen", Path: "/lokaler"}, {Title: "Kontakt oss – Tjølling, Larvik", Path: "/kontakt"}},
		[]site.Event{testEvent(), kurs}, booking.Calendar{})

	require.Equal(t, `# Aspargesgården

> Selskapslokaler i låve og fjøs på en gård i Tjølling, Larvik, ved kysten i Vestfold. Låvebryllup er spesialiteten; gården tar også imot konfirmasjoner, dåp, minnestunder, bursdager, firmasamlinger, kurs, konserter og konferanser.

- Adresse: Østbyveien 75, 3280 Tjodalyng (Larvik kommune, Vestfold)
- Telefon: +47 90 91 98 12
- Nettside: https://aspargesgaarden.no/
- Facebook: https://facebook.com/aspargesgaarden
- Instagram: https://instagram.com/aspargesgaarden

## Lokaler

- Låven: plass til 100 gjester
- Fjøset: plass til 68 gjester

## Sider

- [Selskapslokaler: låven, fjøset og hagen](https://aspargesgaarden.no/lokaler)
- [Kontakt oss – Tjølling, Larvik](https://aspargesgaarden.no/kontakt)

## Kommende arrangementer

- [Julemarked på Aspargesgården](https://aspargesgaarden.no/arrangementer/2026/julemarked): 14.–15. november 2026 kl. 12.00–16.00
- [Takkebakstkurs](https://aspargesgaarden.no/arrangementer/2026/takkebakstkurs): 28. november 2026 kl. 10.00–15.00 (avlyst)
`, got)
}

func TestLLMsWithoutEvents(t *testing.T) {
	require.Contains(t, LLMs(testBase, nil, nil, booking.Calendar{}), "## Kommende arrangementer\n\nIngen kommende arrangementer for øyeblikket.\n")
}

func TestLLMsFreeWeekends(t *testing.T) {
	taken := booking.Days{}
	// The weekend of 13–15 November whole, and Saturday 28 November.
	for _, d := range []string{"2026-11-13", "2026-11-14", "2026-11-15", "2026-11-28"} {
		taken[date(d)] = true
	}

	for d := date("2027-06-01"); d.Month == 6; d = d.AddDays(1) {
		taken[d] = true
	}

	// Not while the site cannot vouch for it.
	require.NotContains(t, LLMs(testBase, nil, nil, booking.Calendar{Unavailable: true}), "Ledige helger")

	got := LLMs(testBase, nil, nil, booking.NewCalendar(taken, date("2026-11-11")))

	require.Contains(t, got, `## Ledige helger

Fra bookingkalenderen, november 2026 – oktober 2027. En helg er fredag til søndag. Ring +47 90 91 98 12 for å bekrefte en dato.

- November 2026: 20.–22. november, 27.–29. november (fredag og søndag)
- Desember 2026 – mai 2027: alle helgene
- Juni 2027: ingen
- Juli – oktober 2027: alle helgene
`)
}
