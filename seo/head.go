package seo

import (
	"strconv"
	"strings"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/static"
)

// Meta is what a page says about itself to browsers, search engines, link
// previews and assistants.
type Meta struct {
	// Title is the page's own title: what it is and where. <title> adds the
	// site name after it.
	Title       string
	Description string
	// Path is the canonical path, e.g. "/lokaler". Empty for pages that have
	// no address of their own (404), which then get no canonical link.
	Path string
	// Image is the link preview; the zero Image means DefaultImage.
	Image Image
	// JSONLD are the page's schema.org nodes, typed values from this
	// package.
	JSONLD []any
	// NoIndex is the beta site (--noindex), whose response header already
	// keeps it out of search engines; it gets no robots tag.
	NoIndex bool
}

// Image is a share image: a JPEG every scraper can read.
type Image struct {
	// URL is site-relative (/static/… or /img/…); tags make it absolute.
	URL           string
	Width, Height int
	Alt           string
}

// DefaultImage is the link preview of pages without a photo of their own.
func DefaultImage() Image {
	return Image{
		URL: static.URL("share.jpg"), Width: 1200, Height: 630,
		Alt: "Aspargesgårdens logo over pyntede, runde bord i låven",
	}
}

// PhotoImage is p as a share image: its share crop, or for a photo too small
// for one the largest JPEG of its banner crop, or of the whole photo. ok is
// false for a photo with no JPEG at all.
func PhotoImage(p site.Photo) (img Image, ok bool) {
	for _, k := range []images.Kind{images.Share, images.Banner, images.Full} {
		ref, err := images.RefFor(p.Image(), k)
		if err != nil {
			continue
		}

		if v, ok := largestJPEG(ref); ok {
			return Image{URL: v.URL, Width: v.Width, Height: ref.Height * v.Width / ref.Width, Alt: p.Alt}, true
		}
	}

	return Image{}, false
}

func largestJPEG(ref images.Ref) (images.Variant, bool) {
	var best images.Variant
	for _, v := range ref.Variants {
		if v.Format == images.JPEG && v.Width > best.Width {
			best = v
		}
	}

	return best, best.Width > 0
}

// FullTitle is the page's <title>.
func (m Meta) FullTitle() string { return m.Title + " | " + Venue.Name }

// Tags are the page's <head> tags about itself, for ui.Page to place.
func Tags(baseURL string, m Meta) []elem.Node {
	img := m.Image
	if img.URL == "" {
		img = DefaultImage()
	}

	tags := []elem.Node{
		elem.Title(nil, elem.Text(m.FullTitle())),
		named("description", m.Description),
	}

	// Google shows large previews (Discover, big thumbnails) only when a
	// page allows them.
	if !m.NoIndex {
		tags = append(tags, named("robots", "max-image-preview:large"))
	}

	if m.Path != "" {
		tags = append(tags, elem.Link(attrs.Props{attrs.Rel: "canonical", attrs.Href: baseURL + m.Path}))
	}

	tags = append(tags,
		property("og:type", "website"),
		property("og:site_name", Venue.Name),
		property("og:locale", "nb_NO"),
		property("og:title", m.Title),
		property("og:description", m.Description),
	)

	if m.Path != "" {
		tags = append(tags, property("og:url", baseURL+m.Path))
	}

	tags = append(tags,
		property("og:image", baseURL+img.URL),
		property("og:image:type", "image/jpeg"),
		property("og:image:width", strconv.Itoa(img.Width)),
		property("og:image:height", strconv.Itoa(img.Height)),
		property("og:image:alt", img.Alt),
		named("twitter:card", "summary_large_image"),
	)

	if len(m.JSONLD) > 0 {
		tags = append(tags, script(m.JSONLD))
	}

	return tags
}

// Clip shortens plain text to at most n runes, at a word, marking the cut.
func Clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}

	cut := string(r[:n-1])
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}

	return strings.TrimRight(cut, " ,.;:–-") + "…"
}

func named(name, content string) elem.Node {
	return elem.Meta(attrs.Props{attrs.Name: name, attrs.Content: content})
}

func property(p, content string) elem.Node {
	return elem.Meta(attrs.Props{"property": p, attrs.Content: content})
}
