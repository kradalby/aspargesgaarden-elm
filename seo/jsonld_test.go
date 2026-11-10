package seo

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/kradalby/asparges/site"
)

func date(s string) site.Date {
	d, err := site.ParseDate(s)
	if err != nil {
		panic(err)
	}

	return d
}

// required lists, per schema.org type, the properties search engines need
// before they use a node (Google's required ones, and ours where we promise
// more). checkNodes walks every node, nested ones too.
var required = map[string][]string{
	"Event":          {"name", "url", "startDate", "eventStatus", "eventAttendanceMode", "location", "organizer"},
	"Place":          {"name", "address", "geo"},
	"PostalAddress":  {"streetAddress", "postalCode", "addressLocality", "addressCountry"},
	"GeoCoordinates": {"latitude", "longitude"},
	"LocalBusiness":  {"@id", "name", "url", "telephone", "logo", "address", "geo", "image", "sameAs"},
	"EventVenue":     {"name", "maximumAttendeeCapacity"},
	"WebSite":        {"@id", "name", "url"},
	"Organization":   {"name", "url"},
	"Offer":          {},
}

func checkNodes(t *testing.T, nodes []any) {
	t.Helper()

	b, err := json.Marshal(graph{Context: schemaOrg, Graph: nodes})
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(b, &doc))
	require.Equal(t, "https://schema.org", doc["@context"])

	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case []any:
			for _, x := range v {
				walk(x)
			}
		case map[string]any:
			checkNode(t, v)

			for _, x := range v {
				walk(x)
			}
		}
	}
	walk(doc["@graph"])
}

func checkNode(t *testing.T, n map[string]any) {
	t.Helper()

	types := make([]string, 0, 2)
	switch ty := n["@type"].(type) {
	case string:
		types = append(types, ty)
	case []any:
		for _, x := range ty {
			types = append(types, x.(string)) //nolint:forcetypeassert // fails the test on a non-string
		}
	}

	if _, ref := n["@id"]; len(types) == 0 && ref && len(n) == 1 {
		return // a reference to a node described elsewhere
	}

	require.NotEmpty(t, types, "node without @type: %v", n)

	// The venue node is LocalBusiness and EventVenue at once; capacity is
	// stated per room, so the room rule does not apply to it.
	isBusiness := len(types) > 1

	for _, ty := range types {
		props, known := required[ty]
		require.True(t, known, "unexpected type %s", ty)

		if ty == "EventVenue" && isBusiness {
			continue
		}

		for _, p := range props {
			require.NotEmpty(t, n[p], "%s needs %s: %v", ty, p, n)
		}
	}

	if _, ok := n["price"]; ok {
		require.Equal(t, "NOK", n["priceCurrency"], "a price needs its currency")
	}

	if start, ok := n["startDate"].(string); ok {
		end, _ := n["endDate"].(string)
		checkDates(t, start, end)
	}
}

// checkDates: both are ISO 8601, a time always has an offset, and the event
// does not end before it starts.
func checkDates(t *testing.T, start, end string) {
	t.Helper()

	parse := func(s string) time.Time {
		if len(s) == len("2006-01-02") {
			d, err := time.Parse(time.DateOnly, s)
			require.NoError(t, err)

			return d
		}

		d, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err, "%s must be a date or a time with offset", s)

		return d
	}

	s := parse(start)
	if end != "" {
		require.False(t, parse(end).Before(s), "%s ends before %s", end, start)
	}
}

func testEvent() site.Event {
	p := testPhoto('a', 2400, 1600)

	return site.Event{
		ID: 1, Year: 2026, Slug: "julemarked", Title: "Julemarked på Aspargesgården",
		Start: date("2026-11-14"), StartTime: "12:00", End: date("2026-11-15"), EndTime: "16:00",
		PriceText: "Gratis inngang", Status: site.Published, MainPhoto: &p,
	}
}

func TestEventNodes(t *testing.T) {
	nodes := EventNodes(testBase, EventInput{Event: testEvent(), Description: "Mer enn 30 utstillere.", TicketURL: "https://fabnite.com/julemarked"})
	checkNodes(t, nodes)

	require.Len(t, nodes, 1)
	ev := nodes[0].(Event) //nolint:forcetypeassert // fails the test otherwise

	require.Equal(t, "2026-11-14T12:00:00+01:00", ev.StartDate)
	require.Equal(t, "2026-11-15T16:00:00+01:00", ev.EndDate)
	require.Equal(t, "https://schema.org/EventScheduled", ev.EventStatus)
	require.Equal(t, "https://aspargesgaarden.no/arrangementer/2026/julemarked", ev.URL)
	require.Equal(t, &Offer{Type: "Offer", URL: "https://fabnite.com/julemarked", Price: "0", PriceCurrency: "NOK", Description: "Gratis inngang"}, ev.Offers)
	require.True(t, ev.IsAccessibleForFree)
	require.Len(t, ev.Image, 3, "share, banner and card")
	require.Contains(t, ev.Image[0], "/share-")
	require.Equal(t, "Østbyveien 75", ev.Location.Address.StreetAddress)

	// One graph across pages: the event's place and organiser are the venue.
	venue := VenueNode(testBase).ID
	require.Equal(t, venue, ev.Location.ID)
	require.Equal(t, venue, ev.Organizer.ID)
}

// The offset is the one in force on the day: summer time in May, winter
// time in November, and the night the clocks change.
func TestEventDatesFollowOsloTime(t *testing.T) {
	tests := []struct {
		day, clock, want string
	}{
		{"2026-05-19", "18:30", "2026-05-19T18:30:00+02:00"},
		{"2026-11-14", "12:00", "2026-11-14T12:00:00+01:00"},
		{"2026-03-29", "01:30", "2026-03-29T01:30:00+01:00"},
		{"2026-03-29", "03:30", "2026-03-29T03:30:00+02:00"},
		{"2026-10-25", "04:00", "2026-10-25T04:00:00+01:00"},
		{"2026-10-24", "23:00", "2026-10-24T23:00:00+02:00"},
		{"2026-10-24", "", "2026-10-24"},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, when(date(tt.day), tt.clock), "%s %s", tt.day, tt.clock)
	}
}

func TestEventEnd(t *testing.T) {
	tests := []struct {
		name      string
		edit      func(*site.Event)
		wantStart string
		wantEnd   string
	}{
		{
			name: "one evening", edit: func(e *site.Event) { e.End, e.EndTime = site.Date{}, "22:00" },
			wantStart: "2026-11-14T12:00:00+01:00", wantEnd: "2026-11-14T22:00:00+01:00",
		},
		// An unknown end is left out rather than guessed.
		{
			name: "no end", edit: func(e *site.Event) { e.End, e.EndTime = site.Date{}, "" },
			wantStart: "2026-11-14T12:00:00+01:00",
		},
		{
			name: "days without times", edit: func(e *site.Event) { e.StartTime, e.EndTime = "", "" },
			wantStart: "2026-11-14", wantEnd: "2026-11-15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := testEvent()
			tt.edit(&e)

			nodes := EventNodes(testBase, EventInput{Event: e})
			checkNodes(t, nodes)

			ev := nodes[0].(Event) //nolint:forcetypeassert // fails the test otherwise
			require.Equal(t, tt.wantStart, ev.StartDate)
			require.Equal(t, tt.wantEnd, ev.EndDate)
		})
	}
}

// A course on separate days is one Event per day, not one long run.
func TestEventOnSeparateDays(t *testing.T) {
	e := testEvent()
	e.Start, e.End, e.StartTime, e.EndTime = date("2026-10-31"), site.Date{}, "10:00", "15:00"
	e.ExtraDates = []site.Date{date("2026-11-28"), date("2026-11-29")}

	nodes := EventNodes(testBase, EventInput{Event: e})
	checkNodes(t, nodes)

	got := make([][2]string, 0, len(nodes))
	for _, n := range nodes {
		ev := n.(Event) //nolint:forcetypeassert // fails the test otherwise
		got = append(got, [2]string{ev.StartDate, ev.EndDate})
	}

	require.Empty(t, cmp.Diff([][2]string{
		{"2026-10-31T10:00:00+01:00", "2026-10-31T15:00:00+01:00"},
		{"2026-11-28T10:00:00+01:00", "2026-11-28T15:00:00+01:00"},
		{"2026-11-29T10:00:00+01:00", "2026-11-29T15:00:00+01:00"},
	}, got))
}

func TestCancelledEvent(t *testing.T) {
	e := testEvent()
	e.Cancelled = true

	ev := EventNodes(testBase, EventInput{Event: e})[0].(Event) //nolint:forcetypeassert // fails the test otherwise
	require.Equal(t, "https://schema.org/EventCancelled", ev.EventStatus)
}

func TestOffer(t *testing.T) {
	tests := []struct {
		price, ticket string
		want          *Offer
		free          bool
	}{
		{price: "", ticket: "", want: nil},
		{price: "", ticket: "https://fabnite.com/x", want: &Offer{Type: "Offer", URL: "https://fabnite.com/x"}},
		{price: "Gratis inngang", want: &Offer{Type: "Offer", Price: "0", PriceCurrency: "NOK", Description: "Gratis inngang"}, free: true},
		{price: "250,-", want: &Offer{Type: "Offer", Price: "250", PriceCurrency: "NOK", Description: "250,-"}},
		{price: "Kr 1650,- (inkluderer råvarer til baking og enkel lunsj).", want: &Offer{Type: "Offer", Price: "1650", PriceCurrency: "NOK", Description: "Kr 1650,- (inkluderer råvarer til baking og enkel lunsj)."}},
		{price: "kr 1 250 per person", want: &Offer{Type: "Offer", Price: "1250", PriceCurrency: "NOK", Description: "kr 1 250 per person"}},
		// Doubt: no price, only the words.
		{price: "690,- per person inkl. mat, 1 glass bobler", want: &Offer{Type: "Offer", Description: "690,- per person inkl. mat, 1 glass bobler"}},
		{price: "Voksne 250,-, barn 100,-", want: &Offer{Type: "Offer", Description: "Voksne 250,-, barn 100,-"}},
		{price: "250,-, barn gratis", want: &Offer{Type: "Offer", Description: "250,-, barn gratis"}},
		{price: "Se billettsiden", want: &Offer{Type: "Offer", Description: "Se billettsiden"}},
		{price: "250", want: &Offer{Type: "Offer", Description: "250"}},
	}

	for _, tt := range tests {
		got, free := offer(tt.price, tt.ticket)
		require.Equal(t, tt.want, got, tt.price)
		require.Equal(t, tt.free, free, tt.price)
	}
}

func TestVenueNode(t *testing.T) {
	nodes := []any{VenueNode(testBase), WebSiteNode(testBase)}
	checkNodes(t, nodes)

	v := VenueNode(testBase)
	require.Equal(t, []string{"LocalBusiness", "EventVenue"}, v.Type)
	require.Equal(t, []Place{
		{Type: "EventVenue", Name: "Låven", MaximumAttendeeCapacity: 100},
		{Type: "EventVenue", Name: "Fjøset", MaximumAttendeeCapacity: 68},
	}, v.ContainsPlace)
	require.Equal(t, "+4790919812", v.Telephone)
	require.True(t, strings.HasPrefix(v.Logo, testBase+"/static/Aspargesgaarden_HOVEDLOGO"), v.Logo)

	for _, img := range v.Image {
		require.True(t, strings.HasPrefix(img, testBase+"/static/"), img)
	}
}

// Text in the data cannot end the script element: the browser must see one
// JSON-LD block that reads back exactly as written.
func TestJSONLDCannotBreakOut(t *testing.T) {
	evil := `</script><script>alert(1)</script><!-- & "quotes" ` + " "
	e := testEvent()
	e.Title = evil

	out := render(Tags(testBase, Meta{Title: "x", Description: "y", Path: "/", JSONLD: EventNodes(testBase, EventInput{Event: e})}))

	doc, err := html.Parse(strings.NewReader(out))
	require.NoError(t, err)

	var scripts []string
	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.Data == "script" {
			require.NotNil(t, n.FirstChild)
			scripts = append(scripts, n.FirstChild.Data)
		}
	}

	require.Len(t, scripts, 1)

	var got graph
	require.NoError(t, json.Unmarshal([]byte(scripts[0]), &got))

	ev, ok := got.Graph[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, evil, ev["name"])
}
