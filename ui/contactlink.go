package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/static"
)

// ContactLink is the "Ta kontakt" button that leads to the contact page.
func ContactLink() elem.Node {
	return elem.Div(attrs.Props{attrs.Class: "w-36 py-3 mx-auto lg:w-44"},
		elem.A(attrs.Props{attrs.Href: "/kontakt"},
			elem.Img(attrs.Props{attrs.Src: static.URL("takontakt.png"), attrs.Alt: "Ta kontakt"})))
}
