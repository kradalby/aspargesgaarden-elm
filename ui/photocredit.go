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
// along its bottom edge on hover (or tap). Without a name there is no bar.
func PhotoCredit(by Photographer, picture elem.Node) elem.Node {
	kids := []elem.Node{picture}

	if by.Name != "" {
		var name elem.Node = elem.Text("av " + by.Name)
		if by.Website != "" {
			name = elem.A(attrs.Props{
				attrs.Href:   by.Website,
				attrs.Target: "_blank",
				attrs.Rel:    "noopener noreferrer",
			}, name)
		}

		kids = append(kids, elem.Div(attrs.Props{
			attrs.Class: "opacity-0 absolute left-0 bottom-0 right-0 z-10 flex justify-around items-end bg-brown text-black group-hover:opacity-100",
		}, name))
	}

	return elem.Div(nil, elem.Div(attrs.Props{attrs.Class: "group relative"}, kids...))
}
