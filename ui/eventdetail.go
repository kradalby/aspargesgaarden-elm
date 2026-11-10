package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// EventDetail is one fact about an event, its label in bold: "Dato: …".
// label carries its own colon, as site.Event.DateLabel does.
func EventDetail(label, value string) elem.Node {
	return elem.P(attrs.Props{attrs.Class: "pb-3 text-sort"},
		elem.Strong(attrs.Props{attrs.Class: "font-bold"}, elem.Text(label)),
		elem.Text(" "+value))
}
