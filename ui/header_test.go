package ui

import (
	"slices"
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

			require.Equal(t, 2, strings.Count(got, `aria-current="page"`), "once in each copy of the menu")
			require.Contains(t, got, `aria-current="page" class="font-bold text-sm text-sort uppercase hover:text-brown-link" href=`)
			require.Contains(t, got, ">"+tt.want+"</a>")
		})
	}
}

// An event in the menu sits under /arrangementer; only its own page marks
// it, and the list and other events still mark Arrangementer.
func TestHeaderMarksEventInMenu(t *testing.T) {
	menu := append(slices.Clone(testMenu), MenuItem{Label: "Julemarked", Href: "/arrangementer/2026/julemarked"})

	for current, want := range map[string]string{
		"/arrangementer":                 `href="/arrangementer">Arrangementer</a>`,
		"/arrangementer/2026/julemarked": `href="/arrangementer/2026/julemarked">Julemarked</a>`,
		"/arrangementer/2025/julemarked": `href="/arrangementer">Arrangementer</a>`,
	} {
		got := Header(menu, current).Render()

		require.Equal(t, 2, strings.Count(got, `aria-current="page"`), current)
		require.Contains(t, got, `aria-current="page" class="font-bold text-sm text-sort uppercase hover:text-brown-link" `+want, current)
	}
}

// The old site's menu was a hidden checkbox no keyboard could reach. The
// menu now comes twice, and each copy must be display: none at the other's
// widths, so a screen reader meets one.
func TestHeaderMenuIsReachable(t *testing.T) {
	got := Header(testMenu, "/").Render()

	require.NotContains(t, got, "<input")
	require.NotContains(t, got, " id=\"")
	require.Equal(t, 2, strings.Count(got, `<nav aria-label="Hovedmeny"`))
	require.Contains(t, got, `<details class="w-full lg:hidden"><summary `)
	require.Contains(t, got, `<nav aria-label="Hovedmeny" class="hidden lg:flex lg:items-center">`)
	require.Contains(t, got, `<span class="sr-only">Meny</span></summary>`)
}
