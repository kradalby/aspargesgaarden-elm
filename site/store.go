package site

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kradalby/asparges/db"
	"github.com/kradalby/asparges/db/dbsqlc"
	"github.com/kradalby/asparges/images"
)

// Store reads and writes the domain types through the database, applying the
// rules in this package on every write. now is the clock for timestamps and
// for anything that depends on today.
type Store struct {
	db  *db.DB
	now func() time.Time
}

// NewStore returns a Store over d with the given clock.
func NewStore(d *db.DB, now func() time.Time) *Store {
	return &Store{db: d, now: now}
}

// read runs fn in one read transaction, so what it puts together from
// several tables is one moment's state, whatever is written meanwhile.
func read[T any](ctx context.Context, s *Store, fn func(*dbsqlc.Queries) (T, error)) (T, error) {
	var out T

	err := s.db.ReadTx(ctx, func(q *dbsqlc.Queries) error {
		var err error
		out, err = fn(q)

		return err
	})

	return out, err
}

// write runs fn in one transaction and returns what fn read back inside it.
// The commit is the last step that can fail: a write that reports an error
// stored nothing, and one that reports success stored all of it, so a
// caller can tie what follows a write (dropping kept pages) to its result.
func write[T any](ctx context.Context, s *Store, fn func(*dbsqlc.Queries) (T, error)) (T, error) {
	var out T

	err := s.db.InTx(ctx, func(q *dbsqlc.Queries) error {
		var err error
		out, err = fn(q)

		return err
	})
	if err != nil {
		var zero T
		return zero, err
	}

	return out, nil
}

// Events lists every event, drafts included, newest start first.
func (s *Store) Events(ctx context.Context) ([]Event, error) {
	return read(ctx, s, func(q *dbsqlc.Queries) ([]Event, error) {
		return events(ctx, q, "list events", q.ListEvents)
	})
}

// PublishedEvents lists the public events; Split divides them for the page.
func (s *Store) PublishedEvents(ctx context.Context) ([]Event, error) {
	return read(ctx, s, func(q *dbsqlc.Queries) ([]Event, error) {
		return events(ctx, q, "list published events", q.ListPublishedEvents)
	})
}

// Menu is the header links that events contribute today.
func (s *Store) Menu(ctx context.Context) ([]MenuEntry, error) {
	evs, err := read(ctx, s, func(q *dbsqlc.Queries) ([]Event, error) {
		return events(ctx, q, "list menu events", q.ListPublishedMenuEvents)
	})
	if err != nil {
		return nil, err
	}

	return Menu(evs, s.now()), nil
}

// Event loads one event by id, whatever its status.
func (s *Store) Event(ctx context.Context, id int64) (Event, error) {
	return read(ctx, s, func(q *dbsqlc.Queries) (Event, error) {
		return event(ctx, q, id)
	})
}

// PublishedEvent loads the public event at /arrangementer/{year}/{slug}.
func (s *Store) PublishedEvent(ctx context.Context, year int, slug string) (Event, error) {
	return read(ctx, s, func(q *dbsqlc.Queries) (Event, error) {
		return one(ctx, q, fmt.Sprintf("event %d/%s", year, slug), func(ctx context.Context) (dbsqlc.Event, error) {
			return q.GetPublishedEventByYearSlug(ctx, dbsqlc.GetPublishedEventByYearSlugParams{
				Year: sql.NullInt64{Int64: int64(year), Valid: true},
				Slug: slug,
			})
		})
	})
}

// NewestPublishedEvent is the latest public event with slug, the target of
// evergreen addresses such as /julemarked.
func (s *Store) NewestPublishedEvent(ctx context.Context, slug string) (Event, error) {
	return read(ctx, s, func(q *dbsqlc.Queries) (Event, error) {
		return one(ctx, q, "newest event "+slug, func(ctx context.Context) (dbsqlc.Event, error) {
			return q.GetNewestPublishedEventBySlug(ctx, slug)
		})
	})
}

// CreateEvent stores e as a new draft; see Revise for the rules.
func (s *Store) CreateEvent(ctx context.Context, e Event) (Event, error) {
	next, err := Revise(nil, e)
	if err != nil {
		return Event{}, err
	}

	now := s.now()
	next.CreatedAt, next.UpdatedAt = now, now

	return write(ctx, s, func(q *dbsqlc.Queries) (Event, error) {
		id, err := q.CreateEvent(ctx, createParams(next))
		if err != nil {
			return Event{}, eventWriteErr(err, next)
		}

		if err := writeEventChildren(ctx, q, id, next); err != nil {
			return Event{}, err
		}

		return event(ctx, q, id)
	})
}

// UpdateEvent saves an edit of an existing event; see Revise for the rules.
// seen is the UpdatedAt the editor loaded: if anyone saved the event since,
// nothing is written and the error is ErrStale.
func (s *Store) UpdateEvent(ctx context.Context, e Event, seen time.Time) (Event, error) {
	return s.modify(ctx, e.ID, seen, func(prev Event) (Event, error) {
		return Revise(&prev, e)
	})
}

// Publish makes the event public; it fails with what is missing if it is not
// ready. seen is as for UpdateEvent.
func (s *Store) Publish(ctx context.Context, id int64, seen time.Time) (Event, error) {
	return s.modify(ctx, id, seen, func(prev Event) (Event, error) {
		return prev.Publish(s.now())
	})
}

// Unpublish hides a published event. seen is as for UpdateEvent.
func (s *Store) Unpublish(ctx context.Context, id int64, seen time.Time) (Event, error) {
	return s.modify(ctx, id, seen, func(prev Event) (Event, error) {
		return prev.Unpublish(), nil
	})
}

// CopyEvent stores a new draft made from event id; see Event.Copy.
func (s *Store) CopyEvent(ctx context.Context, id int64) (Event, error) {
	src, err := s.Event(ctx, id)
	if err != nil {
		return Event{}, err
	}

	return s.CreateEvent(ctx, src.Copy())
}

// DeleteEvent removes an event with its dates and photo links. Its photos
// stay; those nothing else uses are deleted a week later (DeleteUnusedPhotos)
// unless something takes them up again.
func (s *Store) DeleteEvent(ctx context.Context, id int64) error {
	return s.db.InTx(ctx, func(q *dbsqlc.Queries) error {
		photoIDs, err := q.ListEventPhotoIDs(ctx, id)
		if err != nil {
			return fmt.Errorf("photos of event %d: %w", id, err)
		}

		n, err := q.DeleteEvent(ctx, id)
		if err != nil {
			return fmt.Errorf("deleting event %d: %w", id, err)
		}

		if n == 0 {
			return fmt.Errorf("event %d: %w", id, ErrNotFound)
		}

		return releasePhotos(ctx, q, photoIDs, s.now())
	})
}

// releasePhotos starts the unused grace period of those of ids that nothing
// uses any more, in the transaction that took them out of use: the week
// counts from then, not from the photo's last edit.
func releasePhotos(ctx context.Context, q *dbsqlc.Queries, ids []int64, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}

	if err := q.ReleasePhotos(ctx, dbsqlc.ReleasePhotosParams{UpdatedAt: now.Unix(), Ids: ids}); err != nil {
		return fmt.Errorf("releasing photos %v: %w", ids, err)
	}

	return nil
}

// modify loads event id, lets change produce its next state, and stores it,
// all in one transaction so two admins cannot interleave: the event must
// still be the one the editor saw.
//
// shortcut: updated_at has whole seconds, so a second save within the same
// second as the one the editor loaded passes; add a version counter to
// events if two editors ever meet that.
func (s *Store) modify(ctx context.Context, id int64, seen time.Time, change func(Event) (Event, error)) (Event, error) {
	return write(ctx, s, func(q *dbsqlc.Queries) (Event, error) {
		prev, err := event(ctx, q, id)
		if err != nil {
			return Event{}, err
		}

		if prev.UpdatedAt.Unix() != seen.Unix() {
			return Event{}, fmt.Errorf("event %d saved at %s, editor has %s: %w",
				prev.ID, prev.UpdatedAt.Format(time.RFC3339), seen.UTC().Format(time.RFC3339), ErrStale)
		}

		next, err := change(prev)
		if err != nil {
			return Event{}, err
		}

		next.ID, next.CreatedAt, next.UpdatedAt = prev.ID, prev.CreatedAt, s.now()
		if err := q.UpdateEvent(ctx, updateParams(next)); err != nil {
			return Event{}, eventWriteErr(err, next)
		}

		if err := writeEventChildren(ctx, q, id, next); err != nil {
			return Event{}, err
		}

		return event(ctx, q, id)
	})
}

// writeEventChildren replaces the event's extra dates and photo links.
func writeEventChildren(ctx context.Context, q *dbsqlc.Queries, id int64, e Event) error {
	before, err := q.ListEventPhotoIDs(ctx, id)
	if err != nil {
		return fmt.Errorf("photos of event %d: %w", id, err)
	}

	if err := q.DeleteEventDates(ctx, id); err != nil {
		return fmt.Errorf("clearing dates of event %d: %w", id, err)
	}

	for _, d := range e.ExtraDates {
		if err := q.InsertEventDate(ctx, dbsqlc.InsertEventDateParams{EventID: id, Date: d.String()}); err != nil {
			return fmt.Errorf("adding date %s to event %d: %w", d, id, err)
		}
	}

	if err := q.DeleteEventPhotos(ctx, id); err != nil {
		return fmt.Errorf("clearing photos of event %d: %w", id, err)
	}

	for i, p := range e.photos() {
		role := "extra"
		if i == 0 && e.MainPhoto != nil {
			role = "main"
		}

		err := q.InsertEventPhoto(ctx, dbsqlc.InsertEventPhotoParams{
			EventID: id, PhotoID: p.ID, Role: role, Position: int64(i),
		})
		if db.IsForeignKeyViolation(err) {
			return e.photoErr(p.ID, ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("adding photo %d to event %d: %w", p.ID, id, err)
		}

		if err := writePhotoMeta(ctx, q, e, p); err != nil {
			return err
		}
	}

	return releasePhotos(ctx, q, before, e.UpdatedAt)
}

// writePhotoMeta stores the alt text and credit the event form edits with
// it. An unchanged photo is left alone, so its pages' lastmod stays put.
func writePhotoMeta(ctx context.Context, q *dbsqlc.Queries, e Event, p Photo) error {
	row, err := q.GetPhoto(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("loading photo %d: %w", p.ID, err)
	}

	ref := photographerRef(p.Photographer)
	if row.Alt == p.Alt && row.PhotographerID == ref {
		return nil
	}

	_, err = q.UpdatePhotoMeta(ctx, dbsqlc.UpdatePhotoMetaParams{
		Alt: p.Alt, PhotographerID: ref, UpdatedAt: e.UpdatedAt.Unix(), ID: p.ID,
	})
	if db.IsForeignKeyViolation(err) {
		return e.photoErr(p.ID, fmt.Errorf("photographer %d: %w", ref.Int64, ErrNotFound))
	}
	if err != nil {
		return fmt.Errorf("updating photo %d: %w", p.ID, err)
	}

	return nil
}

func eventWriteErr(err error, e Event) error {
	if db.IsUniqueViolation(err) {
		return fieldErr(FieldSlug, fmt.Errorf("%w: %d/%s", ErrSlugTaken, e.Year, e.Slug))
	}

	return fmt.Errorf("writing event %q: %w", e.Title, err)
}

func event(ctx context.Context, q *dbsqlc.Queries, id int64) (Event, error) {
	return one(ctx, q, fmt.Sprintf("event %d", id), func(ctx context.Context) (dbsqlc.Event, error) {
		return q.GetEvent(ctx, id)
	})
}

func one(ctx context.Context, q *dbsqlc.Queries, what string, get func(context.Context) (dbsqlc.Event, error)) (Event, error) {
	evs, err := events(ctx, q, what, func(ctx context.Context) ([]dbsqlc.Event, error) {
		row, err := get(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", what, ErrNotFound)
		}

		return []dbsqlc.Event{row}, err
	})
	if err != nil {
		return Event{}, err
	}

	return evs[0], nil
}

// events loads rows with their dates and photos. q must be a transaction,
// or a photo saved between its queries could be linked but not loaded.
//
// shortcut: it reads every date, photo link, photo and crop, whatever rows
// asked for. The tables hold a few rows per event and pages are cached by
// render/; filter by event id if an event page ever shows up in profiles.
func events(ctx context.Context, q *dbsqlc.Queries, what string, list func(context.Context) ([]dbsqlc.Event, error)) ([]Event, error) {
	rows, err := list(ctx)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, err
		}

		return nil, fmt.Errorf("%s: %w", what, err)
	}

	dateRows, err := q.ListAllEventDates(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing event dates: %w", err)
	}

	dates := map[int64][]Date{}
	for _, r := range dateRows {
		d, err := ParseDate(r.Date)
		if err != nil {
			return nil, fmt.Errorf("event %d: %w", r.EventID, err)
		}
		dates[r.EventID] = append(dates[r.EventID], d)
	}

	linkRows, err := q.ListAllEventPhotos(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing event photos: %w", err)
	}

	links := map[int64][]dbsqlc.EventPhoto{}
	for _, r := range linkRows {
		links[r.EventID] = append(links[r.EventID], r)
	}

	_, photos, err := allPhotos(ctx, q)
	if err != nil {
		return nil, err
	}

	out := make([]Event, 0, len(rows))
	for _, r := range rows {
		e, err := eventFromRow(r)
		if err != nil {
			return nil, err
		}

		e.ExtraDates = dates[r.ID]
		for _, l := range links[r.ID] {
			p, ok := photos[l.PhotoID]
			if !ok {
				return nil, fmt.Errorf("event %d links photo %d, which is not there", r.ID, l.PhotoID)
			}

			if l.Role == "main" {
				e.MainPhoto = &p
			} else {
				e.ExtraPhotos = append(e.ExtraPhotos, p)
			}
		}

		out = append(out, e)
	}

	return out, nil
}

func eventFromRow(r dbsqlc.Event) (Event, error) {
	e := Event{
		ID:          r.ID,
		Year:        int(r.Year.Int64),
		Slug:        r.Slug,
		Title:       r.Title,
		Summary:     r.Summary,
		BodyMD:      r.BodyMd,
		StartTime:   r.StartTime,
		EndTime:     r.EndTime,
		PriceText:   r.PriceText,
		TicketURL:   r.TicketUrl,
		ShowInMenu:  r.ShowInMenu != 0,
		MenuLabel:   r.MenuLabel,
		Status:      Status(r.Status),
		Cancelled:   r.Cancelled != 0,
		PublishedAt: fromUnixNull(r.PublishedAt),
		CreatedAt:   fromUnix(r.CreatedAt),
		UpdatedAt:   fromUnix(r.UpdatedAt),
	}

	var err error
	if e.Start, err = parseOptionalDate(r.StartDate); err != nil {
		return Event{}, fmt.Errorf("event %d: %w", r.ID, err)
	}

	if e.End, err = parseOptionalDate(r.EndDate); err != nil {
		return Event{}, fmt.Errorf("event %d: %w", r.ID, err)
	}

	return e, nil
}

func createParams(e Event) dbsqlc.CreateEventParams {
	return dbsqlc.CreateEventParams{
		Year:        sql.NullInt64{Int64: int64(e.Year), Valid: e.Year != 0},
		Slug:        e.Slug,
		Title:       e.Title,
		Summary:     e.Summary,
		BodyMd:      e.BodyMD,
		StartDate:   e.Start.String(),
		StartTime:   e.StartTime,
		EndDate:     e.End.String(),
		EndTime:     e.EndTime,
		PriceText:   e.PriceText,
		TicketUrl:   e.TicketURL,
		ShowInMenu:  boolInt(e.ShowInMenu),
		MenuLabel:   e.MenuLabel,
		Status:      string(e.Status),
		Cancelled:   boolInt(e.Cancelled),
		PublishedAt: toUnixNull(e.PublishedAt),
		CreatedAt:   e.CreatedAt.Unix(),
		UpdatedAt:   e.UpdatedAt.Unix(),
	}
}

func updateParams(e Event) dbsqlc.UpdateEventParams {
	c := createParams(e)

	return dbsqlc.UpdateEventParams{
		Year: c.Year, Slug: c.Slug, Title: c.Title, Summary: c.Summary, BodyMd: c.BodyMd,
		StartDate: c.StartDate, StartTime: c.StartTime, EndDate: c.EndDate, EndTime: c.EndTime,
		PriceText: c.PriceText, TicketUrl: c.TicketUrl, ShowInMenu: c.ShowInMenu, MenuLabel: c.MenuLabel,
		Status: c.Status, Cancelled: c.Cancelled, PublishedAt: c.PublishedAt, UpdatedAt: c.UpdatedAt,
		ID: e.ID,
	}
}

// AddPhoto records an uploaded original with the centred crop of every kind
// it is large enough for (the pages show it whole for the rest), and reports
// whether it is new. The same content twice is one photo: the stored one is
// returned as it is, its unused grace period (DeleteUnusedPhotos) restarted,
// since uploading it again means someone is about to use it.
func (s *Store) AddPhoto(ctx context.Context, p Photo) (Photo, bool, error) {
	now := s.now().Unix()
	created := false

	stored, err := write(ctx, s, func(q *dbsqlc.Queries) (Photo, error) {
		id, err := q.GetPhotoIDBySha256(ctx, p.SHA256)
		switch {
		case err == nil:
			if err := q.TouchPhoto(ctx, dbsqlc.TouchPhotoParams{UpdatedAt: now, ID: id}); err != nil {
				return Photo{}, fmt.Errorf("renewing photo %d: %w", id, err)
			}

			return photo(ctx, q, id)
		case !errors.Is(err, sql.ErrNoRows):
			return Photo{}, fmt.Errorf("finding photo %s: %w", p.SHA256, err)
		}

		id, err = q.CreatePhoto(ctx, dbsqlc.CreatePhotoParams{
			Sha256:         p.SHA256,
			Mime:           p.MIME,
			Width:          int64(p.Width),
			Height:         int64(p.Height),
			Alt:            p.Alt,
			PhotographerID: photographerRef(p.Photographer),
			CreatedAt:      now,
			UpdatedAt:      now,
		})
		if db.IsForeignKeyViolation(err) {
			return Photo{}, fmt.Errorf("photographer %d: %w", p.Photographer.ID, ErrNotFound)
		}
		if err != nil {
			return Photo{}, fmt.Errorf("adding photo %s: %w", p.SHA256, err)
		}

		for _, k := range images.CropKinds {
			r, err := images.DefaultCrop(k, p.Width, p.Height)
			if err != nil {
				continue
			}

			if err := upsertCrop(ctx, q, id, k, r); err != nil {
				return Photo{}, err
			}
		}

		created = true

		return photo(ctx, q, id)
	})

	return stored, created, err
}

// Photo loads one photo with its crops.
func (s *Store) Photo(ctx context.Context, id int64) (Photo, error) {
	return read(ctx, s, func(q *dbsqlc.Queries) (Photo, error) {
		return photo(ctx, q, id)
	})
}

func photo(ctx context.Context, q *dbsqlc.Queries, id int64) (Photo, error) {
	row, err := q.GetPhoto(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Photo{}, fmt.Errorf("photo %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Photo{}, fmt.Errorf("loading photo %d: %w", id, err)
	}

	crops, err := q.ListPhotoCrops(ctx, id)
	if err != nil {
		return Photo{}, fmt.Errorf("loading crops of photo %d: %w", id, err)
	}

	p := photoFromRow(row)
	for _, c := range crops {
		p.Crops[images.Kind(c.Kind)] = rectFromRow(c)
	}

	if row.PhotographerID.Valid {
		ph, err := q.GetPhotographer(ctx, row.PhotographerID.Int64)
		if err != nil {
			return Photo{}, fmt.Errorf("loading photographer of photo %d: %w", id, err)
		}
		p.Photographer = new(photographerFromRow(ph))
	}

	return p, nil
}

// Photos lists every photo with its crops, oldest first. images/ derives its
// expected file set from this.
func (s *Store) Photos(ctx context.Context) ([]Photo, error) {
	return read(ctx, s, func(q *dbsqlc.Queries) ([]Photo, error) {
		list, _, err := allPhotos(ctx, q)
		return list, err
	})
}

// UpdatePhoto saves the editable fields of p: alt text and photographer, which
// a nil Photographer clears.
func (s *Store) UpdatePhoto(ctx context.Context, p Photo) (Photo, error) {
	return write(ctx, s, func(q *dbsqlc.Queries) (Photo, error) {
		n, err := q.UpdatePhotoMeta(ctx, dbsqlc.UpdatePhotoMetaParams{
			Alt:            p.Alt,
			PhotographerID: photographerRef(p.Photographer),
			UpdatedAt:      s.now().Unix(),
			ID:             p.ID,
		})
		if db.IsForeignKeyViolation(err) {
			return Photo{}, fmt.Errorf("photographer %d: %w", p.Photographer.ID, ErrNotFound)
		}
		if err != nil {
			return Photo{}, fmt.Errorf("updating photo %d: %w", p.ID, err)
		}

		if n == 0 {
			return Photo{}, fmt.Errorf("photo %d: %w", p.ID, ErrNotFound)
		}

		return photo(ctx, q, p.ID)
	})
}

// Gallery lists the gallery's photos in order.
func (s *Store) Gallery(ctx context.Context) ([]Photo, error) {
	return read(ctx, s, func(q *dbsqlc.Queries) ([]Photo, error) {
		_, byID, err := allPhotos(ctx, q)
		if err != nil {
			return nil, err
		}

		ids, err := q.ListGalleryPhotoIDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing gallery: %w", err)
		}

		out := make([]Photo, 0, len(ids))
		for _, id := range ids {
			p, ok := byID[id]
			if !ok {
				return nil, fmt.Errorf("gallery lists photo %d, which is not there", id)
			}

			out = append(out, p)
		}

		return out, nil
	})
}

// SetGallery replaces the gallery with photoIDs, in that order. A photo it
// leaves out that nothing else uses is deleted a week later
// (DeleteUnusedPhotos) unless something takes it up again.
func (s *Store) SetGallery(ctx context.Context, photoIDs []int64) error {
	now := s.now()

	return s.db.InTx(ctx, func(q *dbsqlc.Queries) error {
		before, err := q.ListGalleryPhotoIDs(ctx)
		if err != nil {
			return fmt.Errorf("listing gallery: %w", err)
		}

		if err := q.DeleteGallery(ctx); err != nil {
			return fmt.Errorf("clearing gallery: %w", err)
		}

		for i, id := range photoIDs {
			err := q.InsertGalleryPhoto(ctx, dbsqlc.InsertGalleryPhotoParams{PhotoID: id, Position: int64(i), UpdatedAt: now.Unix()})
			switch {
			case db.IsForeignKeyViolation(err):
				return fmt.Errorf("photo %d: %w", id, ErrNotFound)
			case db.IsUniqueViolation(err):
				return fmt.Errorf("photo %d is in the gallery twice: %w", id, ErrDuplicatePhoto)
			case err != nil:
				return fmt.Errorf("adding photo %d to gallery: %w", id, err)
			}
		}

		return releasePhotos(ctx, q, before, now)
	})
}

// GalleryUpdatedAt is the last change to the gallery page, for the sitemap;
// zero for an empty gallery.
func (s *Store) GalleryUpdatedAt(ctx context.Context) (time.Time, error) {
	ts, err := s.db.Queries().GalleryUpdatedAt(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("gallery lastmod: %w", err)
	}

	if ts == 0 {
		return time.Time{}, nil
	}

	return fromUnix(ts), nil
}

// Photographers lists every photographer for the admin's picker,
// alphabetically whatever the case.
func (s *Store) Photographers(ctx context.Context) ([]Photographer, error) {
	rows, err := s.db.Queries().ListPhotographers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing photographers: %w", err)
	}

	out := make([]Photographer, 0, len(rows))
	for _, r := range rows {
		out = append(out, photographerFromRow(r))
	}

	return out, nil
}

// Photographer loads one photographer.
func (s *Store) Photographer(ctx context.Context, id int64) (Photographer, error) {
	return photographer(ctx, s.db.Queries(), id)
}

func photographer(ctx context.Context, q *dbsqlc.Queries, id int64) (Photographer, error) {
	r, err := q.GetPhotographer(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Photographer{}, fmt.Errorf("photographer %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Photographer{}, fmt.Errorf("loading photographer %d: %w", id, err)
	}

	return photographerFromRow(r), nil
}

// CreatePhotographer stores a new photographer; see Photographer.Normalize.
// A name already used, whatever its case or spacing, is ErrPhotographerTaken.
func (s *Store) CreatePhotographer(ctx context.Context, p Photographer) (Photographer, error) {
	p, err := p.Normalize()
	if err != nil {
		return Photographer{}, err
	}

	return write(ctx, s, func(q *dbsqlc.Queries) (Photographer, error) {
		id, err := q.CreatePhotographer(ctx, dbsqlc.CreatePhotographerParams{
			Name: p.Name, NameKey: nameKey(p.Name), Url: p.URL,
		})
		if db.IsUniqueViolation(err) {
			return Photographer{}, fmt.Errorf("%w: %q", ErrPhotographerTaken, p.Name)
		}
		if err != nil {
			return Photographer{}, fmt.Errorf("creating photographer %q: %w", p.Name, err)
		}

		return photographer(ctx, q, id)
	})
}

// FindOrCreatePhotographer returns the photographer named p.Name, creating it
// from p if there is none. A found one keeps its link, so an import never
// undoes a correction made in the admin.
func (s *Store) FindOrCreatePhotographer(ctx context.Context, p Photographer) (Photographer, error) {
	p, err := p.Normalize()
	if err != nil {
		return Photographer{}, err
	}

	return write(ctx, s, func(q *dbsqlc.Queries) (Photographer, error) {
		id, err := q.EnsurePhotographer(ctx, dbsqlc.EnsurePhotographerParams{
			Name: p.Name, NameKey: nameKey(p.Name), Url: p.URL,
		})
		if err != nil {
			return Photographer{}, fmt.Errorf("ensuring photographer %q: %w", p.Name, err)
		}

		return photographer(ctx, q, id)
	})
}

// UpdatePhotographer saves p's name and link. The photos crediting p count as
// changed, since every page showing them does.
func (s *Store) UpdatePhotographer(ctx context.Context, p Photographer) (Photographer, error) {
	p, err := p.Normalize()
	if err != nil {
		return Photographer{}, err
	}

	return write(ctx, s, func(q *dbsqlc.Queries) (Photographer, error) {
		n, err := q.UpdatePhotographer(ctx, dbsqlc.UpdatePhotographerParams{
			Name: p.Name, NameKey: nameKey(p.Name), Url: p.URL, ID: p.ID,
		})
		switch {
		case db.IsUniqueViolation(err):
			return Photographer{}, fmt.Errorf("%w: %q", ErrPhotographerTaken, p.Name)
		case err != nil:
			return Photographer{}, fmt.Errorf("updating photographer %d: %w", p.ID, err)
		case n == 0:
			return Photographer{}, fmt.Errorf("photographer %d: %w", p.ID, ErrNotFound)
		}

		err = q.TouchPhotographerPhotos(ctx, dbsqlc.TouchPhotographerPhotosParams{
			UpdatedAt: s.now().Unix(), PhotographerID: photographerRef(&p),
		})
		if err != nil {
			return Photographer{}, fmt.Errorf("touching photos of photographer %d: %w", p.ID, err)
		}

		return photographer(ctx, q, p.ID)
	})
}

// DeletePhotographer removes a photographer no photo credits.
func (s *Store) DeletePhotographer(ctx context.Context, id int64) error {
	n, err := s.db.Queries().DeletePhotographer(ctx, id)
	if db.IsForeignKeyViolation(err) {
		return fmt.Errorf("photographer %d: %w", id, ErrPhotographerInUse)
	}
	if err != nil {
		return fmt.Errorf("deleting photographer %d: %w", id, err)
	}

	if n == 0 {
		return fmt.Errorf("photographer %d: %w", id, ErrNotFound)
	}

	return nil
}

// allPhotos loads every photo with its crops and photographer, by id order
// and by id. q must be a transaction; see mergePhotos.
func allPhotos(ctx context.Context, q *dbsqlc.Queries) ([]Photo, map[int64]Photo, error) {
	rows, err := q.ListPhotos(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listing photos: %w", err)
	}

	photographers, err := q.ListPhotographers(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listing photographers: %w", err)
	}

	crops, err := q.ListAllPhotoCrops(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("listing crops: %w", err)
	}

	return mergePhotos(rows, photographers, crops)
}

// mergePhotos puts photos together from their tables' rows. Rows read
// outside one transaction can disagree (a crop of a photo saved after the
// photos were listed); that is an error here, never a crash.
func mergePhotos(rows []dbsqlc.Photo, photographerRows []dbsqlc.Photographer, crops []dbsqlc.PhotoCrop) ([]Photo, map[int64]Photo, error) {
	photographers := make(map[int64]Photographer, len(photographerRows))
	for _, r := range photographerRows {
		photographers[r.ID] = photographerFromRow(r)
	}

	list := make([]Photo, 0, len(rows))
	byID := make(map[int64]Photo, len(rows))
	for _, r := range rows {
		p := photoFromRow(r)
		if r.PhotographerID.Valid {
			ph, ok := photographers[r.PhotographerID.Int64]
			if !ok {
				return nil, nil, fmt.Errorf("photo %d credits photographer %d, who is not there", r.ID, r.PhotographerID.Int64)
			}
			p.Photographer = &ph
		}
		list = append(list, p)
		byID[p.ID] = p
	}

	// Crops is a map, so the list and byID share each photo's crops.
	for _, c := range crops {
		p, ok := byID[c.PhotoID]
		if !ok {
			return nil, nil, fmt.Errorf("crop %s of photo %d, which is not there", c.Kind, c.PhotoID)
		}
		p.Crops[images.Kind(c.Kind)] = rectFromRow(c)
	}

	return list, byID, nil
}

func photoFromRow(r dbsqlc.Photo) Photo {
	return Photo{
		ID:        r.ID,
		SHA256:    r.Sha256,
		MIME:      r.Mime,
		Width:     int(r.Width),
		Height:    int(r.Height),
		Alt:       r.Alt,
		Crops:     map[images.Kind]images.Rect{},
		CreatedAt: fromUnix(r.CreatedAt),
		UpdatedAt: fromUnix(r.UpdatedAt),
	}
}

func photographerFromRow(r dbsqlc.Photographer) Photographer {
	return Photographer{ID: r.ID, Name: r.Name, URL: r.Url}
}

func photographerRef(p *Photographer) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: p.ID, Valid: true}
}

func rectFromRow(c dbsqlc.PhotoCrop) images.Rect {
	return images.Rect{X: int(c.X), Y: int(c.Y), W: int(c.W), H: int(c.H)}
}

func parseOptionalDate(s string) (Date, error) {
	if s == "" {
		return Date{}, nil
	}

	return ParseDate(s)
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}

	return 0
}

func fromUnix(ts int64) time.Time { return time.Unix(ts, 0).UTC() }

func fromUnixNull(ts sql.NullInt64) time.Time {
	if !ts.Valid {
		return time.Time{}
	}

	return fromUnix(ts.Int64)
}

func toUnixNull(t time.Time) sql.NullInt64 {
	return sql.NullInt64{Int64: t.Unix(), Valid: !t.IsZero()}
}
