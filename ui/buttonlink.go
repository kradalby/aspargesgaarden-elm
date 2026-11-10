package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// ButtonLinkProps configures a ButtonLink.
type ButtonLinkProps struct {
	Href  string
	Label string
	// AriaLabel names the target where Label alone is vague ("Les mer");
	// it must start with Label so speech input still finds the button.
	AriaLabel string
	// External opens another site in a new tab.
	External bool
}

// ButtonLink is a link that looks like a button: the one thing a reader
// should do next.
func ButtonLink(p ButtonLinkProps) elem.Node {
	a := attrs.Props{
		attrs.Href:  p.Href,
		attrs.Class: "inline-block mt-2 px-6 py-3 bg-mose text-white text-sm font-bold uppercase tracking-wide hover:bg-mork-eukalyptus",
	}
	if p.AriaLabel != "" {
		a[attrs.AriaLabel] = p.AriaLabel
	}

	if p.External {
		a[attrs.Target] = "_blank"
		a[attrs.Rel] = "noopener noreferrer"
	}

	return elem.A(a, elem.Text(p.Label))
}
