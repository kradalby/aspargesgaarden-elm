package server

import (
	"encoding/xml"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/static"
)

type sitemapDoc struct {
	URLs []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
	} `xml:"url"`
}

func (f *fixture) sitemap(t *testing.T) sitemapDoc {
	t.Helper()

	rec := f.get(t, "/sitemap.xml")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/xml; charset=utf-8", rec.Header().Get("Content-Type"))

	var doc sitemapDoc
	require.NoError(t, xml.Unmarshal(rec.Body.Bytes(), &doc))

	return doc
}

// Every address in the sitemap is a page that answers 200 and calls itself
// by that address; drafts are left out, past events kept.
func TestSitemapListsThePagesThatAnswer(t *testing.T) {
	f := withMarkets(t)

	market, err := f.store.NewestPublishedEvent(t.Context(), "julemarked")
	require.NoError(t, err)

	f.clock.set(oslo("2026-11-10 13:30"))
	require.NoError(t, f.store.SetGallery(t.Context(), []int64{market.MainPhoto.ID}))

	urls := f.sitemap(t).URLs
	paths := make([]string, 0, len(urls))
	lastmod := map[string]string{}

	for _, u := range urls {
		path := strings.TrimPrefix(u.Loc, "https://aspargesgaarden.no")
		paths = append(paths, path)
		lastmod[path] = u.LastMod

		rec := f.get(t, path)
		require.Equal(t, http.StatusOK, rec.Code, path)
		require.Contains(t, rec.Body.String(), `<link href="`+u.Loc+`" rel="canonical">`, path)
	}

	require.Equal(t, []string{
		"/", "/kontakt", "/lokaler", "/om-oss", "/arrangementer", "/galleri",
		"/arrangementer/2025/julemarked", "/arrangementer/2026/julemarked",
	}, paths)

	require.Empty(t, lastmod["/lokaler"], "words in code have no date we can vouch for")
	require.Equal(t, "2026-11-10T12:30:00Z", lastmod["/galleri"])
	require.Equal(t, "2026-11-10T11:00:00Z", lastmod["/arrangementer/2026/julemarked"])
	require.Equal(t, lastmod["/arrangementer/2026/julemarked"], lastmod["/arrangementer"])
}

// An edit moves the event's lastmod, and with it the list's.
func TestSitemapLastmodFollowsEdits(t *testing.T) {
	f := withMarkets(t)

	e, err := f.store.NewestPublishedEvent(t.Context(), "julemarked")
	require.NoError(t, err)

	f.clock.set(oslo("2026-11-11 09:15"))
	e.Summary = "Nå med flere utstillere."
	_, err = f.store.UpdateEvent(t.Context(), e)
	require.NoError(t, err)

	for _, u := range f.sitemap(t).URLs {
		if strings.HasSuffix(u.Loc, "/arrangementer/2026/julemarked") || strings.HasSuffix(u.Loc, "/arrangementer") {
			require.Equal(t, "2026-11-11T08:15:00Z", u.LastMod, u.Loc)
		}
	}
}

func TestRobots(t *testing.T) {
	rec := withMarkets(t).get(t, "/robots.txt")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), "User-agent: *\nAllow: /\nDisallow: /admin\n")
	require.Contains(t, rec.Body.String(), "Sitemap: https://aspargesgaarden.no/sitemap.xml\n")
	require.Empty(t, rec.Header().Get("X-Robots-Tag"))
}

// The beta site: nothing may be indexed, but the files stay readable.
func TestNoIndex(t *testing.T) {
	f := newFixture(t, false, func(c *Config) { c.NoIndex = true })
	f.publish(t, site.Event{Title: "Julemarked", Start: date("2026-11-14")})

	for _, target := range []string{"/", "/arrangementer/2026/julemarked", "/finnes-ikke", "/facilities", "/robots.txt", "/sitemap.xml", "/llms.txt", "/img/x.jpg", static.URL("site.css")} {
		rec := f.get(t, target)
		require.Equal(t, "noindex", rec.Header().Get("X-Robots-Tag"), target)
	}

	robots := f.get(t, "/robots.txt").Body.String()
	require.Contains(t, robots, "User-agent: *\nDisallow: /\n")
	require.NotContains(t, robots, "Allow")

	require.Len(t, f.sitemap(t).URLs, 7, "the sitemap is still served")
}

func TestLLMsTxt(t *testing.T) {
	rec := withMarkets(t).get(t, "/llms.txt")

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.True(t, strings.HasPrefix(body, "# Aspargesgården\n"))
	require.Contains(t, body, "- [Muligheter](https://aspargesgaarden.no/lokaler)\n")
	require.Contains(t, body, "- [Julemarked](https://aspargesgaarden.no/arrangementer/2026/julemarked): 14. november 2026 - 15. november 2026\n")
	require.NotContains(t, body, "/arrangementer/2025/", "only what is coming")
	require.NotContains(t, body, "Kladd", "drafts are not public")
}
