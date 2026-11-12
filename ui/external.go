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
)

// The v2 tracker lives at /script.js (/umami.js was v1's and is gone) and
// posts its beacons to /api/send beside itself, so UmamiOrigin covers both
// the script and connect-src.
const umamiScript = UmamiOrigin + "/script.js"

// Umami is the analytics site pages report to. The zero value is off: no
// tracker on the pages, and its origin out of the CSP. The server makes one
// at start and hands it to every page and to CSP, so the two agree.
type Umami struct {
	// SiteID is the website's id on the Umami server.
	SiteID string
	// Domain is the one host the tracker reports from (data-domains): a
	// copy of the pages anywhere else, a local run say, stays silent.
	Domain string
}

// On reports whether pages carry the tracker.
func (u Umami) On() bool { return u.SiteID != "" }

// loadOrigins lists, per kind of load, the outside origins allowed. CSP and
// the tests over rendered pages and built CSS both read it.
func loadOrigins(u Umami) map[string][]string {
	o := map[string][]string{}
	if u.On() {
		o["script-src"] = []string{UmamiOrigin}
		o["connect-src"] = []string{UmamiOrigin}
	}

	return o
}

// CSP is the Content-Security-Policy of every public response. It enforces
// what the tests check: own files only, plus loadOrigins.
func CSP(u Umami) string {
	return csp(loadOrigins(u), false, "'self'")
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

		if len(v) == 0 {
			v = []string{"'none'"}
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
// stylesheet, image, font…) given the CSP directive that governs it and
// the analytics setting. Own URLs (relative, same origin) are always
// allowed.
func ExternalLoadAllowed(u Umami, directive, ref string) bool {
	r, err := url.Parse(ref)
	if err != nil {
		return false
	}

	if r.Scheme == "" && r.Host == "" {
		return true
	}

	return slices.Contains(loadOrigins(u)[directive], r.Scheme+"://"+r.Host)
}
