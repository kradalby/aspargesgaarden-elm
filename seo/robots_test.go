package seo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRobots(t *testing.T) {
	require.Equal(t, `User-agent: *
Allow: /
Disallow: /admin

Sitemap: https://aspargesgaarden.no/sitemap.xml
`, Robots(testBase, false))
}

func TestRobotsNoindex(t *testing.T) {
	require.Equal(t, `User-agent: *
Disallow: /

Sitemap: https://beta.aspargesgaarden.no/sitemap.xml
`, Robots("https://beta.aspargesgaarden.no", true))
}
