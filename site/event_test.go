package site

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

// legacyEvent is one file of the old site's content/events, as the import
// stores it: eventDates minus the start date become ExtraDates, and the
// free-text "ca. 15:00" is normalised to 15:00.
type legacyEvent struct {
	file  string
	event Event
	// line is the date line the site writes for the file.
	label, line string
}

func legacy(id int64, title, start, startTime, end, endTime string, extras ...string) Event {
	e := Event{
		ID:        id,
		Title:     title,
		Slug:      Slugify(title),
		Start:     d(start),
		StartTime: startTime,
		EndTime:   endTime,
		Status:    Published,
	}
	e.Year = e.Start.Year
	if end != "" {
		e.End = d(end)
	}
	for _, x := range extras {
		e.ExtraDates = append(e.ExtraDates, d(x))
	}

	return e
}

// legacyEvents are in the old site's glob order (file name order).
var legacyEvents = []legacyEvent{
	{
		file:  "2024-04-24-ingunn-anvik.md",
		event: legacy(1, "Hageglede", "2024-04-24", "19:00", "", ""),
		label: "Dato:", line: "24. april 2024 kl. 19.00 på Aspargesgården",
	},
	{
		file:  "2024-05-29-bettan.md",
		event: legacy(2, "Vårkonsert med Bettan", "2024-05-29", "18:30", "", ""),
		label: "Dato:", line: "29. mai 2024 kl. 18.30 på Aspargesgården",
	},
	{
		file:  "2024-13-13-gourmet.md",
		event: legacy(3, "Gourmetaften", "2024-10-01", "", "", ""),
		label: "Dato:", line: "1. oktober 2024 på Aspargesgården",
	},
	{
		file:  "2025-03-22-afternoon.md",
		event: legacy(4, "Afternoon tea - ladies and gentlemen", "2025-03-22", "16:00", "", ""),
		label: "Dato:", line: "22. mars 2025 kl. 16.00 på Aspargesgården",
	},
	{
		file:  "2025-03-26-manneken.md",
		event: legacy(5, "Mannekengoppvisning", "2025-03-26", "18:30", "", ""),
		label: "Dato:", line: "26. mars 2025 kl. 18.30 på Aspargesgården",
	},
	{
		file:  "2025-06-04-didrik-solli.md",
		event: legacy(6, "O sole mio med Didrik Solli-Tangen", "2025-06-04", "17:00", "", ""),
		label: "Dato:", line: "4. juni 2025 kl. 17.00 på Aspargesgården",
	},
	{
		file:  "2025-12-01-didrik-solli-tangen.md",
		event: legacy(7, "Julekonsert med Didrik Solli-Tangen", "2025-12-01", "19:00", "", ""),
		label: "Dato:", line: "1. desember 2025 kl. 19.00 på Aspargesgården",
	},
	{
		file:  "2025-12-06-julemarked.md",
		event: legacy(8, "Julemarked på Aspargesgården", "2025-11-15", "12:00", "2025-11-16", "16:00"),
		label: "Dato:", line: "15. november kl. 12.00 – 16. november 2025 kl. 16.00 på Aspargesgården",
	},
	{
		file:  "2026-04-15-manneken.md",
		event: legacy(9, "Mannekengoppvisning", "2026-04-15", "18:30", "", ""),
		label: "Dato:", line: "15. april 2026 kl. 18.30 på Aspargesgården",
	},
	{
		file:  "2026-05-19-varkonsert.md",
		event: legacy(10, "Vårkonsert med Black & Lekker", "2026-05-19", "19:30", "", ""),
		label: "Dato:", line: "19. mai 2026 kl. 19.30 på Aspargesgården",
	},
	{
		file:  "2026-10-31-takkebakstkurs.md",
		event: legacy(11, "Takkebakstkurs med Anita Saga", "2026-10-31", "10:00", "2026-10-31", "15:00", "2026-11-28", "2026-11-29"),
		label: "Datoer:", line: "31. oktober, 28. november og 29. november 2026 kl. 10.00–15.00 på Aspargesgården",
	},
	{
		file:  "2026-11-14-julemarked.md",
		event: legacy(12, "Julemarked på Aspargesgården", "2026-11-14", "12:00", "2026-11-15", "16:00"),
		label: "Dato:", line: "14. november kl. 12.00 – 15. november 2026 kl. 16.00 på Aspargesgården",
	},
}

func allLegacy() []Event {
	out := make([]Event, len(legacyEvents))
	for i, l := range legacyEvents {
		out[i] = l.event
	}

	return out
}

func oslo(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, Oslo)
	if err != nil {
		panic(err)
	}

	return t
}

func ids(events []Event) []int64 {
	out := make([]int64, 0, len(events))
	for _, e := range events {
		out = append(out, e.ID)
	}

	return out
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name               string
		now                time.Time
		upcoming, pastHead []int64
	}{
		{
			// The old site's last build: dist/events lists exactly this.
			name:     "old build date",
			now:      oslo("2026-10-10 12:00"),
			upcoming: []int64{11, 12},
			pastHead: []int64{10, 9, 7, 8, 6, 5, 4, 3, 2, 1},
		},
		{
			name:     "course has started, market is sooner",
			now:      oslo("2026-11-10 12:00"),
			upcoming: []int64{12, 11},
			pastHead: []int64{10},
		},
		{
			name:     "second day of the market",
			now:      oslo("2026-11-15 15:00"),
			upcoming: []int64{12, 11},
		},
		{
			name:     "last minute of the market",
			now:      time.Date(2026, 11, 15, 22, 59, 0, 0, time.UTC),
			upcoming: []int64{12, 11},
		},
		{
			// 00:00 in Oslo is still the 15th in UTC.
			name:     "midnight in Oslo after the market",
			now:      time.Date(2026, 11, 15, 23, 0, 0, 0, time.UTC),
			upcoming: []int64{11},
			pastHead: []int64{12, 10},
		},
		{
			name:     "last course day",
			now:      oslo("2026-11-29 23:59"),
			upcoming: []int64{11},
		},
		{
			// The course started before the market, so it sorts after it
			// among past events.
			name:     "everything over",
			now:      oslo("2026-11-30 00:00"),
			upcoming: []int64{},
			pastHead: []int64{12, 11, 10},
		},
		{
			// The old site called the 2025 market past on its second day,
			// because it only looked at start dates.
			name:     "2025 market second day",
			now:      oslo("2025-11-16 10:00"),
			upcoming: []int64{8, 7, 9, 10, 11, 12},
			pastHead: []int64{6, 5, 4, 3, 2, 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upcoming, past := Split(allLegacy(), tt.now)
			require.Equal(t, tt.upcoming, ids(upcoming))
			require.Len(t, past, len(legacyEvents)-len(upcoming))
			if tt.pastHead != nil {
				require.Equal(t, tt.pastHead, ids(past)[:len(tt.pastHead)])
			}
		})
	}
}

func TestSplitUndatedDraftFirst(t *testing.T) {
	events := append(allLegacy(), Event{ID: 99, Title: "Ny kladd"})

	upcoming, past := Split(events, oslo("2026-11-10 12:00"))
	require.Equal(t, []int64{99, 12, 11}, ids(upcoming))
	require.NotContains(t, ids(past), int64(99))
}

func TestNextDate(t *testing.T) {
	course := legacyEvents[10].event
	market := legacyEvents[11].event

	tests := []struct {
		name  string
		event Event
		now   time.Time
		want  Date
	}{
		{"before start", course, oslo("2026-10-01 09:00"), d("2026-10-31")},
		{"on start day", course, oslo("2026-10-31 20:00"), d("2026-10-31")},
		{"between course days", course, oslo("2026-11-01 09:00"), d("2026-11-28")},
		{"last course day", course, oslo("2026-11-29 09:00"), d("2026-11-29")},
		{"course over", course, oslo("2026-11-30 09:00"), Date{}},
		{"inside a run of days", market, oslo("2026-11-15 09:00"), d("2026-11-15")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.event.NextDate(tt.now))
		})
	}
}

func TestMenu(t *testing.T) {
	market := legacyEvents[11].event
	market.ShowInMenu = true
	market.MenuLabel = "Julemarked"
	market.Slug = "julemarked"

	course := legacyEvents[10].event
	course.ShowInMenu = true

	draft := legacy(13, "Julemarked 2027", "2027-11-13", "", "", "")
	draft.Status = Draft
	draft.ShowInMenu = true

	events := []Event{course, legacyEvents[9].event, draft, market}

	require.Equal(t, []MenuEntry{
		{Label: "Julemarked", Path: "/arrangementer/2026/julemarked"},
		{Label: "Takkebakstkurs med Anita Saga", Path: "/arrangementer/2026/takkebakstkurs-med-anita-saga"},
	}, Menu(events, oslo("2026-11-15 23:59")))

	require.Equal(t, []MenuEntry{
		{Label: "Takkebakstkurs med Anita Saga", Path: "/arrangementer/2026/takkebakstkurs-med-anita-saga"},
	}, Menu(events, oslo("2026-11-16 00:00")))

	require.Empty(t, Menu(events, oslo("2026-11-30 00:00")))
}

func photoWithAlt(id int64, alt string) Photo {
	return Photo{ID: id, Alt: alt, Width: 100, Height: 100}
}

func TestRevise(t *testing.T) {
	now := oslo("2026-10-10 12:00")

	t.Run("new draft gets slug and year", func(t *testing.T) {
		got, err := Revise(nil, Event{
			Title:      "Vårkonsert med Black & Lekker",
			Start:      d("2027-05-19"),
			ExtraDates: []Date{d("2027-05-21"), d("2027-05-20")},
			Status:     Published, // ignored: only Publish publishes
		})
		require.NoError(t, err)
		require.Equal(t, "varkonsert-med-black-lekker", got.Slug)
		require.Equal(t, 2027, got.Year)
		require.Equal(t, Draft, got.Status)
		require.Equal(t, []Date{d("2027-05-20"), d("2027-05-21")}, got.ExtraDates)
	})

	t.Run("draft without date or title", func(t *testing.T) {
		got, err := Revise(nil, Event{})
		require.NoError(t, err)
		require.Empty(t, got.Slug)
		require.Zero(t, got.Year)
	})

	t.Run("hand-made slug is checked", func(t *testing.T) {
		_, err := Revise(nil, Event{Title: "x", Slug: "galleri"})
		require.ErrorIs(t, err, ErrSlugReserved)
	})

	t.Run("draft slug and year follow edits", func(t *testing.T) {
		prev, err := Revise(nil, Event{Title: "Julemarked", Start: d("2026-11-14")})
		require.NoError(t, err)
		prev.ID = 7

		next := prev
		next.Slug = "julemarked-2027"
		next.Start = d("2027-11-13")
		got, err := Revise(&prev, next)
		require.NoError(t, err)
		require.Equal(t, "julemarked-2027", got.Slug)
		require.Equal(t, 2027, got.Year)
		require.EqualValues(t, 7, got.ID)
	})

	// A working title with a typo must not become the address at publish.
	t.Run("draft slug follows the title until one is chosen", func(t *testing.T) {
		prev, err := Revise(nil, Event{Title: "Julemarkd", Start: d("2026-11-14")})
		require.NoError(t, err)
		require.Equal(t, "julemarkd", prev.Slug)
		require.True(t, prev.SlugFromTitle)

		// The app sends the stored slug back with the corrected title.
		next := prev
		next.Title = "Julemarked"
		fixed, err := Revise(&prev, next)
		require.NoError(t, err)
		require.Equal(t, "julemarked", fixed.Slug)
		require.True(t, fixed.SlugFromTitle)

		// One she types herself stays when the title changes.
		next = fixed
		next.Slug = "jul"
		chosen, err := Revise(&fixed, next)
		require.NoError(t, err)
		require.Equal(t, "jul", chosen.Slug)
		require.False(t, chosen.SlugFromTitle)

		next = chosen
		next.Title = "Julemarked på gården"
		kept, err := Revise(&chosen, next)
		require.NoError(t, err)
		require.Equal(t, "jul", kept.Slug)

		// Emptying the field gives the address back to the title.
		next = kept
		next.Slug = ""
		back, err := Revise(&kept, next)
		require.NoError(t, err)
		require.Equal(t, "julemarked-pa-garden", back.Slug)
		require.True(t, back.SlugFromTitle)
	})

	// "Lag kopi" keeps the slug on purpose: next year's market is
	// /arrangementer/2027/julemarked whatever its title says.
	t.Run("a copy's slug is chosen", func(t *testing.T) {
		src, err := Revise(nil, Event{Title: "Julemarked", Start: d("2026-11-14")})
		require.NoError(t, err)
		require.True(t, src.SlugFromTitle)

		cp, err := Revise(nil, src.Copy())
		require.NoError(t, err)
		require.False(t, cp.SlugFromTitle)

		next := cp
		next.Title, next.Start = "Julemarked 2027", d("2027-11-13")
		got, err := Revise(&cp, next)
		require.NoError(t, err)
		require.Equal(t, "julemarked", got.Slug)
		require.Equal(t, 2027, got.Year)
	})

	t.Run("published address is frozen", func(t *testing.T) {
		prev := legacyEvents[11].event
		prev.MainPhoto = &Photo{ID: 1, Alt: "Bålfat på tunet"}
		prev, err := prev.Publish(now)
		require.NoError(t, err)

		next := prev
		next.Slug = ""
		next.Start = d("2027-01-02")
		next.End = Date{}
		got, err := Revise(&prev, next)
		require.NoError(t, err)
		require.Equal(t, prev.Slug, got.Slug)
		require.Equal(t, 2026, got.Year, "year stays with the address")

		next.Slug = "julemarked"
		_, err = Revise(&prev, next)
		require.ErrorIs(t, err, ErrSlugFrozen)

		// Unpublished again, the address is still frozen.
		hidden := prev.Unpublish()
		_, err = Revise(&hidden, next)
		require.ErrorIs(t, err, ErrSlugFrozen)
	})

	t.Run("edit cannot break a published event", func(t *testing.T) {
		prev := legacyEvents[11].event
		prev.MainPhoto = &Photo{ID: 1, Alt: "Bålfat på tunet"}
		prev, err := prev.Publish(now)
		require.NoError(t, err)

		next := prev
		next.Title = " "
		next.MainPhoto = nil
		_, err = Revise(&prev, next)
		require.ErrorIs(t, err, ErrTitleRequired)
		require.ErrorIs(t, err, ErrMainPhotoRequired)
	})
}

func TestApply(t *testing.T) {
	prev := legacyEvents[11].event
	prev.MainPhoto = &Photo{ID: 1, Alt: "Bålfat på tunet"}
	prev, err := prev.Publish(oslo("2026-10-10 12:00"))
	require.NoError(t, err)

	// What Revise refuses, Apply still shows: no title, a bad time, and a
	// changed slug on a published event (kept, as a save would keep it).
	next := prev
	next.Title, next.StartTime, next.Slug = "", "25:00", "noe-annet"
	_, err = Revise(&prev, next)
	require.Error(t, err)

	got := Apply(&prev, next)
	require.Equal(t, prev.Slug, got.Slug)
	require.Equal(t, prev.Year, got.Year)
	require.Equal(t, Published, got.Status)
	require.Empty(t, got.Title)
	require.Equal(t, "25:00", got.StartTime)

	fresh := Apply(nil, Event{Title: "Påskemarked", Start: d("2027-03-27")})
	require.Equal(t, "paskemarked", fresh.Slug)
	require.Equal(t, 2027, fresh.Year)
	require.Equal(t, Draft, fresh.Status)
}

func TestValidate(t *testing.T) {
	base := Event{Title: "Kurs", Slug: "kurs", Start: d("2026-10-31"), StartTime: "10:00"}

	tests := []struct {
		name string
		mod  func(*Event)
		want error
	}{
		{"valid", func(*Event) {}, nil},
		{"loose time", func(e *Event) { e.EndTime = "ca. 15:00" }, ErrInvalidTime},
		{"end day before start", func(e *Event) { e.End = d("2026-10-30") }, ErrEndBeforeStart},
		{"end without start", func(e *Event) { e.Start = Date{}; e.End = d("2026-10-30") }, ErrEndBeforeStart},
		{"same-day end time before start time", func(e *Event) { e.EndTime = "09:00" }, ErrEndBeforeStart},
		{"overnight is fine", func(e *Event) { e.End = d("2026-11-01"); e.EndTime = "02:00" }, nil},
		{"extra date before start", func(e *Event) { e.ExtraDates = []Date{d("2026-10-01")} }, ErrExtraDate},
		{"extra date on start", func(e *Event) { e.ExtraDates = []Date{d("2026-10-31")} }, ErrExtraDate},
		{"extra date twice", func(e *Event) { e.ExtraDates = []Date{d("2026-11-28"), d("2026-11-28")} }, ErrExtraDate},
		{"ticket link", func(e *Event) { e.TicketURL = "https://ticketco.events/no/nb/e/julemarked" }, nil},
		{"ticket link without scheme", func(e *Event) { e.TicketURL = "ticketco.events/x" }, ErrTicketURL},
		{"ticket link javascript", func(e *Event) { e.TicketURL = "javascript:alert(1)" }, ErrTicketURL},
		{"photo twice", func(e *Event) {
			e.MainPhoto = &Photo{ID: 1}
			e.ExtraPhotos = []Photo{{ID: 1}}
		}, ErrDuplicatePhoto},
		{"poster is the main photo", func(e *Event) {
			e.MainPhoto = &Photo{ID: 1}
			e.Poster = &Photo{ID: 1}
		}, ErrDuplicatePhoto},
		{"draft may lack a photo", func(e *Event) { e.Status = Draft }, nil},
		{"published needs a photo", func(e *Event) { e.Status = Published }, ErrMainPhotoRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := base
			tt.mod(&e)
			err := e.Validate()
			if tt.want == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestPublish(t *testing.T) {
	first := oslo("2026-10-10 12:00")

	_, err := Event{}.Publish(first)
	for _, want := range []error{ErrTitleRequired, ErrStartRequired, ErrSlugInvalid, ErrMainPhotoRequired} {
		require.ErrorIs(t, err, want)
	}

	e := legacyEvents[11].event
	e.Status = Draft
	e.MainPhoto = &Photo{ID: 1, Alt: "Bålfat på tunet"}
	e.ExtraPhotos = []Photo{photoWithAlt(2, "")}
	_, err = e.Publish(first)
	require.ErrorIs(t, err, ErrAltRequired)

	e.ExtraPhotos[0].Alt = "Boder i låven"
	got, err := e.Publish(first)
	require.NoError(t, err)
	require.Equal(t, Published, got.Status)
	require.Equal(t, first, got.PublishedAt)
	require.True(t, got.SlugFrozen())

	again, err := got.Unpublish().Publish(first.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, first, again.PublishedAt, "first publish time is kept")
}

func TestCopy(t *testing.T) {
	src := legacyEvents[11].event
	src.ID = 12
	src.Slug = "julemarked"
	src.ShowInMenu = true
	src.Cancelled = true
	src.MainPhoto = &Photo{ID: 1, Alt: "Bålfat"}
	src.ExtraPhotos = []Photo{photoWithAlt(2, "Boder")}
	src.Poster = new(photoWithAlt(3, "Plakat"))
	src.PublishedAt = oslo("2026-10-10 12:00")
	src.CreatedAt = src.PublishedAt

	got := src.Copy()

	want := Event{
		Slug:        "julemarked",
		Title:       src.Title,
		StartTime:   "12:00",
		EndTime:     "16:00",
		ShowInMenu:  true,
		Status:      Draft,
		MainPhoto:   &Photo{ID: 1, Alt: "Bålfat"},
		ExtraPhotos: []Photo{photoWithAlt(2, "Boder")},
		Poster:      new(photoWithAlt(3, "Plakat")),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("Copy() mismatch (-want +got):\n%s", diff)
	}

	got.MainPhoto.Alt = "changed"
	got.ExtraPhotos[0].Alt = "changed"
	got.Poster.Alt = "changed"
	require.Equal(t, "Bålfat", src.MainPhoto.Alt, "copy shares no photo with its source")
	require.Equal(t, "Boder", src.ExtraPhotos[0].Alt)
	require.Equal(t, "Plakat", src.Poster.Alt)
}

func TestPath(t *testing.T) {
	e := Event{Year: 2026, Slug: "julemarked"}
	require.Equal(t, "/arrangementer/2026/julemarked", e.Path())
}
