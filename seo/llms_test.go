package seo

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/site"
)

func TestLLMs(t *testing.T) {
	kurs := site.Event{
		Year: 2026, Slug: "takkebakstkurs", Title: "Takkebakstkurs", Start: date("2026-11-28"),
		StartTime: "10:00", EndTime: "15:00", Cancelled: true,
	}

	got := LLMs(testBase,
		[]Link{{Title: "Selskapslokaler: låven, fjøset og hagen", Path: "/lokaler"}, {Title: "Kontakt oss – Tjølling, Larvik", Path: "/kontakt"}},
		[]site.Event{testEvent(), kurs})

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
	require.Contains(t, LLMs(testBase, nil, nil), "## Kommende arrangementer\n\nIngen kommende arrangementer for øyeblikket.\n")
}
