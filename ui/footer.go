package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/static"
)

// Footer links to the farm's social media.
func Footer() elem.Node {
	return elem.Footer(attrs.Props{attrs.Class: "w-full text-center p-4"},
		elem.Div(nil,
			ContactPoint("https://facebook.com/aspargesgaarden", "fb.png", "Facebook"),
			ContactPoint("https://instagram.com/aspargesgaarden", "insta.png", "Instagram"),
		))
}

// ContactPoint is a social media icon that grows on hover and opens the
// profile in a new tab. icon names an embedded file.
func ContactPoint(href, icon, name string) elem.Node {
	return elem.A(attrs.Props{
		attrs.Class:  "inline-block w-14 h-14 p-2 transition duration-200 ease-in-out hover:scale-125",
		attrs.Href:   href,
		attrs.Target: "_blank",
		attrs.Rel:    "noopener noreferrer",
	}, elem.Img(attrs.Props{attrs.Src: static.URL(icon), attrs.Alt: name}))
}
