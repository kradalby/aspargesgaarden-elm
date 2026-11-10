package ui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSectionHeading(t *testing.T) {
	got := SectionHeading("Tidligere arrangementer").Render()

	require.Contains(t, got, `<h2 class="text-center mt-16 py-6 text-3xl font-display text-tre md:text-left md:text-5xl">`)
	require.Contains(t, got, ">Tidligere arrangementer</h2>")
}
