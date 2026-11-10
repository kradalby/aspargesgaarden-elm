package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// EventColumn is an event page's reading column: a comfortable line length
// centred on wide screens, inset on a phone.
func EventColumn(kids ...elem.Node) elem.Node {
	return Container(elem.Article(attrs.Props{attrs.Class: "max-w-4xl mx-auto w-full pb-8 px-4 text-sort md:px-0"}, kids...))
}
