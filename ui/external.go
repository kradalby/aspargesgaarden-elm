package ui

import (
	"net/url"
	"slices"
	"strings"
)

// Everything the site needs is built into the binary; an outside service
// that goes away must not break a page. These are the only origins a page
// may load anything from, each a service by nature and optional: pages work
// without them.
const (
	// UmamiOrigin serves the analytics script and receives its beacons.
	UmamiOrigin = "https://umami.kradalby.no"
	// CalendarOrigin serves the booking calendar framed on the contact page.
	CalendarOrigin = "https://calendar.google.com"
)

// The Umami site this site reports to.
const (
	umamiScript = UmamiOrigin + "/umami.js"
	umamiSiteID = "168bd9f6-fec6-40dc-95b5-b0e4082c7a98"
)

// loadOrigins lists, per kind of load, the outside origins allowed. CSP and
// the tests over rendered pages and built CSS both read it.
var loadOrigins = map[string][]string{
	"script-src":  {UmamiOrigin},
	"connect-src": {UmamiOrigin},
	"frame-src":   {CalendarOrigin},
}

// CSP is the Content-Security-Policy of every public response. It enforces
// what the tests check: own files only, plus loadOrigins.
func CSP() string {
	directives := []string{
		"default-src 'self'",
		"script-src 'self' " + strings.Join(loadOrigins["script-src"], " "),
		"connect-src 'self' " + strings.Join(loadOrigins["connect-src"], " "),
		"frame-src " + strings.Join(loadOrigins["frame-src"], " "),
		"img-src 'self'",
		"font-src 'self'",
		"style-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'self'",
	}

	return strings.Join(directives, "; ")
}

// ExternalLoadAllowed reports whether a page may load ref (a script, frame,
// stylesheet, image, font…) given the CSP directive that governs it. Own
// URLs (relative, same origin) are always allowed.
func ExternalLoadAllowed(directive, ref string) bool {
	u, err := url.Parse(ref)
	if err != nil {
		return false
	}

	if u.Scheme == "" && u.Host == "" {
		return true
	}

	return slices.Contains(loadOrigins[directive], u.Scheme+"://"+u.Host)
}
