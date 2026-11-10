package site

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/db"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newTestStore(t *testing.T, start time.Time) (*Store, *fakeClock) {
	t.Helper()

	d, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	clock := &fakeClock{t: start}

	return NewStore(d, clock.now), clock
}

func sha(c byte) string { return strings.Repeat(string(c), 64) }

func addPhoto(t *testing.T, s *Store, c byte, alt string) Photo {
	t.Helper()

	p, err := s.AddPhoto(t.Context(), Photo{SHA256: sha(c), MIME: "image/jpeg", Width: 4000, Height: 3000, Alt: alt})
	require.NoError(t, err)

	return p
}

func TestStoreEventRoundTrip(t *testing.T) {
	start := oslo("2026-10-10 12:00")
	s, _ := newTestStore(t, start)
	ctx := t.Context()

	main := addPhoto(t, s, 'a', "Anita Saga ved bakebordet")
	extra := addPhoto(t, s, 'b', "Ferske takker")

	in := Event{
		Title:       "Takkebakstkurs med Anita Saga",
		Summary:     "Lær å bake takker.",
		BodyMD:      "**Velkommen!**",
		Start:       d("2026-10-31"),
		StartTime:   "10:00",
		End:         d("2026-10-31"),
		EndTime:     "15:00",
		ExtraDates:  []Date{d("2026-11-29"), d("2026-11-28")},
		PriceText:   "950 kr",
		TicketURL:   "https://example.com/billett",
		ShowInMenu:  true,
		MenuLabel:   "Kurs",
		Cancelled:   true,
		MainPhoto:   &main,
		ExtraPhotos: []Photo{extra},
	}

	got, err := s.CreateEvent(ctx, in)
	require.NoError(t, err)

	want := in
	want.ID = got.ID
	want.Year = 2026
	want.Slug = "takkebakstkurs-med-anita-saga"
	want.Status = Draft
	want.ExtraDates = []Date{d("2026-11-28"), d("2026-11-29")}
	want.CreatedAt = start.Truncate(time.Second).UTC()
	want.UpdatedAt = want.CreatedAt
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("CreateEvent() mismatch (-want +got):\n%s", diff)
	}

	loaded, err := s.Event(ctx, got.ID)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(got, loaded))

	all, err := s.Events(ctx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Empty(t, cmp.Diff(got, all[0]))
}

func TestStorePublishLifecycle(t *testing.T) {
	s, clock := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()

	e, err := s.CreateEvent(ctx, Event{Title: "Julemarked", Start: d("2026-11-14"), End: d("2026-11-15")})
	require.NoError(t, err)

	_, err = s.Publish(ctx, e.ID)
	require.ErrorIs(t, err, ErrMainPhotoRequired)

	p := addPhoto(t, s, 'a', "")
	e.MainPhoto = &p
	e, err = s.UpdateEvent(ctx, e)
	require.NoError(t, err)

	_, err = s.Publish(ctx, e.ID)
	require.ErrorIs(t, err, ErrAltRequired)

	p.Alt = "Bålfat på tunet"
	_, err = s.UpdatePhoto(ctx, p)
	require.NoError(t, err)

	_, err = s.PublishedEvent(ctx, 2026, "julemarked")
	require.ErrorIs(t, err, ErrNotFound, "a draft is not public")

	clock.t = oslo("2026-10-11 09:00")
	pub, err := s.Publish(ctx, e.ID)
	require.NoError(t, err)
	require.Equal(t, Published, pub.Status)
	require.Equal(t, clock.t.UTC(), pub.PublishedAt)
	require.Equal(t, clock.t.UTC(), pub.UpdatedAt)

	got, err := s.PublishedEvent(ctx, 2026, "julemarked")
	require.NoError(t, err)
	require.Equal(t, pub.ID, got.ID)

	// The address is frozen now.
	pub.Slug = "julemarked-2026"
	_, err = s.UpdateEvent(ctx, pub)
	require.ErrorIs(t, err, ErrSlugFrozen)

	// An edit that would break the live page is refused.
	pub.Slug = ""
	pub.Title = ""
	_, err = s.UpdateEvent(ctx, pub)
	require.ErrorIs(t, err, ErrTitleRequired)

	_, err = s.Unpublish(ctx, e.ID)
	require.NoError(t, err)
	_, err = s.PublishedEvent(ctx, 2026, "julemarked")
	require.ErrorIs(t, err, ErrNotFound)

	again, err := s.Publish(ctx, e.ID)
	require.NoError(t, err)
	require.Equal(t, pub.PublishedAt, again.PublishedAt, "first publish time is kept")

	require.NoError(t, s.DeleteEvent(ctx, e.ID))
	require.ErrorIs(t, s.DeleteEvent(ctx, e.ID), ErrNotFound)
	_, err = s.Event(ctx, e.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

// The yearly julemarked: copy last year's, give it a date, publish, and
// /julemarked moves to the new one.
func TestStoreCopyAndEvergreen(t *testing.T) {
	s, clock := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()

	p := addPhoto(t, s, 'a', "Bålfat på tunet")
	e, err := s.CreateEvent(ctx, Event{
		Title: "Julemarked på Aspargesgården", Slug: "julemarked",
		Start: d("2026-11-14"), End: d("2026-11-15"), StartTime: "12:00", EndTime: "16:00",
		ShowInMenu: true, MenuLabel: "Julemarked", MainPhoto: &p,
	})
	require.NoError(t, err)
	e, err = s.Publish(ctx, e.ID)
	require.NoError(t, err)

	newest, err := s.NewestPublishedEvent(ctx, "julemarked")
	require.NoError(t, err)
	require.Equal(t, e.ID, newest.ID)

	cp, err := s.CopyEvent(ctx, e.ID)
	require.NoError(t, err)
	require.NotEqual(t, e.ID, cp.ID)
	require.Equal(t, Draft, cp.Status)
	require.Equal(t, "julemarked", cp.Slug)
	require.Zero(t, cp.Year)
	require.True(t, cp.Start.IsZero())
	require.Equal(t, p.ID, cp.MainPhoto.ID)

	// Same slug in the same year is taken.
	cp.Start = d("2026-12-05")
	_, err = s.UpdateEvent(ctx, cp)
	require.ErrorIs(t, err, ErrSlugTaken)

	cp.Start, cp.End = d("2027-11-13"), d("2027-11-14")
	cp, err = s.UpdateEvent(ctx, cp)
	require.NoError(t, err)
	require.Equal(t, 2027, cp.Year)

	// A second copy cannot take the same year either.
	cp2, err := s.CopyEvent(ctx, e.ID)
	require.NoError(t, err)
	cp2.Start = d("2027-12-01")
	_, err = s.UpdateEvent(ctx, cp2)
	require.ErrorIs(t, err, ErrSlugTaken)

	_, err = s.Publish(ctx, cp.ID)
	require.NoError(t, err)

	newest, err = s.NewestPublishedEvent(ctx, "julemarked")
	require.NoError(t, err)
	require.Equal(t, cp.ID, newest.ID)
	require.Equal(t, "/arrangementer/2027/julemarked", newest.Path())

	_, err = s.NewestPublishedEvent(ctx, "sommerfest")
	require.ErrorIs(t, err, ErrNotFound)

	// Both markets are in the menu until each is over.
	menu, err := s.Menu(ctx)
	require.NoError(t, err)
	require.Equal(t, []MenuEntry{
		{Label: "Julemarked", Path: "/arrangementer/2026/julemarked"},
		{Label: "Julemarked", Path: "/arrangementer/2027/julemarked"},
	}, menu)

	clock.t = oslo("2026-11-16 00:00")
	menu, err = s.Menu(ctx)
	require.NoError(t, err)
	require.Equal(t, []MenuEntry{{Label: "Julemarked", Path: "/arrangementer/2027/julemarked"}}, menu)

	published, err := s.PublishedEvents(ctx)
	require.NoError(t, err)
	upcoming, past := Split(published, clock.t)
	require.Equal(t, []int64{cp.ID}, ids(upcoming))
	require.Equal(t, []int64{e.ID}, ids(past))
}

func TestStoreEventPhotoErrors(t *testing.T) {
	s, _ := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()

	_, err := s.CreateEvent(ctx, Event{Title: "Kurs", MainPhoto: &Photo{ID: 42}})
	require.ErrorIs(t, err, ErrNotFound)

	events, err := s.Events(ctx)
	require.NoError(t, err)
	require.Empty(t, events, "a failed create leaves nothing behind")
}

func TestStorePhotos(t *testing.T) {
	start := oslo("2026-10-10 12:00")
	s, clock := newTestStore(t, start)
	ctx := t.Context()

	a := addPhoto(t, s, 'a', "")
	require.Equal(t, a, addPhoto(t, s, 'a', "ignored"), "same content is one photo")
	b := addPhoto(t, s, 'b', "")

	clock.t = start.Add(time.Hour)
	require.ErrorIs(t, s.SetCrop(ctx, a.ID, CropCard, Rect{X: 3000, W: 2400, H: 3000}), ErrCropOutside)
	require.ErrorIs(t, s.SetCrop(ctx, a.ID, "full", Rect{W: 1, H: 1}), ErrCropKind)
	require.ErrorIs(t, s.SetCrop(ctx, 999, CropCard, Rect{W: 1, H: 1}), ErrNotFound)
	require.NoError(t, s.SetCrop(ctx, a.ID, CropCard, Rect{X: 800, W: 2400, H: 3000}))
	require.NoError(t, s.SetCrop(ctx, a.ID, CropShare, Rect{Y: 450, W: 4000, H: 2100}))

	a, err := s.Photo(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, map[CropKind]Rect{
		CropCard:  {X: 800, W: 2400, H: 3000},
		CropShare: {Y: 450, W: 4000, H: 2100},
	}, a.Crops)
	require.Equal(t, clock.t.UTC(), a.UpdatedAt, "a crop change is a change of the photo")

	lucky, err := s.CreatePhotographer(ctx, Photographer{Name: "Lucky Thirteen", URL: "https://luckythirteen.no/"})
	require.NoError(t, err)

	a.Alt = "Låven en vinterkveld"
	a.Photographer = &lucky
	updated, err := s.UpdatePhoto(ctx, a)
	require.NoError(t, err)
	require.Equal(t, "Låven en vinterkveld", updated.Alt)
	require.Equal(t, &lucky, updated.Photographer)

	_, err = s.UpdatePhoto(ctx, Photo{ID: 999})
	require.ErrorIs(t, err, ErrNotFound)
	_, err = s.UpdatePhoto(ctx, Photo{ID: a.ID, Photographer: &Photographer{ID: 999}})
	require.ErrorIs(t, err, ErrNotFound, "unknown photographer")

	photos, err := s.Photos(ctx)
	require.NoError(t, err)
	require.Len(t, photos, 2)
	require.Empty(t, cmp.Diff(updated, photos[0]))

	_, err = s.Photo(ctx, 999)
	require.ErrorIs(t, err, ErrNotFound)

	// In use by an event: cannot be deleted until the event is gone.
	e, err := s.CreateEvent(ctx, Event{Title: "Kurs", MainPhoto: &b})
	require.NoError(t, err)
	require.ErrorIs(t, s.DeletePhoto(ctx, b.ID), ErrPhotoInUse)
	require.NoError(t, s.DeleteEvent(ctx, e.ID))
	require.NoError(t, s.DeletePhoto(ctx, b.ID))
	require.ErrorIs(t, s.DeletePhoto(ctx, b.ID), ErrNotFound)
}

func TestStoreGallery(t *testing.T) {
	start := oslo("2026-10-10 12:00")
	s, clock := newTestStore(t, start)
	ctx := t.Context()

	at, err := s.GalleryUpdatedAt(ctx)
	require.NoError(t, err)
	require.True(t, at.IsZero())

	a := addPhoto(t, s, 'a', "Låven")
	b := addPhoto(t, s, 'b', "Fjøset")
	c := addPhoto(t, s, 'c', "Hagen")

	clock.t = start.Add(time.Hour)
	require.NoError(t, s.SetGallery(ctx, []int64{c.ID, a.ID, b.ID}))

	gallery, err := s.Gallery(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{c.ID, a.ID, b.ID}, photoIDs(gallery))

	at, err = s.GalleryUpdatedAt(ctx)
	require.NoError(t, err)
	require.Equal(t, clock.t.UTC(), at)

	// A photo edit moves the gallery's lastmod too.
	clock.t = start.Add(2 * time.Hour)
	a.Alt = "Låven i snø"
	_, err = s.UpdatePhoto(ctx, a)
	require.NoError(t, err)
	at, err = s.GalleryUpdatedAt(ctx)
	require.NoError(t, err)
	require.Equal(t, clock.t.UTC(), at)

	require.ErrorIs(t, s.DeletePhoto(ctx, a.ID), ErrPhotoInUse)

	require.ErrorIs(t, s.SetGallery(ctx, []int64{a.ID, a.ID}), ErrDuplicatePhoto)
	require.ErrorIs(t, s.SetGallery(ctx, []int64{a.ID, 999}), ErrNotFound)
	gallery, err = s.Gallery(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{c.ID, a.ID, b.ID}, photoIDs(gallery), "a failed reorder changes nothing")

	require.NoError(t, s.SetGallery(ctx, []int64{b.ID}))
	require.NoError(t, s.DeletePhoto(ctx, a.ID))
	gallery, err = s.Gallery(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{b.ID}, photoIDs(gallery))
}

func TestStorePhotographers(t *testing.T) {
	start := oslo("2026-10-10 12:00")
	s, clock := newTestStore(t, start)
	ctx := t.Context()

	anita, err := s.CreatePhotographer(ctx, Photographer{Name: " Anita  Saga "})
	require.NoError(t, err)
	require.Equal(t, Photographer{ID: anita.ID, Name: "Anita Saga"}, anita)

	_, err = s.CreatePhotographer(ctx, Photographer{Name: "anita SAGA"})
	require.ErrorIs(t, err, ErrPhotographerTaken)
	_, err = s.CreatePhotographer(ctx, Photographer{Name: ""})
	require.ErrorIs(t, err, ErrPhotographerName)
	_, err = s.CreatePhotographer(ctx, Photographer{Name: "Ola", URL: "javascript:alert(1)"})
	require.ErrorIs(t, err, ErrPhotographerURL)

	oyvind, err := s.FindOrCreatePhotographer(ctx, Photographer{Name: "Øyvind Ås", URL: "https://example.com/"})
	require.NoError(t, err)
	again, err := s.FindOrCreatePhotographer(ctx, Photographer{Name: "øyvind ås", URL: "https://other.example/"})
	require.NoError(t, err)
	require.Equal(t, oyvind, again, "a known name is returned unchanged")
	_, err = s.FindOrCreatePhotographer(ctx, Photographer{Name: " "})
	require.ErrorIs(t, err, ErrPhotographerName)

	lucky, err := s.CreatePhotographer(ctx, Photographer{Name: "Lucky Thirteen"})
	require.NoError(t, err)

	list, err := s.Photographers(ctx)
	require.NoError(t, err)
	require.Equal(t, []Photographer{anita, lucky, oyvind}, list, "alphabetical, whatever the case")

	// Credited on a gallery photo and an event photo, the credit comes back on
	// the public read paths.
	p := addPhoto(t, s, 'a', "Låven")
	p.Photographer = &anita
	p, err = s.UpdatePhoto(ctx, p)
	require.NoError(t, err)
	require.NoError(t, s.SetGallery(ctx, []int64{p.ID}))
	e, err := s.CreateEvent(ctx, Event{Title: "Kurs", MainPhoto: &p})
	require.NoError(t, err)
	require.Equal(t, &anita, e.MainPhoto.Photographer)

	// One correction fixes every credit, and moves the gallery's lastmod.
	clock.t = start.Add(time.Hour)
	anita.URL = "https://anitasaga.no/"
	anita, err = s.UpdatePhotographer(ctx, anita)
	require.NoError(t, err)

	gallery, err := s.Gallery(ctx)
	require.NoError(t, err)
	require.Equal(t, &anita, gallery[0].Photographer)
	e, err = s.Event(ctx, e.ID)
	require.NoError(t, err)
	require.Equal(t, &anita, e.MainPhoto.Photographer)
	at, err := s.GalleryUpdatedAt(ctx)
	require.NoError(t, err)
	require.Equal(t, clock.t.UTC(), at)

	_, err = s.UpdatePhotographer(ctx, Photographer{ID: anita.ID, Name: "LUCKY thirteen"})
	require.ErrorIs(t, err, ErrPhotographerTaken)
	_, err = s.UpdatePhotographer(ctx, Photographer{ID: anita.ID, Name: "Anita", URL: "ftp://x"})
	require.ErrorIs(t, err, ErrPhotographerURL)
	_, err = s.UpdatePhotographer(ctx, Photographer{ID: 999, Name: "Ola"})
	require.ErrorIs(t, err, ErrNotFound)

	got, err := s.Photographer(ctx, anita.ID)
	require.NoError(t, err)
	require.Equal(t, anita, got, "failed updates change nothing")
	_, err = s.Photographer(ctx, 999)
	require.ErrorIs(t, err, ErrNotFound)

	require.ErrorIs(t, s.DeletePhotographer(ctx, anita.ID), ErrPhotographerInUse)
	p.Photographer = nil
	p, err = s.UpdatePhoto(ctx, p)
	require.NoError(t, err)
	require.Nil(t, p.Photographer)
	require.NoError(t, s.DeletePhotographer(ctx, anita.ID))
	require.ErrorIs(t, s.DeletePhotographer(ctx, anita.ID), ErrNotFound)

	// A photo can be credited from the start.
	b, err := s.AddPhoto(ctx, Photo{SHA256: sha('b'), MIME: "image/jpeg", Width: 10, Height: 10, Photographer: &lucky})
	require.NoError(t, err)
	require.Equal(t, &lucky, b.Photographer)
	_, err = s.AddPhoto(ctx, Photo{SHA256: sha('c'), MIME: "image/jpeg", Width: 10, Height: 10, Photographer: &Photographer{ID: 999}})
	require.ErrorIs(t, err, ErrNotFound)
}

func photoIDs(photos []Photo) []int64 {
	out := make([]int64, 0, len(photos))
	for _, p := range photos {
		out = append(out, p.ID)
	}

	return out
}
