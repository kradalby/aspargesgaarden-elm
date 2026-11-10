package seo

import (
	"cmp"
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/chasefleming/elem-go"
	"github.com/chasefleming/elem-go/attrs"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/static"
)

// The schema.org types a page can describe itself with. Fields follow
// schema.org's names; values come from Venue and the site's own data.

type PostalAddress struct {
	Type            string `json:"@type"`
	StreetAddress   string `json:"streetAddress"`
	PostalCode      string `json:"postalCode"`
	AddressLocality string `json:"addressLocality"`
	AddressRegion   string `json:"addressRegion,omitempty"`
	AddressCountry  string `json:"addressCountry"`
}

type GeoCoordinates struct {
	Type      string  `json:"@type"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Place struct {
	Type                    string          `json:"@type"`
	ID                      string          `json:"@id,omitempty"`
	Name                    string          `json:"name"`
	Address                 *PostalAddress  `json:"address,omitempty"`
	Geo                     *GeoCoordinates `json:"geo,omitempty"`
	MaximumAttendeeCapacity int             `json:"maximumAttendeeCapacity,omitempty"`
}

type Organization struct {
	Type string `json:"@type"`
	ID   string `json:"@id,omitempty"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// LocalBusiness is the venue, typed both LocalBusiness and EventVenue.
type LocalBusiness struct {
	Type          []string       `json:"@type"`
	ID            string         `json:"@id"`
	Name          string         `json:"name"`
	AlternateName string         `json:"alternateName,omitempty"`
	Description   string         `json:"description"`
	URL           string         `json:"url"`
	Telephone     string         `json:"telephone"`
	Logo          string         `json:"logo"`
	Image         []string       `json:"image"`
	Address       PostalAddress  `json:"address"`
	Geo           GeoCoordinates `json:"geo"`
	SameAs        []string       `json:"sameAs"`
	ContainsPlace []Place        `json:"containsPlace"`
}

type WebSite struct {
	Type          string `json:"@type"`
	ID            string `json:"@id"`
	Name          string `json:"name"`
	AlternateName string `json:"alternateName,omitempty"`
	URL           string `json:"url"`
	InLanguage    string `json:"inLanguage"`
	Publisher     Ref    `json:"publisher"`
}

// Ref points at a node described elsewhere on the page.
type Ref struct {
	ID string `json:"@id"`
}

type Event struct {
	Type                string       `json:"@type"`
	Name                string       `json:"name"`
	Description         string       `json:"description,omitempty"`
	URL                 string       `json:"url"`
	StartDate           string       `json:"startDate"`
	EndDate             string       `json:"endDate,omitempty"`
	EventStatus         string       `json:"eventStatus"`
	EventAttendanceMode string       `json:"eventAttendanceMode"`
	Location            Place        `json:"location"`
	Image               []string     `json:"image,omitempty"`
	Organizer           Organization `json:"organizer"`
	Offers              *Offer       `json:"offers,omitempty"`
	IsAccessibleForFree bool         `json:"isAccessibleForFree,omitempty"`
	InLanguage          string       `json:"inLanguage"`
}

// Offer is a way to attend. Price is set only when the price text names one
// amount beyond doubt; the text itself is always the description.
type Offer struct {
	Type          string `json:"@type"`
	URL           string `json:"url,omitempty"`
	Price         string `json:"price,omitempty"`
	PriceCurrency string `json:"priceCurrency,omitempty"`
	Description   string `json:"description,omitempty"`
}

const schemaOrg = "https://schema.org"

type graph struct {
	Context string `json:"@context"`
	Graph   []any  `json:"@graph"`
}

// script is the nodes as one JSON-LD block. json.Marshal escapes <, > and &,
// so no text in the data (an event's "</script>") can end the element; the
// raw node keeps the HTML renderer from escaping the JSON a second time.
func script(nodes []any) elem.Node {
	b, err := json.Marshal(graph{Context: schemaOrg, Graph: nodes})
	if err != nil {
		panic("seo: JSON-LD of this package's own types: " + err.Error())
	}

	return elem.Script(attrs.Props{attrs.Type: "application/ld+json"}, elem.Raw(string(b)))
}

func venueID(baseURL string) string   { return baseURL + "/#venue" }
func websiteID(baseURL string) string { return baseURL + "/#website" }

func address() PostalAddress {
	return PostalAddress{
		Type: "PostalAddress", StreetAddress: Venue.Street, PostalCode: Venue.PostalCode,
		AddressLocality: Venue.PostTown, AddressRegion: Venue.Region, AddressCountry: Venue.Country,
	}
}

func geo() GeoCoordinates {
	return GeoCoordinates{Type: "GeoCoordinates", Latitude: Venue.Latitude, Longitude: Venue.Longitude}
}

// VenueNode is the venue as a business and event venue, for the front and
// contact pages.
func VenueNode(baseURL string) LocalBusiness {
	imgs := []string{baseURL + DefaultImage().URL}
	if v, ok := largestJPEG(static.Photo("forside")); ok {
		imgs = append(imgs, baseURL+v.URL)
	}

	rooms := make([]Place, len(Venue.Rooms))
	for i, r := range Venue.Rooms {
		rooms[i] = Place{Type: "EventVenue", Name: r.Name, MaximumAttendeeCapacity: r.Guests}
	}

	addr, g := address(), geo()

	return LocalBusiness{
		Type:          []string{"LocalBusiness", "EventVenue"},
		ID:            venueID(baseURL),
		Name:          Venue.Name,
		AlternateName: Venue.AlternateName,
		Description:   Venue.Description,
		URL:           baseURL + "/",
		Telephone:     Venue.PhoneE164,
		Logo:          baseURL + static.URL("Aspargesgaarden_HOVEDLOGO_bredde_DempetSort.png"),
		Image:         imgs,
		Address:       addr,
		Geo:           g,
		SameAs:        []string{Venue.Facebook, Venue.Instagram},
		ContainsPlace: rooms,
	}
}

// WebSiteNode names the site, which search engines show beside results.
func WebSiteNode(baseURL string) WebSite {
	return WebSite{
		Type: "WebSite", ID: websiteID(baseURL), Name: Venue.Name, AlternateName: Venue.AlternateName,
		URL: baseURL + "/", InLanguage: "nb", Publisher: Ref{ID: venueID(baseURL)},
	}
}

// EventInput is what an event page shows, as the structured data needs it.
type EventInput struct {
	Event       site.Event
	Description string
	// TicketURL is the ticket link the page shows: none once the event is
	// over or cancelled.
	TicketURL string
}

// EventNodes describes an event page. An event on separate days (a course)
// is one Event per day, as search engines ask, so no day in between looks
// like part of it.
func EventNodes(baseURL string, in EventInput) []any {
	e := in.Event

	// Location and organiser are the venue node of the front and contact
	// pages: the same @id ties them together, and each copy keeps the
	// properties search engines need, since that node is not on this page.
	base := Event{
		Type:                "Event",
		Name:                e.Title,
		Description:         in.Description,
		URL:                 baseURL + e.Path(),
		EventStatus:         schemaOrg + "/EventScheduled",
		EventAttendanceMode: schemaOrg + "/OfflineEventAttendanceMode",
		Location:            Place{Type: "Place", ID: venueID(baseURL), Name: Venue.Name, Address: new(address()), Geo: new(geo())},
		Image:               eventImages(baseURL, e),
		Organizer:           Organization{Type: "Organization", ID: venueID(baseURL), Name: Venue.Name, URL: baseURL + "/"},
		InLanguage:          "nb",
	}

	if e.Cancelled {
		base.EventStatus = schemaOrg + "/EventCancelled"
	}

	base.Offers, base.IsAccessibleForFree = offer(e.PriceText, in.TicketURL)

	if len(e.ExtraDates) == 0 {
		base.StartDate = when(e.Start, e.StartTime)

		switch {
		case e.EndTime != "":
			base.EndDate = when(cmp.Or(e.End, e.Start), e.EndTime)
		case !e.End.IsZero() && e.End != e.Start:
			base.EndDate = when(e.End, "")
		}

		return []any{base}
	}

	days := append([]site.Date{e.Start}, e.ExtraDates...)
	slices.SortFunc(days, site.Date.Compare)

	out := make([]any, len(days))
	for i, d := range days {
		ev := base
		ev.StartDate = when(d, e.StartTime)
		if e.EndTime != "" {
			ev.EndDate = when(d, e.EndTime)
		}
		out[i] = ev
	}

	return out
}

// when is a day and an optional HH:MM in Oslo as ISO 8601: with the offset
// that is in force on that day (+01:00 or +02:00), or the bare date.
func when(d site.Date, clock string) string {
	h, m, ok := strings.Cut(clock, ":")
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)

	if !ok || err1 != nil || err2 != nil {
		return d.String()
	}

	return time.Date(d.Year, d.Month, d.Day, hh, mm, 0, 0, site.Oslo).Format(time.RFC3339)
}

// eventImages are the event's photo in its share, banner and card crops,
// as search engines like several aspect ratios.
func eventImages(baseURL string, e site.Event) []string {
	if e.MainPhoto == nil {
		return nil
	}

	var out []string

	if img, ok := PhotoImage(*e.MainPhoto); ok {
		out = append(out, baseURL+img.URL)
	}

	for _, k := range []images.Kind{images.Banner, images.Card} {
		ref, err := images.RefFor(e.MainPhoto.Image(), k)
		if err != nil {
			continue
		}

		if v, ok := largestJPEG(ref); ok && !slices.Contains(out, baseURL+v.URL) {
			out = append(out, baseURL+v.URL)
		}
	}

	return out
}

var (
	// amountRE is a whole number of kroner, with thousands spaced or dotted.
	amountRE = regexp.MustCompile(`\d+(?:[ .]\d{3})*`)
	// currencyRE marks a number as money: "kr", "NOK" or the trailing ",-".
	currencyRE = regexp.MustCompile(`(?i)\bkr\b|\bnok\b|\d,-`)
	freeRE     = regexp.MustCompile(`(?i)\bgratis\b`)
)

// offer is the event's Offer, if the page shows a price or a ticket link.
// The price text is free-form Norwegian, so it is parsed only when there is
// no doubt: "Gratis inngang" is free; "Kr 1650,- (inkluderer lunsj)" is 1650
// kroner. Anything with more than one number ("Voksne 250,-, barn 100,-",
// "690,- inkl. 1 glass") or a free part ("250,-, barn gratis") has its text
// as the description and no price.
func offer(priceText, ticketURL string) (*Offer, bool) {
	if priceText == "" && ticketURL == "" {
		return nil, false
	}

	o := &Offer{Type: "Offer", URL: ticketURL, Description: priceText}
	amounts := amountRE.FindAllString(priceText, -1)

	switch {
	case len(amounts) == 0 && freeRE.MatchString(priceText):
		o.Price, o.PriceCurrency = "0", "NOK"
		return o, true
	case len(amounts) == 1 && currencyRE.MatchString(priceText) && !freeRE.MatchString(priceText):
		o.Price, o.PriceCurrency = strings.NewReplacer(" ", "", ".", "").Replace(amounts[0]), "NOK"
	}

	return o, false
}
