package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/static"
)

// siteName prefixes every <title>, as on the old site.
const siteName = "Aspargesgården"

// Meta is what a page says about itself to browsers and search engines. The
// SEO layer extends this struct (Open Graph, JSON-LD) without touching pages
// that do not need more.
type Meta struct {
	// Title is the page's own part of <title>: "Aspargesgården | <Title>".
	Title       string
	Description string
	// Path is the canonical path, e.g. "/lokaler". Empty for pages that have
	// no address of their own (404), which then get no canonical link.
	Path string
}

// PageProps configures a Page.
type PageProps struct {
	// BaseURL is the public origin, without a trailing slash.
	BaseURL string
	Meta    Meta
	// Menu is the main menu, in order.
	Menu []MenuItem
}

// Page is a whole HTML document: head, header, the body nodes, footer and
// analytics. Every public page goes through it.
func Page(p PageProps, body ...elem.Node) *elem.Element {
	head := []elem.Node{
		elem.Meta(attrs.Props{attrs.Charset: "utf-8"}),
		elem.Meta(attrs.Props{attrs.Name: "viewport", attrs.Content: "width=device-width, initial-scale=1"}),
		elem.Title(nil, elem.Text(siteName+" | "+p.Meta.Title)),
		elem.Meta(attrs.Props{attrs.Name: "description", attrs.Content: p.Meta.Description}),
	}

	if p.Meta.Path != "" {
		head = append(head, elem.Link(attrs.Props{attrs.Rel: "canonical", attrs.Href: p.BaseURL + p.Meta.Path}))
	}

	head = append(head,
		elem.Link(attrs.Props{attrs.Rel: "icon", attrs.Href: static.URL("favicon.ico"), attrs.Sizes: "any"}),
		elem.Link(attrs.Props{attrs.Rel: "icon", attrs.Href: static.URL("favicon.svg"), attrs.Type: "image/svg+xml"}),
	)

	// Every font is above the fold on every page (menu, headline, text), and
	// the stylesheet would only ask for them once it has been parsed.
	for _, u := range static.FontURLs() {
		head = append(head, elem.Link(attrs.Props{
			attrs.Rel:         "preload",
			attrs.Href:        u,
			"as":              "font",
			attrs.Type:        "font/woff2",
			attrs.Crossorigin: "anonymous",
		}))
	}

	head = append(head,
		elem.Link(attrs.Props{attrs.Rel: "stylesheet", attrs.Href: static.URL("site.css")}),
		elem.Script(attrs.Props{attrs.Src: umamiScript, "data-website-id": umamiSiteID, attrs.Defer: "true"}),
	)

	kids := append([]elem.Node{Header(p.Menu, p.Meta.Path)},
		// display: contents keeps the body's flex column exactly as the old
		// site laid it out while still marking the main landmark.
		elem.Main(attrs.Props{attrs.Class: "contents"}, body...),
		Footer())

	return elem.Html(attrs.Props{attrs.Lang: "nb"},
		elem.Head(nil, head...),
		elem.Body(nil, elem.Div(attrs.Props{attrs.Class: "antialiased flex flex-col h-screen justify-start"}, kids...)),
	)
}
