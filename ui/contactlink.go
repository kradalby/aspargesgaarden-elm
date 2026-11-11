package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// ContactLink is the "Ta kontakt" button that leads to the contact page,
// centred under the text it closes: the old site's brush stroke, with its
// words as text. The stroke is the old artwork with its lettering removed,
// at the old sizes (144×50, from lg 176×61).
func ContactLink() elem.Node {
	return elem.Div(attrs.Props{attrs.Class: "py-3"},
		elem.A(attrs.Props{
			attrs.Href:  "/kontakt",
			attrs.Class: "mx-auto flex w-36 aspect-[1026/358] items-center justify-center bg-[url(/static/brush-stroke.png)] bg-contain bg-no-repeat text-white text-base uppercase tracking-wider hover:brightness-90 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-sort lg:w-44 lg:text-lg",
		}, elem.Text("Ta kontakt")))
}
