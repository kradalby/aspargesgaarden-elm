// Package seo is how aspargesgaarden.no describes itself to search engines
// and AI assistants: the venue's facts, the <head> tags every page carries
// (title, description, canonical, Open Graph, JSON-LD), sitemap.xml,
// robots.txt, llms.txt, IndexNow pings and the crawler metrics.
//
// Everything here is generated from the same values the visible pages show,
// so what a crawler reads cannot drift from what a visitor sees.
package seo

import "fmt"

// Room is a space guests can rent, with the most guests it seats.
type Room struct {
	Name   string
	Guests int
}

// Facts is what is true about the venue. The pages that state a fact and
// the structured data both read it from Venue.
type Facts struct {
	Name string
	// AlternateName is the spelling in the domain, which people also type.
	AlternateName string
	Description   string
	// Phone is how the site writes the number; PhoneE164 is the same number
	// for machines.
	Phone, PhoneE164 string
	Street           string
	PostalCode       string
	PostTown         string // the postal place, what the address line says
	Municipality     string
	Area             string // what people call the place
	Region           string
	Country          string // ISO 3166-1
	// Latitude and Longitude are the address point in Kartverket's address
	// register (WGS84 / EUREF89).
	Latitude, Longitude float64
	Facebook, Instagram string
	Rooms               []Room
}

// Venue is Aspargesgården. No prices: the owner does not publish them.
var Venue = Facts{
	Name:          "Aspargesgården",
	AlternateName: "Aspargesgaarden",
	Description: "Selskapslokaler i låve og fjøs på en gård i Tjølling, Larvik, ved kysten i Vestfold. " +
		"Låvebryllup er spesialiteten; gården tar også imot konfirmasjoner, dåp, minnestunder, " +
		"bursdager, firmasamlinger, kurs, konserter og konferanser.",
	Phone:        "+47 90 91 98 12",
	PhoneE164:    "+4790919812",
	Street:       "Østbyveien 75",
	PostalCode:   "3280",
	PostTown:     "Tjodalyng",
	Municipality: "Larvik",
	Area:         "Tjølling",
	Region:       "Vestfold",
	Country:      "NO",
	Latitude:     59.04720,
	Longitude:    10.12467,
	Facebook:     "https://facebook.com/aspargesgaarden",
	Instagram:    "https://instagram.com/aspargesgaarden",
	Rooms: []Room{
		{Name: "Låven", Guests: 100},
		{Name: "Fjøset", Guests: 68},
	},
}

// AddressLine is the address as the contact page writes it.
func (f Facts) AddressLine() string {
	return fmt.Sprintf("%s, %s %s", f.Street, f.PostalCode, f.PostTown)
}

// Guests is how many the named room seats; an unknown room is a programming
// error, since rooms are constants.
func (f Facts) Guests(room string) int {
	for _, r := range f.Rooms {
		if r.Name == room {
			return r.Guests
		}
	}

	panic("seo: no room " + room)
}
