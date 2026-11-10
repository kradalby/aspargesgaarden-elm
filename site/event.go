package site

import (
	"cmp"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Status is an event's lifecycle state. Only published events are public.
type Status string

const (
	Draft     Status = "draft"
	Published Status = "published"
)

// Event is one event page.
type Event struct {
	ID int64
	// Year and Slug form the address /arrangementer/{Year}/{Slug}. Until the
	// first publish they follow the start date and the title; from then on
	// they are frozen for good, so shared links keep working.
	Year  int
	Slug  string
	Title string
	// Summary is the short text in the list; BodyMD the full Markdown text.
	Summary   string
	BodyMD    string
	Start     Date
	StartTime string // HH:MM or ""
	// End is the last day of a run of consecutive days; zero means the event
	// ends on the day it starts.
	End     Date
	EndTime string // HH:MM or ""
	// ExtraDates are further separate days (a course), all after Start,
	// sorted. Each repeats StartTime and EndTime.
	ExtraDates  []Date
	PriceText   string
	TicketURL   string
	ShowInMenu  bool
	MenuLabel   string
	Status      Status
	Cancelled   bool
	PublishedAt time.Time // first publish; zero if never published
	CreatedAt   time.Time
	UpdatedAt   time.Time
	MainPhoto   *Photo
	ExtraPhotos []Photo
}

// Path is the event's address on the public site.
func (e Event) Path() string {
	return fmt.Sprintf("/arrangementer/%d/%s", e.Year, e.Slug)
}

// SlugFrozen reports whether the address can no longer change.
func (e Event) SlugFrozen() bool { return !e.PublishedAt.IsZero() }

// Revise applies the save rules to next, the edited version of prev (nil for
// a new event), and returns what is to be stored. Status and publish time only
// change through Publish and Unpublish. Until first publish an empty slug is
// made from the title and the year follows the start date; after it, both are
// kept and an attempt to change the slug fails.
func Revise(prev *Event, next Event) (Event, error) {
	if prev != nil && prev.SlugFrozen() && next.Slug != "" && next.Slug != prev.Slug {
		return Event{}, fieldErr(FieldSlug, fmt.Errorf("%w: %q is published", ErrSlugFrozen, prev.Slug))
	}

	next = Apply(prev, next)

	if err := next.Validate(); err != nil {
		return Event{}, err
	}

	return next, nil
}

// Apply is Revise without the rules: the event as it would be stored. The
// admin previews drafts with it, which may break any rule while being typed.
func Apply(prev *Event, next Event) Event {
	if prev == nil {
		next.Status, next.PublishedAt = Draft, time.Time{}
	} else {
		next.ID, next.Status, next.PublishedAt, next.CreatedAt = prev.ID, prev.Status, prev.PublishedAt, prev.CreatedAt
	}

	if next.SlugFrozen() {
		next.Slug, next.Year = prev.Slug, prev.Year
	} else {
		if next.Slug == "" {
			next.Slug = Slugify(next.Title)
		}
		next.Year = next.Start.Year
	}

	next.ExtraDates = slices.SortedFunc(slices.Values(next.ExtraDates), Date.Compare)

	return next
}

// Validate checks the rules every stored event keeps, plus Publishable when
// the event is published, so an edit can never break a live page.
func (e Event) Validate() error {
	var errs []error

	if e.Slug != "" {
		errs = append(errs, fieldErr(FieldSlug, ValidateSlug(e.Slug)))
	}

	for _, t := range []struct {
		f     Field
		clock string
	}{{FieldStartTime, e.StartTime}, {FieldEndTime, e.EndTime}} {
		if t.clock != "" && !ValidClock(t.clock) {
			errs = append(errs, fieldErr(t.f, fmt.Errorf("%w: %q", ErrInvalidTime, t.clock)))
		}
	}

	if !e.End.IsZero() && (e.Start.IsZero() || e.End.Before(e.Start)) {
		errs = append(errs, fieldErr(FieldEnd, ErrEndBeforeStart))
	}

	sameDay := e.End.IsZero() || e.End == e.Start
	if sameDay && e.StartTime != "" && e.EndTime != "" && e.EndTime <= e.StartTime {
		errs = append(errs, fieldErr(FieldEndTime, fmt.Errorf("%w: %s-%s", ErrEndBeforeStart, e.StartTime, e.EndTime)))
	}

	for i, d := range e.ExtraDates {
		if e.Start.IsZero() || !d.After(e.Start) || (i > 0 && d == e.ExtraDates[i-1]) {
			errs = append(errs, fieldErr(FieldExtraDates, fmt.Errorf("%w: %s", ErrExtraDate, d)))
			break
		}
	}

	if e.TicketURL != "" && !validLink(e.TicketURL) {
		errs = append(errs, fieldErr(FieldTicketURL, fmt.Errorf("%w: %q", ErrTicketURL, e.TicketURL)))
	}

	seen := map[int64]bool{}
	for _, p := range e.photos() {
		// The second use of a photo is always among the further photos.
		if seen[p.ID] {
			errs = append(errs, &FieldError{Field: FieldExtraPhotos, PhotoID: p.ID, Err: fmt.Errorf("%w: %d", ErrDuplicatePhoto, p.ID)})
		}
		seen[p.ID] = true
	}

	if e.Status == Published {
		errs = append(errs, e.Publishable())
	}

	return errors.Join(errs...)
}

// validLink reports whether s is an absolute http or https address: links the
// pages put in an href must not run script or point nowhere.
func validLink(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// Publishable lists what is missing before the event can go public: a title,
// a start date, an address, and a main photo, with alt text on every photo.
// The photo is required because the list, the page banner and the share
// image are all built around it.
func (e Event) Publishable() error {
	var errs []error

	if strings.TrimSpace(e.Title) == "" {
		errs = append(errs, fieldErr(FieldTitle, ErrTitleRequired))
	}

	if e.Start.IsZero() {
		errs = append(errs, fieldErr(FieldStart, ErrStartRequired))
	}

	if e.Slug == "" {
		errs = append(errs, fieldErr(FieldSlug, fmt.Errorf("%w: empty", ErrSlugInvalid)))
	}

	if e.MainPhoto == nil {
		errs = append(errs, fieldErr(FieldMainPhoto, ErrMainPhotoRequired))
	}

	for _, p := range e.photos() {
		if strings.TrimSpace(p.Alt) == "" {
			errs = append(errs, e.photoErr(p.ID, ErrAltRequired))
		}
	}

	return errors.Join(errs...)
}

func (e Event) photos() []Photo {
	if e.MainPhoto == nil {
		return e.ExtraPhotos
	}

	return append([]Photo{*e.MainPhoto}, e.ExtraPhotos...)
}

// Publish makes the event public. The first publish stamps PublishedAt,
// which freezes the address.
func (e Event) Publish(now time.Time) (Event, error) {
	e.Status = Published
	if e.PublishedAt.IsZero() {
		e.PublishedAt = now
	}

	if err := e.Validate(); err != nil {
		return Event{}, err
	}

	return e, nil
}

// Unpublish hides the event again. Its address stays frozen, so publishing it
// later brings back the same link.
func (e Event) Unpublish() Event {
	e.Status = Draft
	return e
}

// Copy is "Lag kopi": a new draft of a recurring event with the same text,
// photos and menu setting, and no dates, since those are what changes. The
// slug is kept so a copied julemarked stays "julemarked"; it can be edited
// until the copy is published.
func (e Event) Copy() Event {
	c := e
	c.ID = 0
	c.Year = 0
	c.Start, c.End, c.ExtraDates = Date{}, Date{}, nil
	c.Status, c.Cancelled = Draft, false
	c.PublishedAt, c.CreatedAt, c.UpdatedAt = time.Time{}, time.Time{}, time.Time{}
	if e.MainPhoto != nil {
		p := *e.MainPhoto
		c.MainPhoto = &p
	}
	c.ExtraPhotos = slices.Clone(e.ExtraPhotos)

	return c
}

// days is every day the event starts on: Start and the extra dates.
func (e Event) days() []Date {
	return append([]Date{e.Start}, e.ExtraDates...)
}

// LastDay is the final day the event runs.
func (e Event) LastDay() Date {
	return slices.MaxFunc(append(e.days(), e.End), Date.Compare)
}

// IsOver reports whether the event's last day has ended in Oslo. Until then,
// including during the last day, it counts as upcoming.
func (e Event) IsOver(now time.Time) bool {
	return Today(now).After(e.LastDay())
}

// NextDate is the day the event next starts: the first of its days that is
// today or later, or today while a multi-day run is in progress. It is zero
// once the event is over.
func (e Event) NextDate(now time.Time) Date {
	if e.IsOver(now) {
		return Date{}
	}

	today := Today(now)
	for _, d := range slices.SortedFunc(slices.Values(e.days()), Date.Compare) {
		if !d.Before(today) {
			return d
		}
	}

	return today
}

// Split divides events into upcoming, soonest first by NextDate, and past,
// newest first by start date. This is the old site's order; the difference is
// that an event stays upcoming until its last day is over, where the old site
// dropped a multi-day event to past after its first day.
func Split(events []Event, now time.Time) (upcoming, past []Event) {
	for _, e := range events {
		// Only a draft in the admin's preview lacks a date: it is shown
		// first among what is coming, where its date will put it.
		if !e.Start.IsZero() && e.IsOver(now) {
			past = append(past, e)
		} else {
			upcoming = append(upcoming, e)
		}
	}

	slices.SortStableFunc(upcoming, func(a, b Event) int {
		return cmp.Or(
			a.NextDate(now).Compare(b.NextDate(now)),
			a.Start.Compare(b.Start),
			cmp.Compare(a.ID, b.ID),
		)
	})
	slices.SortStableFunc(past, func(a, b Event) int {
		return cmp.Or(b.Start.Compare(a.Start), cmp.Compare(b.ID, a.ID))
	})

	return upcoming, past
}

// MenuEntry is a link in the site header.
type MenuEntry struct {
	Label string
	Path  string
}

// Menu is the header links contributed by events: each published event with
// ShowInMenu, labelled by MenuLabel or else its title, until it is over.
func Menu(events []Event, now time.Time) []MenuEntry {
	var shown []Event
	for _, e := range events {
		if e.Status == Published && e.ShowInMenu {
			shown = append(shown, e)
		}
	}

	upcoming, _ := Split(shown, now)

	entries := make([]MenuEntry, 0, len(upcoming))
	for _, e := range upcoming {
		label := strings.TrimSpace(e.MenuLabel)
		if label == "" {
			label = e.Title
		}
		entries = append(entries, MenuEntry{Label: label, Path: e.Path()})
	}

	return entries
}
