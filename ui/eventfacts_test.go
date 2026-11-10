package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestEventFacts(t *testing.T) {
	golden.HTML(t, "eventfacts", EventFacts(EventFactsProps{
		DateLabel: "Dato:",
		DateLine:  "01. desember 2025 kl 19:00 på Aspargesgården",
		Price:     "695,-",
		TicketURL: "https://fabnite.com/no/events/julekonsert",
	}).Render())

	bare := EventFacts(EventFactsProps{DateLabel: "Dato:", DateLine: "x"}).Render()
	require.NotContains(t, bare, "Pris:")
	require.NotContains(t, bare, "<a")
}
