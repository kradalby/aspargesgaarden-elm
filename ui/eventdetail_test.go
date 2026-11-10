package ui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventDetail(t *testing.T) {
	require.Equal(t,
		`<p class="pb-3 text-sort"><strong class="font-bold">Pris:</strong> 695,- &amp; mat</p>`,
		EventDetail("Pris:", "695,- & mat").Render())
}
