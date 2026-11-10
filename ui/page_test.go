package ui

import (
	"testing"

	"github.com/chasefleming/elem-go"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/golden"
)

func testPage(path string) string {
	return Page(PageProps{
		BaseURL: "https://aspargesgaarden.no",
		Meta:    Meta{Title: "Muligheter", Description: "Lokaler & mat", Path: path},
		Menu:    testMenu,
	}, Container(Headline("Muligheter"), elem.P(nil, elem.Text("Tekst")))).Render()
}

func TestPage(t *testing.T) {
	golden.HTML(t, "page", testPage("/lokaler"))
}

func TestPageWithoutAddressHasNoCanonical(t *testing.T) {
	require.NotContains(t, testPage(""), `rel="canonical"`)
}

// The old site rendered the Umami tag as an empty <p>, so it never loaded.
func TestPageLoadsUmami(t *testing.T) {
	require.Contains(t, testPage("/"),
		`<script data-website-id="168bd9f6-fec6-40dc-95b5-b0e4082c7a98" defer src="https://umami.kradalby.no/umami.js"></script>`)
}
