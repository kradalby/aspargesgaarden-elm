package ui

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/static"
)

// Meta is what a page says about itself; the seo package owns its tags.
type Meta = seo.Meta

// PageProps configures a Page.
type PageProps struct {
	// BaseURL is the public origin, without a trailing slash.
	BaseURL string
	Meta    Meta
	// Menu is the main menu, in order.
	Menu []MenuItem
	// Umami is the analytics site; the zero value loads no tracker.
	Umami Umami
}

// Page is a whole HTML document: head, header, the body nodes, footer and
// analytics. Every public page goes through it.
func Page(p PageProps, body ...elem.Node) *elem.Element {
	head := append([]elem.Node{
		elem.Meta(attrs.Props{attrs.Charset: "utf-8"}),
		elem.Meta(attrs.Props{attrs.Name: "viewport", attrs.Content: "width=device-width, initial-scale=1"}),
	}, seo.Tags(p.BaseURL, p.Meta)...)

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

	head = append(head, elem.Link(attrs.Props{attrs.Rel: "stylesheet", attrs.Href: static.URL("site.css")}))

	if u := p.Umami; u.On() {
		head = append(head, elem.Script(attrs.Props{
			attrs.Src: umamiScript, "data-website-id": u.SiteID, "data-domains": u.Domain, attrs.Defer: "true",
		}))
	}

	// The lightbox is the site's only own script, and loads only where a
	// gallery is, so a page author cannot forget or misplace it.
	if holdsGallery(body) {
		head = append(head, elem.Script(attrs.Props{attrs.Src: static.URL("gallery.js"), attrs.Defer: "true"}))
	}

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
