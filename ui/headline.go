package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// headlineClass is the old site's one heading look, which it gave the page
// title, every event's title in the list and "Tidligere arrangementer".
const headlineClass = "text-center py-6 text-4xl font-medium font-display text-tre md:text-left md:text-6xl lg:pt-12"

// Headline is the page's <h1> in the display font. A page has exactly one
// (pages_test.go checks); a second heading in the same look must be an <h2>.
func Headline(text string) elem.Node {
	return elem.H1(attrs.Props{attrs.Class: headlineClass}, elem.Text(text))
}
