package seo

import (
	"fmt"
	"strings"

	"github.com/kradalby/asparges/site"
)

// Link is a page as llms.txt lists it.
type Link struct {
	Title, Path string
}

// LLMs is /llms.txt (llmstxt.org): the venue in a few lines of Markdown for
// AI assistants, made from Venue, the main menu and the coming events.
func LLMs(baseURL string, pages []Link, upcoming []site.Event) string {
	var b strings.Builder

	v := Venue
	fmt.Fprintf(&b, "# %s\n\n> %s\n\n", v.Name, v.Description)
	fmt.Fprintf(&b, "- Adresse: %s (%s kommune, %s)\n", v.AddressLine(), v.Municipality, v.Region)
	fmt.Fprintf(&b, "- Telefon: %s\n", v.Phone)
	fmt.Fprintf(&b, "- Nettside: %s/\n", baseURL)
	fmt.Fprintf(&b, "- Facebook: %s\n- Instagram: %s\n", v.Facebook, v.Instagram)

	b.WriteString("\n## Lokaler\n\n")
	for _, r := range v.Rooms {
		fmt.Fprintf(&b, "- %s: plass til %d gjester\n", r.Name, r.Guests)
	}

	b.WriteString("\n## Sider\n\n")
	for _, p := range pages {
		fmt.Fprintf(&b, "- [%s](%s%s)\n", p.Title, baseURL, p.Path)
	}

	b.WriteString("\n## Kommende arrangementer\n\n")
	if len(upcoming) == 0 {
		b.WriteString("Ingen kommende arrangementer for øyeblikket.\n")
	}

	for _, e := range upcoming {
		line := strings.TrimSuffix(e.DateLine(), " på "+v.Name)
		if e.Cancelled {
			line += " (avlyst)"
		}

		fmt.Fprintf(&b, "- [%s](%s%s): %s\n", e.Title, baseURL, e.Path(), line)
	}

	return b.String()
}
