package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// Headline is the page's <h1> in the display font. A page has exactly one
// (pages_test.go checks); a second heading in the same look must be an <h2>.
func Headline(text string) elem.Node {
	// The old Elm list also had pt-10 (overridden by py-6) and xl:pt-12
	// (same as lg:); only what rendered is kept.
	return elem.H1(attrs.Props{attrs.Class: "text-center py-6 text-4xl font-medium font-display text-tre md:text-left md:text-6xl lg:pt-12"}, elem.Text(text))
}
