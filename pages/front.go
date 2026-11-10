// Package pages builds the public pages of aspargesgaarden.no as elem-go
// trees. HTML is only ever produced through elem-go, never a template
// language, so every value is escaped by construction.
package pages

import (
	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"
)

// Front is a stand-in until the component kit replaces it with the real
// front page.
func Front(baseURL string) *elem.Element {
	return elem.Html(attrs.Props{attrs.Lang: "nb"},
		elem.Head(nil,
			elem.Meta(attrs.Props{attrs.Charset: "utf-8"}),
			elem.Meta(attrs.Props{attrs.Name: "viewport", attrs.Content: "width=device-width, initial-scale=1"}),
			elem.Title(nil, elem.Text("Aspargesgården")),
			elem.Link(attrs.Props{attrs.Rel: "canonical", attrs.Href: baseURL + "/"}),
		),
		elem.Body(nil,
			elem.H1(nil, elem.Text("Aspargesgården")),
		),
	)
}
