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

// Header is the logo and the main menu. Below lg the menu folds behind a
// burger that toggles it without JavaScript: the label checks a hidden
// checkbox, and the menu shows while it is checked (peer-checked).
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

	return elem.Header(attrs.Props{attrs.Class: "flex flex-wrap items-center px-4 py-0 border-b border-brown-border lg:px-16 lg:py-2"},
		elem.Div(attrs.Props{attrs.Class: "flex flex-1 justify-between items-center"},
			elem.A(attrs.Props{attrs.Href: "/"},
				elem.Img(attrs.Props{
					attrs.Class: "h-28",
					attrs.Src:   static.URL("Aspargesgaarden_HOVEDLOGO_bredde_DempetSort.png"),
					attrs.Alt:   "Aspargesgården",
				}))),
		elem.Label(attrs.Props{attrs.For: "menu-toggle", attrs.Class: "block cursor-pointer lg:hidden"}, elem.Raw(burger)),
		elem.Input(attrs.Props{attrs.Type: "checkbox", attrs.ID: "menu-toggle", attrs.Class: "peer hidden"}),
		elem.Div(attrs.Props{attrs.Class: "hidden w-full peer-checked:block lg:flex lg:items-center lg:w-auto"},
			elem.Nav(attrs.Props{attrs.AriaLabel: "Hovedmeny"},
				elem.Ul(attrs.Props{attrs.Class: "items-center justify-between text-base text-brown-link lg:flex"}, items...))),
	)
}

// elem-go has no SVG elements; the classes inside are still scanned.
const burger = `<svg class="fill-current text-brown-link" width="20" height="20" viewBox="0 0 20 20"><title>Meny</title><path d="M0 3h20v2H0V3zm0 6h20v2H0V9zm0 6h20v2H0v-2z"></path></svg>`

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
