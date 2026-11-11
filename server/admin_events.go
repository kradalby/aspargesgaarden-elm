package server

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/kradalby/asparges/site"
)

// adminAPI registers every route of docs/admin-api.md.
func (s *Server) adminAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/api/me", s.me)

	mux.HandleFunc("GET /admin/api/events", s.listEvents)
	mux.HandleFunc("POST /admin/api/events", s.createEvent)
	mux.HandleFunc("GET /admin/api/events/{id}", s.withEvent(func(w http.ResponseWriter, r *http.Request, e site.Event) {
		s.writeJSON(w, r, http.StatusOK, s.eventJSON(r.Context(), e))
	}))
	mux.HandleFunc("PUT /admin/api/events/{id}", s.withEvent(s.updateEvent))
	mux.HandleFunc("POST /admin/api/events/{id}/publish", s.withEvent(s.publishEvent))
	mux.HandleFunc("POST /admin/api/events/{id}/unpublish", s.withEvent(s.unpublishEvent))
	mux.HandleFunc("POST /admin/api/events/{id}/copy", s.withEvent(s.copyEvent))
	mux.HandleFunc("DELETE /admin/api/events/{id}", s.withEvent(s.deleteEvent))
	mux.HandleFunc("POST /admin/api/preview", s.preview)

	s.photoRoutes(mux)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, r, http.StatusOK, map[string]string{"name": userOf(r.Context()).Name, "siteUrl": s.cfg.BaseURL})
}

type eventJSON struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	Slug       string `json:"slug"`
	SlugFrozen bool   `json:"slugFrozen"`
	// SlugFromTitle says the slug follows the title until first publish.
	SlugFromTitle bool        `json:"slugFromTitle"`
	Year          int         `json:"year"`
	Path          string      `json:"path"`
	URL           string      `json:"url"`
	DateText      string      `json:"dateText"`
	IsOver        bool        `json:"isOver"`
	Title         string      `json:"title"`
	Summary       string      `json:"summary"`
	Body          string      `json:"body"`
	Start         string      `json:"start"`
	StartTime     string      `json:"startTime"`
	End           string      `json:"end"`
	EndTime       string      `json:"endTime"`
	ExtraDates    []string    `json:"extraDates"`
	PriceText     string      `json:"priceText"`
	TicketURL     string      `json:"ticketUrl"`
	ShowInMenu    bool        `json:"showInMenu"`
	MenuLabel     string      `json:"menuLabel"`
	Cancelled     bool        `json:"cancelled"`
	MainPhoto     *photoJSON  `json:"mainPhoto"`
	ExtraPhotos   []photoJSON `json:"extraPhotos"`
	Poster        *photoJSON  `json:"poster"`
	PublishedAt   *string     `json:"publishedAt"`
	CreatedAt     string      `json:"createdAt"`
	UpdatedAt     string      `json:"updatedAt"`
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func (s *Server) eventJSON(ctx context.Context, e site.Event) eventJSON {
	out := eventJSON{
		ID: e.ID, Status: string(e.Status), Slug: e.Slug, SlugFrozen: e.SlugFrozen(), SlugFromTitle: e.SlugFromTitle, Year: e.Year,
		DateText: e.DateLine(), IsOver: !e.Start.IsZero() && e.IsOver(s.cfg.Now()),
		Title: e.Title, Summary: e.Summary, Body: e.BodyMD,
		Start: e.Start.String(), StartTime: e.StartTime, End: e.End.String(), EndTime: e.EndTime,
		ExtraDates: []string{}, PriceText: e.PriceText, TicketURL: e.TicketURL,
		ShowInMenu: e.ShowInMenu, MenuLabel: e.MenuLabel, Cancelled: e.Cancelled,
		ExtraPhotos: []photoJSON{},
		CreatedAt:   stamp(e.CreatedAt), UpdatedAt: stamp(e.UpdatedAt),
	}

	// An address needs both its parts: no date or no title yet, no page.
	if e.Year != 0 && e.Slug != "" {
		out.Path = e.Path()
		out.URL = s.cfg.BaseURL + out.Path
	}

	for _, d := range e.ExtraDates {
		out.ExtraDates = append(out.ExtraDates, d.String())
	}

	if e.MainPhoto != nil {
		out.MainPhoto = new(s.photoJSON(ctx, *e.MainPhoto))
	}

	for _, p := range e.ExtraPhotos {
		out.ExtraPhotos = append(out.ExtraPhotos, s.photoJSON(ctx, p))
	}

	if e.Poster != nil {
		out.Poster = new(s.photoJSON(ctx, *e.Poster))
	}

	if !e.PublishedAt.IsZero() {
		out.PublishedAt = new(stamp(e.PublishedAt))
	}

	return out
}

type photoRefJSON struct {
	ID  int64  `json:"id"`
	Alt string `json:"alt"`
	// Caption left out (nil) keeps the stored one.
	Caption        *string `json:"caption"`
	PhotographerID *int64  `json:"photographerId"`
	// UpdatedAt is the photo's version the form loaded; "" skips the check.
	UpdatedAt string `json:"updatedAt"`
}

// eventInput is the form: an event's editable fields, photos by reference.
type eventInput struct {
	UpdatedAt   string         `json:"updatedAt"`
	Slug        string         `json:"slug"`
	Title       string         `json:"title"`
	Summary     string         `json:"summary"`
	Body        string         `json:"body"`
	Start       string         `json:"start"`
	StartTime   string         `json:"startTime"`
	End         string         `json:"end"`
	EndTime     string         `json:"endTime"`
	ExtraDates  []string       `json:"extraDates"`
	PriceText   string         `json:"priceText"`
	TicketURL   string         `json:"ticketUrl"`
	ShowInMenu  bool           `json:"showInMenu"`
	MenuLabel   string         `json:"menuLabel"`
	Cancelled   bool           `json:"cancelled"`
	MainPhoto   *photoRefJSON  `json:"mainPhoto"`
	ExtraPhotos []photoRefJSON `json:"extraPhotos"`
	Poster      *photoRefJSON  `json:"poster"`
}

// toEvent turns the form into an event, loading each photo it refers to
// and setting the alt text, caption and credit the form gives it. strict is for
// saves: an unreadable date is an error; a preview leaves it out.
func (s *Server) toEvent(ctx context.Context, in eventInput, strict bool) (site.Event, error) {
	e := site.Event{
		Slug:       strings.TrimSpace(in.Slug),
		Title:      strings.TrimSpace(in.Title),
		Summary:    in.Summary,
		BodyMD:     in.Body,
		StartTime:  in.StartTime,
		EndTime:    in.EndTime,
		PriceText:  strings.TrimSpace(in.PriceText),
		TicketURL:  strings.TrimSpace(in.TicketURL),
		ShowInMenu: in.ShowInMenu,
		MenuLabel:  strings.TrimSpace(in.MenuLabel),
		Cancelled:  in.Cancelled,
	}

	var errs []error

	date := func(f site.Field, s string) site.Date {
		if s == "" {
			return site.Date{}
		}

		d, err := site.ParseDate(s)
		if err != nil && strict {
			errs = append(errs, &site.FieldError{Field: f, Err: err})
		}

		return d
	}

	e.Start = date(site.FieldStart, in.Start)
	e.End = date(site.FieldEnd, in.End)

	for _, x := range in.ExtraDates {
		if d := date(site.FieldExtraDates, x); !d.IsZero() {
			e.ExtraDates = append(e.ExtraDates, d)
		}
	}

	photo := func(f site.Field, ref photoRefJSON) (site.Photo, bool) {
		p, err := s.cfg.Store.Photo(ctx, ref.ID)
		if err != nil {
			errs = append(errs, &site.FieldError{Field: f, PhotoID: ref.ID, Err: err})
			return site.Photo{}, false
		}

		p.Alt, p.Photographer, p.UpdatedAt = ref.Alt, nil, time.Time{}
		if ref.Caption != nil {
			p.Caption = *ref.Caption
		}

		if strict && ref.UpdatedAt != "" {
			if p.UpdatedAt, err = time.Parse(time.RFC3339, ref.UpdatedAt); err != nil {
				errs = append(errs, &site.FieldError{Field: f, PhotoID: ref.ID, Err: fmt.Errorf("%w: updatedAt %q", errBadRequest, ref.UpdatedAt)})
				return site.Photo{}, false
			}
		}

		if ref.PhotographerID != nil {
			ph, err := s.cfg.Store.Photographer(ctx, *ref.PhotographerID)
			if err != nil {
				errs = append(errs, &site.FieldError{Field: f, PhotoID: ref.ID, Err: err})
				return site.Photo{}, false
			}

			p.Photographer = &ph
		}

		return p, true
	}

	if in.MainPhoto != nil {
		if p, ok := photo(site.FieldMainPhoto, *in.MainPhoto); ok {
			e.MainPhoto = &p
		}
	}

	for _, ref := range in.ExtraPhotos {
		if p, ok := photo(site.FieldExtraPhotos, ref); ok {
			e.ExtraPhotos = append(e.ExtraPhotos, p)
		}
	}

	if in.Poster != nil {
		if p, ok := photo(site.FieldPoster, *in.Poster); ok {
			e.Poster = &p
		}
	}

	return e, errors.Join(errs...)
}

func (s *Server) withEvent(h func(http.ResponseWriter, *http.Request, site.Event)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err == nil {
			var e site.Event
			if e, err = s.cfg.Store.Event(r.Context(), id); err == nil {
				h(w, r, e)
				return
			}
		}

		s.apiFail(w, r, err, nil)
	}
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	all, err := s.cfg.Store.Events(r.Context())
	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	var drafts, published []site.Event

	for _, e := range all {
		if e.Status == site.Published {
			published = append(published, e)
		} else {
			drafts = append(drafts, e)
		}
	}

	slices.SortStableFunc(drafts, func(a, b site.Event) int {
		return cmp.Or(b.UpdatedAt.Compare(a.UpdatedAt), cmp.Compare(b.ID, a.ID))
	})

	upcoming, past := site.Split(published, s.cfg.Now())

	ctx := r.Context()
	s.writeJSON(w, r, http.StatusOK, map[string][]eventJSON{
		"upcoming": s.eventsJSON(ctx, upcoming), "drafts": s.eventsJSON(ctx, drafts), "past": s.eventsJSON(ctx, past),
	})
}

func (s *Server) eventsJSON(ctx context.Context, events []site.Event) []eventJSON {
	out := make([]eventJSON, len(events))
	for i, e := range events {
		out[i] = s.eventJSON(ctx, e)
	}

	return out
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	var in eventInput
	if err := readJSON(w, r, &in); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	e, err := s.toEvent(r.Context(), in, true)
	if err == nil {
		e, err = s.cfg.Store.CreateEvent(r.Context(), e)
	}

	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	s.Changed()
	s.audit(r, "event created", "event", e.ID, "title", e.Title)
	s.writeJSON(w, r, http.StatusCreated, s.eventJSON(r.Context(), e))
}

func (s *Server) updateEvent(w http.ResponseWriter, r *http.Request, prev site.Event) {
	var in eventInput
	if err := readJSON(w, r, &in); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	e, err := s.toEvent(r.Context(), in, true)
	if err == nil {
		e.ID = prev.ID
		e, err = s.cfg.Store.UpdateEvent(r.Context(), e, parseSeen(in.UpdatedAt))
	}

	s.eventWritten(w, r, e, err, "event saved")
}

func (s *Server) publishEvent(w http.ResponseWriter, r *http.Request, prev site.Event) {
	seen, ok := s.readSeen(w, r)
	if !ok {
		return
	}

	e, err := s.cfg.Store.Publish(r.Context(), prev.ID, seen)
	s.eventWritten(w, r, e, err, "event published")
}

func (s *Server) unpublishEvent(w http.ResponseWriter, r *http.Request, prev site.Event) {
	seen, ok := s.readSeen(w, r)
	if !ok {
		return
	}

	e, err := s.cfg.Store.Unpublish(r.Context(), prev.ID, seen)
	s.eventWritten(w, r, e, err, "event unpublished")
}

func (s *Server) readSeen(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	var in struct {
		UpdatedAt string `json:"updatedAt"`
	}

	if err := readJSON(w, r, &in); err != nil {
		s.apiFail(w, r, err, nil)
		return time.Time{}, false
	}

	return parseSeen(in.UpdatedAt), true
}

// parseSeen reads the updatedAt the app last loaded. Anything unreadable
// can match no stored event, so it fails the precondition as stale.
func parseSeen(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}

	return t
}

// eventWritten answers a precondition write: the event, or on a stale
// precondition the stored one, so the app can offer both versions.
func (s *Server) eventWritten(w http.ResponseWriter, r *http.Request, e site.Event, err error, action string) {
	if errors.Is(err, site.ErrStale) {
		var current site.Event

		id, perr := pathID(r)
		if perr == nil {
			current, perr = s.cfg.Store.Event(r.Context(), id)
		}

		if perr != nil {
			s.apiFail(w, r, perr, nil)
			return
		}

		s.apiFail(w, r, err, map[string]any{
			"current": s.eventJSON(r.Context(), current),
			"photos":  s.stalePhotos(r.Context(), err),
		})

		return
	}

	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	// A published event's photos are wanted on the public pages now.
	var shas []string
	if e.Status == site.Published {
		for _, p := range e.Photos() {
			shas = append(shas, p.SHA256)
		}
	}

	s.Changed(shas...)
	s.audit(r, action, "event", e.ID, "title", e.Title, "status", e.Status)
	s.writeJSON(w, r, http.StatusOK, s.eventJSON(r.Context(), e))
}

// stalePhotos is each photo a conflict names, as stored now: the form's
// version of it is older, so "keep mine" needs its updatedAt and "take
// theirs" its description.
func (s *Server) stalePhotos(ctx context.Context, err error) []photoJSON {
	out := []photoJSON{}

	for _, leaf := range leaves(err) {
		var fe *site.FieldError
		if !errors.Is(leaf, site.ErrStale) || !errors.As(leaf, &fe) || fe.PhotoID == 0 {
			continue
		}

		if p, err := s.cfg.Store.Photo(ctx, fe.PhotoID); err == nil {
			out = append(out, s.photoJSON(ctx, p))
		}
	}

	return out
}

func (s *Server) copyEvent(w http.ResponseWriter, r *http.Request, src site.Event) {
	e, err := s.cfg.Store.CopyEvent(r.Context(), src.ID)
	if err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	s.Changed()
	s.audit(r, "event copied", "event", e.ID, "from", src.ID)
	s.writeJSON(w, r, http.StatusCreated, s.eventJSON(r.Context(), e))
}

func (s *Server) deleteEvent(w http.ResponseWriter, r *http.Request, e site.Event) {
	if err := s.cfg.Store.DeleteEvent(r.Context(), e.ID); err != nil {
		s.apiFail(w, r, err, nil)
		return
	}

	s.Changed()
	s.audit(r, "event deleted", "event", e.ID, "title", e.Title, "status", e.Status)
	w.WriteHeader(http.StatusNoContent)
}
