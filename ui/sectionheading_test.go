package ui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSectionHeading(t *testing.T) {
	require.Equal(t,
		`<h2 class="mt-16 text-center py-6 text-4xl font-medium font-display text-tre md:text-left md:text-6xl lg:pt-12">Tidligere arrangementer</h2>`,
		SectionHeading("Tidligere arrangementer").Render())
}
