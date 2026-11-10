package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// InfoLine is one fact with its icon, as the contact details. Label names
// the icon for readers who cannot see it ("Telefon"). A non-empty href makes
// the text a link ("tel:+47…"), so a phone can call the number.
func InfoLine(icon Icon, label, text, href string) elem.Node {
	var value elem.Node = elem.Text(text)
	if href != "" {
		value = elem.A(attrs.Props{attrs.Href: href}, value)
	}

	return elem.Div(attrs.Props{attrs.Class: "p-2 md:pl-0"},
		iconImg(icon, label, "w-12 pr-2 inline-block"),
		elem.P(attrs.Props{attrs.Class: "inline-block text-sort text-lg font-bold"}, value),
	)
}
