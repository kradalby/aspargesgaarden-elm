package pages

import (
	"net/http"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

// Admin is the document the admin app (elm/admin) runs in: the admin
// stylesheet, the site's fonts and the app's script, and an empty body the
// app takes over. The app reads the address itself, so one document serves
// every admin address.
func Admin() *elem.Element {
	fonts := static.FontURLs()
	head := make([]elem.Node, 0, 8+len(fonts))
	head = append(head,
		elem.Meta(attrs.Props{attrs.Charset: "utf-8"}),
		elem.Meta(attrs.Props{attrs.Name: "viewport", attrs.Content: "width=device-width, initial-scale=1"}),
		elem.Meta(attrs.Props{attrs.Name: "robots", attrs.Content: "noindex, nofollow"}),
		elem.Title(nil, elem.Text("Aspargesgården admin")),
		elem.Link(attrs.Props{attrs.Rel: "icon", attrs.Href: static.URL("favicon.ico"), attrs.Sizes: "any"}),
		elem.Link(attrs.Props{attrs.Rel: "icon", attrs.Href: static.URL("favicon.svg"), attrs.Type: "image/svg+xml"}),
	)

	for _, u := range fonts {
		head = append(head, elem.Link(attrs.Props{
			attrs.Rel:         "preload",
			attrs.Href:        u,
			"as":              "font",
			attrs.Type:        "font/woff2",
			attrs.Crossorigin: "anonymous",
		}))
	}

	head = append(head,
		elem.Link(attrs.Props{attrs.Rel: "stylesheet", attrs.Href: static.URL("admin.css")}),
		elem.Script(attrs.Props{attrs.Src: static.URL("admin.js"), attrs.Defer: "true"}),
	)

	return elem.Html(attrs.Props{attrs.Lang: "nb"},
		elem.Head(nil, head...),
		elem.Body(nil, elem.NoScript(nil, elem.Text("Administrasjonen trenger JavaScript. Slå det på i nettleseren og last siden på nytt."))),
	)
}

// AdminHandler answers GET and HEAD of any path with Admin, under the admin
// CSP, and refuses other methods. Mount it as "/admin/" (no method: a
// method in the pattern would conflict with a method-less "/admin/api/") on
// the tailnet listener, so deep links such as /admin/arrangementer/12 load
// the same document. The more specific /admin/api/ (the JSON API) and
// /static/ (the files Admin names) are separate handlers on that listener.
func AdminHandler() http.Handler {
	// The assets are fixed at build time, so the document is too.
	page := []byte(Admin().Render())

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

			return
		}

		h := w.Header()
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Content-Security-Policy", ui.AdminCSP())
		h.Set("X-Content-Type-Options", "nosniff")
		// Revalidate, so a new deploy's asset hashes reach an open tab's
		// next load.
		h.Set("Cache-Control", "no-cache")
		_, _ = w.Write(page)
	})
}
