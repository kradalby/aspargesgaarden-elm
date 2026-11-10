package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func TestButtonLink(t *testing.T) {
	golden.HTML(t, "buttonlink", ButtonLink(ButtonLinkProps{Href: "/arrangementer/2026/julemarked", Label: "Les mer", AriaLabel: "Les mer om Julemarked"}).Render())

	external := ButtonLink(ButtonLinkProps{Href: "https://fabnite.com/x", Label: "Kjøp billett", External: true}).Render()
	require.Contains(t, external, `rel="noopener noreferrer" target="_blank"`)
	require.NotContains(t, external, "aria-label")
}
