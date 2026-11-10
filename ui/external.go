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

// The Umami site this site reports to. The v2 tracker lives at /script.js
// (/umami.js was v1's and is gone) and posts its beacons to /api/send
// beside itself, so UmamiOrigin covers both the script and connect-src.
const (
	umamiScript = UmamiOrigin + "/script.js"
	umamiSiteID = "168bd9f6-fec6-40dc-95b5-b0e4082c7a98"
	// umamiDomains limits reporting to the live site: beta and local runs
	// show the same pages, and must not count as visits.
	umamiDomains = "aspargesgaarden.no"
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
	return csp(loadOrigins, false, "'self'")
}

// AdminCSP is the policy of the admin app and its API: own files only, no
// outside origin at all, own frames for the page preview, and nothing may
// frame the admin, rebase it or post a form from it. The preview frame
// inherits it, so a public page must render under it too, which own files
// only (the public pages' rule) guarantees.
func AdminCSP() string {
	return csp(nil, true, "'none'")
}

// csp builds a policy from the outside origins per directive. Frames are
// only what origins lists unless ownFrames; lock fills base-uri,
// form-action and frame-ancestors.
func csp(origins map[string][]string, ownFrames bool, lock string) string {
	src := func(directive string, self bool) string {
		v := origins[directive]
		if self {
			v = append([]string{"'self'"}, v...)
		}

		return directive + " " + strings.Join(v, " ")
	}

	directives := []string{
		"default-src 'self'",
		src("script-src", true),
		src("connect-src", true),
		src("frame-src", ownFrames),
		"img-src 'self'",
		"font-src 'self'",
		"style-src 'self'",
		"object-src 'none'",
		"base-uri " + lock,
		"form-action " + lock,
		"frame-ancestors " + lock,
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
