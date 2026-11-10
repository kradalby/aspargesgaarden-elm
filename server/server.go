// Package server routes HTTP requests for asparges.
package server

import (
	"io"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kradalby/asparges/pages"
)

// Public returns the handler for the local listener. In production nginx
// exposes that listener to the internet, so it must only carry public pages;
// metrics, debug and admin belong on the tsnet listener.
func Public(baseURL string, dev bool) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, pages.Front(baseURL).Render())
	})

	// --dev runs without tsnet, so its loopback-only listener stands in for
	// the tailnet one to keep metrics reachable during local work.
	if dev {
		mux.Handle("GET /metrics", promhttp.Handler())
	}

	return mux
}
