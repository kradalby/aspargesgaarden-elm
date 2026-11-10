package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublic(t *testing.T) {
	tests := []struct {
		name     string
		dev      bool
		path     string
		wantCode int
		wantBody string
	}{
		{name: "front", path: "/", wantCode: http.StatusOK, wantBody: "<h1>Aspargesgården</h1>"},
		{name: "front-dev", dev: true, path: "/", wantCode: http.StatusOK, wantBody: "<h1>Aspargesgården</h1>"},
		{name: "unknown", path: "/finnes-ikke", wantCode: http.StatusNotFound},
		// The public listener is the one nginx exposes: no metrics, debug or
		// admin may ever answer there outside --dev.
		{name: "metrics", path: "/metrics", wantCode: http.StatusNotFound},
		{name: "debug", path: "/debug/", wantCode: http.StatusNotFound},
		{name: "admin", path: "/admin/", wantCode: http.StatusNotFound},
		{name: "metrics-dev", dev: true, path: "/metrics", wantCode: http.StatusOK, wantBody: "go_goroutines"},
		{name: "debug-dev", dev: true, path: "/debug/", wantCode: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Public("https://aspargesgaarden.no", tt.dev).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil))

			require.Equal(t, tt.wantCode, rec.Code)
			require.Contains(t, rec.Body.String(), tt.wantBody)
		})
	}
}
