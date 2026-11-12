package seo

import (
	"fmt"
	"strings"

	"github.com/kradalby/asparges/booking"
	"github.com/kradalby/asparges/site"
)

// Link is a page as llms.txt lists it.
type Link struct {
	Title, Path string
}

// LLMs is /llms.txt (llmstxt.org): the venue in a few lines of Markdown for
// AI assistants, made from Venue, the main menu, the coming events and, if
// the server reads it, the booking calendar.
func LLMs(baseURL string, pages []Link, upcoming []site.Event, cal booking.Calendar) string {
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

	if cal.On() {
		freeWeekends(&b, cal)
	}

	return b.String()
}

// freeWeekends is the coming year's free weekends by month: what someone
// planning a wedding asks first. Past weekends are no use to anyone, and
// past a year the list would mostly be days nobody has asked for yet.
func freeWeekends(b *strings.Builder, cal booking.Calendar) {
	fmt.Fprintf(b, "\n## Ledige helger\n\nFra bookingkalenderen, %s. En helg er fredag til søndag. "+
		"Ring %s for å bekrefte en dato.\n\n", strings.ToLower(booking.MonthRange(cal.First(), cal.First().MonthStart(booking.MinMonths-1))), Venue.Phone)

	for _, m := range cal.WeekendsAhead(booking.MinMonths) {
		var free []string

		for _, w := range m.Weekends {
			switch status := w.Status(); {
			case w.AllFree():
				free = append(free, w.Dates())
			case status != "Opptatt":
				free = append(free, w.Dates()+" ("+strings.TrimPrefix(status, "Ledig ")+")")
			}
		}

		list := strings.Join(free, ", ")

		switch {
		case m.Folded():
			list = "alle helgene"
		case len(free) == 0:
			list = "ingen"
		}

		fmt.Fprintf(b, "- %s: %s\n", m.Name(), list)
	}
}
