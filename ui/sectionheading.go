package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// SectionHeading opens a second part of a page in the Headline's look, as
// an <h2>, since the page has its one <h1>.
func SectionHeading(text string) elem.Node {
	return elem.H2(attrs.Props{attrs.Class: "mt-16 " + headlineClass}, elem.Text(text))
}
