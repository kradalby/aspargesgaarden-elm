// Package server routes HTTP requests for asparges.
package server

import (
	"io"
	"net/http"
	"strings"

	"github.com/chasefleming/elem-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kradalby/asparges/pages"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/static"
	"github.com/kradalby/asparges/ui"
)

// redirects maps every old address to its new one, permanently. Old links
// and search results must keep working, so rows are never removed.
var redirects = map[string]string{
	"/facilities": "/lokaler",
	"/about":      "/om-oss",
	"/contact":    "/kontakt",
	// Events and gallery add theirs with their pages: /events, /gallery,
	// /christmas, /christmas/{year}.
}

// shortcut: a fixed Julemarked entry keeps today's menu until events carry
// "Vis i menyen"; then the entries come from the database.
var menuEvents = []site.MenuEntry{{Label: "Julemarked", Path: "/julemarked"}}

// Public returns the handler for the local listener. In production nginx
// exposes that listener to the internet, so it must only carry public pages;
// metrics, debug and admin belong on the tsnet listener.
func Public(baseURL string, dev bool) http.Handler {
	site := pages.Site{BaseURL: baseURL, Menu: pages.Menu(menuEvents...)}

	mux := http.NewServeMux()
	mux.Handle("GET "+static.Prefix, static.Handler())
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) { route(site, w, r) })

	// --dev runs without tsnet, so its loopback-only listener stands in for
	// the tailnet one to keep metrics reachable during local work.
	if dev {
		mux.Handle("GET /metrics", promhttp.Handler())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", ui.CSP())
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	})
}

func route(site pages.Site, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if page, ok := pages.Fixed[path]; ok {
		writePage(w, http.StatusOK, page(site))

		return
	}

	// One hop to the final address, also for an old address with a slash.
	// Only known addresses redirect: anything else is a plain 404.
	trimmed := strings.TrimRight(path, "/")

	to, ok := redirects[trimmed]
	if _, page := pages.Fixed[trimmed]; !ok && page && trimmed != path {
		to, ok = trimmed, true
	}

	if ok {
		if r.URL.RawQuery != "" {
			to += "?" + r.URL.RawQuery
		}

		// The path is one of ours (a Fixed key or a redirects value); only the
		// query comes from the request, and it cannot change the host.
		http.Redirect(w, r, to, http.StatusMovedPermanently) //nolint:gosec // see above

		return
	}

	writePage(w, http.StatusNotFound, pages.NotFound(site))
}

func writePage(w http.ResponseWriter, code int, page *elem.Element) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, page.Render())
}
