package seo

import (
	"encoding/xml"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSitemap(t *testing.T) {
	got, err := Sitemap(testBase, []Page{
		{Path: "/"},
		{Path: "/arrangementer/2026/julemarked", LastMod: time.Date(2026, 10, 10, 9, 30, 0, 0, time.FixedZone("CEST", 2*3600))},
	})
	require.NoError(t, err)

	require.Equal(t, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://aspargesgaarden.no/</loc>
  </url>
  <url>
    <loc>https://aspargesgaarden.no/arrangementer/2026/julemarked</loc>
    <lastmod>2026-10-10T07:30:00Z</lastmod>
  </url>
</urlset>
`, string(got))

	var back urlset
	require.NoError(t, xml.Unmarshal(got, &back))
	require.Len(t, back.URLs, 2)
}

// Addresses are XML-escaped, should one ever carry a query.
func TestSitemapEscapes(t *testing.T) {
	got, err := Sitemap(testBase, []Page{{Path: "/a?b=1&c=2"}})
	require.NoError(t, err)
	require.Contains(t, string(got), "<loc>https://aspargesgaarden.no/a?b=1&amp;c=2</loc>")
}
