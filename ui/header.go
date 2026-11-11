package ui

import (
	"slices"
	"strings"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/static"
)

// MenuItem is one entry of the main menu.
type MenuItem struct {
	Label string
	Href  string
}

// Header is the logo and the main menu. Below lg (a phone, or a desktop
// zoomed in) the menu folds into a <details>: its <summary> is the burger,
// which a keyboard reaches and which tells a screen reader whether the menu
// is open, without JavaScript. The burger sits in the logo's row and the
// menu opens below it, as the old site's did. From lg the menu is a plain
// row; display: none hides the copy for the other width, so a screen reader
// only ever meets one.
// One item is marked as the current page and keeps its colour on hover: the
// one linking to current itself, else the section current lies in. An event
// in the menu lives under /arrangementer, so only its own page marks it.
func Header(menu []MenuItem, current string) elem.Node {
	marked := slices.IndexFunc(menu, func(m MenuItem) bool { return m.Href == current })
	if marked < 0 {
		marked = slices.IndexFunc(menu, func(m MenuItem) bool { return m.Href == "/"+firstSegment(current) })
	}

	items := make([]elem.Node, len(menu))
	for i, m := range menu {
		items[i] = headerLink(m, i == marked)
	}

	nav := func(p attrs.Props) elem.Node {
		p[attrs.AriaLabel] = "Hovedmeny"

		return elem.Nav(p, elem.Ul(attrs.Props{attrs.Class: "items-center justify-between text-base text-brown-link lg:flex"}, items...))
	}

	return elem.Header(attrs.Props{attrs.Class: "relative flex flex-wrap items-center px-4 py-0 border-b border-brown-border lg:px-16 lg:py-2"},
		// Below lg the burger is out of flow: mr-5 keeps its 20 px free, and
		// the logo's own width as basis keeps the menu's row from squeezing
		// in beside the logo at zero width.
		elem.Div(attrs.Props{attrs.Class: "flex flex-auto mr-5 justify-between items-center lg:flex-1 lg:mr-0"},
			elem.A(attrs.Props{attrs.Href: "/"},
				elem.Img(attrs.Props{
					// Squeezed between 1024 and 1280 px, it shrinks whole.
					attrs.Class: "h-28 object-contain object-left",
					attrs.Src:   static.URL("Aspargesgaarden_HOVEDLOGO_bredde_DempetSort.png"),
					attrs.Alt:   "Aspargesgården",
				}))),
		// Closed, the details holds only the summary, which is out of flow,
		// so its row has no height; open, the menu fills that row. h-28 is
		// the logo's height, so the burger stays centred beside the logo.
		elem.Details(attrs.Props{attrs.Class: "w-full lg:hidden"},
			elem.Summary(attrs.Props{attrs.Class: "absolute top-0 right-4 h-28 flex items-center cursor-pointer list-none [&::-webkit-details-marker]:hidden"},
				elem.Raw(burger), elem.Span(attrs.Props{attrs.Class: "sr-only"}, elem.Text("Meny"))),
			nav(attrs.Props{})),
		nav(attrs.Props{attrs.Class: "hidden lg:flex lg:items-center"}),
	)
}

// elem-go has no SVG elements; the classes inside are still scanned.
const burger = `<svg class="fill-current text-brown-link" width="20" height="20" viewBox="0 0 20 20" aria-hidden="true"><path d="M0 3h20v2H0V3zm0 6h20v2H0V9zm0 6h20v2H0v-2z"></path></svg>`

func headerLink(m MenuItem, current bool) elem.Node {
	link := attrs.Props{
		attrs.Href: m.Href,
		// The Elm list set both hovers for the current page and let the later
		// one win; v4 orders them itself, so only the winner is written.
		attrs.Class: cls("font-bold text-sm text-sort uppercase",
			onClass(current, "hover:text-brown-link"),
			onClass(!current, "hover:text-mork-eukalyptus")),
	}
	if current {
		link["aria-current"] = "page"
	}

	return elem.Li(attrs.Props{attrs.Class: "block py-3 px-0 lg:p-4 lg:px-6"}, elem.A(link, elem.Text(m.Label)))
}

func firstSegment(path string) string {
	seg, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")

	return seg
}
