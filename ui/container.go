package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// Container is the page column: full width on a phone, inset from md.
func Container(kids ...elem.Node) elem.Node {
	return elem.Div(attrs.Props{attrs.Class: "flex flex-col md:px-24"}, kids...)
}
