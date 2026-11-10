package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// Lead is the short opening text of a page, larger than the body text.
func Lead(text string) elem.Node {
	return elem.P(attrs.Props{attrs.Class: "pb-4 text-xl text-sort"}, elem.Text(text))
}
