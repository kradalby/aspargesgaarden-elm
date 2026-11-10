package ui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSP(t *testing.T) {
	require.Equal(t,
		"default-src 'self'; script-src 'self' https://umami.kradalby.no; connect-src 'self' https://umami.kradalby.no; "+
			"frame-src https://calendar.google.com; img-src 'self'; font-src 'self'; style-src 'self'; object-src 'none'; "+
			"base-uri 'self'; form-action 'self'; frame-ancestors 'self'",
		CSP())
}

func TestAdminCSP(t *testing.T) {
	// The policy docs/admin-api.md gives the admin app, which its browser
	// tests run under.
	require.Equal(t,
		"default-src 'self'; script-src 'self'; connect-src 'self'; frame-src 'self'; img-src 'self'; "+
			"font-src 'self'; style-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
		AdminCSP())
}

func TestExternalLoadAllowed(t *testing.T) {
	tests := []struct {
		directive, ref string
		want           bool
	}{
		{"script-src", "/static/x.js?v=1", true},
		{"img-src", "/static/x.png", true},
		{"script-src", "https://umami.kradalby.no/script.js", true},
		{"frame-src", "https://calendar.google.com/calendar/embed?src=x", true},
		{"script-src", "https://calendar.google.com/x.js", false},
		{"font-src", "https://fonts.gstatic.com/x.woff2", false},
		{"style-src", "https://unpkg.com/x.css", false},
		{"script-src", "//umami.kradalby.no/script.js", false},
		{"script-src", "http://umami.kradalby.no/script.js", false},
		{"img-src", "data:image/png;base64,AAAA", false},
	}

	for _, tt := range tests {
		t.Run(tt.directive+" "+tt.ref, func(t *testing.T) {
			require.Equal(t, tt.want, ExternalLoadAllowed(tt.directive, tt.ref))
		})
	}
}
