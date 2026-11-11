package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// Photographer credits a photo. Website is optional.
type Photographer struct {
	Name    string
	Website string
}

// PhotoCredit wraps a Picture with the photographer's name, shown in a bar
// along its bottom edge on hover (or tap), or while keyboard focus is in the
// photo or the credit. A click elsewhere on the bar goes through to the
// photo's link, as on the old site, where the whole photo opened the
// lightbox; only a mouse can click the name. A tap lands where the bar is
// still hidden, so it means the photo, and the lightbox offers the credit.
// Focus needs no pointer events: a tap focuses the photo's link between
// touch and click, and must not make the name the click's target then.
// Without a name there is no bar.
func PhotoCredit(by Photographer, picture elem.Node) elem.Node {
	kids := []elem.Node{picture}

	if by.Name != "" {
		var name elem.Node = elem.Text("Foto: " + by.Name)
		if by.Website != "" {
			name = elem.A(attrs.Props{
				attrs.Href:   by.Website,
				attrs.Target: "_blank",
				attrs.Rel:    "noopener noreferrer",
				attrs.Class:  "pointer-events-none pointer-fine:group-hover:pointer-events-auto",
			}, name)
		}

		kids = append(kids, elem.Div(attrs.Props{
			attrs.Class: "opacity-0 pointer-events-none absolute left-0 bottom-0 right-0 z-10 flex justify-around items-end bg-brown font-system text-black group-hover:opacity-100 group-focus-within:opacity-100",
		}, name))
	}

	return elem.Div(nil, elem.Div(attrs.Props{attrs.Class: "group relative"}, kids...))
}
