package seo

// Robots is robots.txt. Search engines and AI crawlers are all welcome, the
// ones that train models too: the venue wants to be found by people asking
// assistants. One group for every crawler, because a crawler with a group
// of its own would skip this one and with it the admin rule. noindex (the
// beta site) shuts every crawler out.
func Robots(baseURL string, noindex bool) string {
	rules := "Allow: /\nDisallow: /admin\n"
	if noindex {
		rules = "Disallow: /\n"
	}

	return "User-agent: *\n" + rules + "\nSitemap: " + baseURL + "/sitemap.xml\n"
}
