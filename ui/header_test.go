package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

var testMenu = []MenuItem{
	{Label: "Muligheter", Href: "/lokaler"},
	{Label: "Arrangementer", Href: "/arrangementer"},
	{Label: "Om gården", Href: "/om-oss"},
}

func TestHeader(t *testing.T) {
	golden.HTML(t, "header", Header(testMenu, "/lokaler").Render())
}

func TestHeaderMarksCurrentSection(t *testing.T) {
	tests := []struct {
		current string
		want    string
	}{
		{current: "/lokaler", want: "Muligheter"},
		{current: "/arrangementer/2026/julemarked", want: "Arrangementer"},
		{current: "/", want: ""},
		{current: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.current, func(t *testing.T) {
			got := Header(testMenu, tt.current).Render()

			if tt.want == "" {
				require.NotContains(t, got, "aria-current")

				return
			}

			require.Equal(t, 1, strings.Count(got, `aria-current="page"`))
			require.Contains(t, got, `aria-current="page" class="font-bold text-sm text-sort uppercase hover:text-brown-link" href=`)
			require.Contains(t, got, ">"+tt.want+"</a>")
		})
	}
}

// The old header gave the menu div and its list the same id.
func TestHeaderIDsAreUnique(t *testing.T) {
	require.Equal(t, 1, strings.Count(Header(testMenu, "/").Render(), ` id="`))
}
