package seo

import (
	"encoding/xml"
	"fmt"
	"time"
)

// Page is one indexable address and when its content last changed.
type Page struct {
	Path string
	// LastMod is the last edit of what the page shows; zero when unknown
	// (pages whose words are code), which leaves lastmod out rather than
	// claim a date search engines would learn not to trust.
	LastMod time.Time
}

type urlset struct {
	XMLName xml.Name     `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

// Sitemap is sitemap.xml for pages, in their order. No priority or
// changefreq: Google ignores both.
func Sitemap(baseURL string, pages []Page) ([]byte, error) {
	set := urlset{URLs: make([]sitemapURL, len(pages))}
	for i, p := range pages {
		set.URLs[i] = sitemapURL{Loc: baseURL + p.Path}
		if !p.LastMod.IsZero() {
			set.URLs[i].LastMod = p.LastMod.UTC().Format(time.RFC3339)
		}
	}

	b, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("sitemap: %w", err)
	}

	return append([]byte(xml.Header), append(b, '\n')...), nil
}
