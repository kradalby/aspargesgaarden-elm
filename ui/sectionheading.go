package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// SectionHeading opens a second part of a page in the Headline's look, one
// step quieter and as an <h2>, since the page has its one <h1>.
func SectionHeading(text string) elem.Node {
	return elem.H2(attrs.Props{attrs.Class: "text-center mt-16 py-6 text-3xl font-display text-tre md:text-left md:text-5xl"}, elem.Text(text))
}
