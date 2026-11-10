package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// ContactLink is the "Ta kontakt" button that leads to the contact page,
// centred under the text it closes.
func ContactLink() elem.Node {
	return elem.Div(attrs.Props{attrs.Class: "py-3 text-center"},
		ButtonLink(ButtonLinkProps{Href: "/kontakt", Label: "Ta kontakt"}))
}
