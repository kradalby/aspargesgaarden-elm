package site

import (
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
)

func TestStoreSeenPreconditions(t *testing.T) {
	s, clock := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()

	main := addPhoto(t, s, 'a', "Boder i låven")
	e, err := s.CreateEvent(ctx, Event{Title: "Julemarked", Start: d("2026-11-14"), MainPhoto: &main})
	require.NoError(t, err)

	loaded := e.UpdatedAt
	clock.t = clock.t.Add(time.Minute)

	// Someone else saves first.
	theirs := e
	theirs.PriceText = "Gratis"
	_, err = s.UpdateEvent(ctx, theirs, loaded)
	require.NoError(t, err)
	clock.t = clock.t.Add(time.Minute)

	mine := e
	mine.PriceText = "50 kr"
	_, err = s.UpdateEvent(ctx, mine, loaded)
	require.ErrorIs(t, err, ErrStale)
	_, err = s.Publish(ctx, e.ID, loaded)
	require.ErrorIs(t, err, ErrStale)
	_, err = s.Unpublish(ctx, e.ID, loaded)
	require.ErrorIs(t, err, ErrStale)

	stored, err := s.Event(ctx, e.ID)
	require.NoError(t, err)
	require.Equal(t, "Gratis", stored.PriceText, "a stale save writes nothing")

	pub, err := s.Publish(ctx, e.ID, stored.UpdatedAt)
	require.NoError(t, err)
	require.Equal(t, Published, pub.Status)

	clock.t = clock.t.Add(time.Minute)
	hidden, err := s.Unpublish(ctx, e.ID, pub.UpdatedAt)
	require.NoError(t, err)
	require.Equal(t, Draft, hidden.Status)
}

func TestStoreEventSavesPhotoMeta(t *testing.T) {
	s, _ := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()

	ola, err := s.CreatePhotographer(ctx, Photographer{Name: "Ola Nordmann"})
	require.NoError(t, err)

	main := addPhoto(t, s, 'a', "")
	main.Alt, main.Photographer = "Boder i låven", &ola

	_, err = s.CreateEvent(ctx, Event{Title: "Julemarked", MainPhoto: &main})
	require.NoError(t, err)

	got, err := s.Photo(ctx, main.ID)
	require.NoError(t, err)
	require.Equal(t, "Boder i låven", got.Alt)
	require.Equal(t, &ola, got.Photographer)

	extra := addPhoto(t, s, 'b', "")
	extra.Photographer = &Photographer{ID: 999}
	_, err = s.CreateEvent(ctx, Event{Title: "Kurs", ExtraPhotos: []Photo{extra}})
	require.ErrorIs(t, err, ErrNotFound)

	var fe *FieldError
	require.ErrorAs(t, err, &fe)
	require.Equal(t, FieldError{Field: FieldExtraPhotos, PhotoID: extra.ID}, FieldError{Field: fe.Field, PhotoID: fe.PhotoID})
}

// Events share photos after "Lag kopi". Saving one must not undo a
// description written in the other since the editor loaded hers: the photo
// carries the version she saw, as the event does.
func TestStoreEventKeepsPhotoEditsMadeElsewhere(t *testing.T) {
	s, clock := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()

	shared := addPhoto(t, s, 'a', "Boder")
	a, err := s.CreateEvent(ctx, Event{Title: "Julemarked", MainPhoto: &shared})
	require.NoError(t, err)
	b, err := s.CopyEvent(ctx, a.ID)
	require.NoError(t, err)

	// A's editor describes the photo better.
	clock.t = clock.t.Add(time.Minute)
	a.MainPhoto.Alt = "Boder med julepynt i låven"
	a, err = s.UpdateEvent(ctx, a, a.UpdatedAt)
	require.NoError(t, err)

	// B's editor, who loaded before that, saves her own change.
	clock.t = clock.t.Add(time.Minute)
	b.Summary = "Neste års marked."
	_, err = s.UpdateEvent(ctx, b, b.UpdatedAt)
	require.ErrorIs(t, err, ErrStale)

	var fe *FieldError
	require.ErrorAs(t, err, &fe)
	require.Equal(t, FieldError{Field: FieldMainPhoto, PhotoID: shared.ID}, FieldError{Field: fe.Field, PhotoID: fe.PhotoID})

	got, err := s.Photo(ctx, shared.ID)
	require.NoError(t, err)
	require.Equal(t, "Boder med julepynt i låven", got.Alt, "a stale save writes nothing")

	// Loaded afresh, the save goes through and keeps the new description.
	b, err = s.Event(ctx, b.ID)
	require.NoError(t, err)
	b.Summary = "Neste års marked."
	_, err = s.UpdateEvent(ctx, b, b.UpdatedAt)
	require.NoError(t, err)

	got, err = s.Photo(ctx, shared.ID)
	require.NoError(t, err)
	require.Equal(t, "Boder med julepynt i låven", got.Alt)

	// "Keep mine" sends hers again with the photo's current version.
	clock.t = clock.t.Add(time.Minute)
	b, err = s.Event(ctx, b.ID)
	require.NoError(t, err)
	b.MainPhoto.Alt = "Boder"
	b, err = s.UpdateEvent(ctx, b, b.UpdatedAt)
	require.NoError(t, err)
	require.Equal(t, "Boder", b.MainPhoto.Alt)

	// A photo that is sent as stored is not a change, whatever its version.
	a.Summary = "Velkommen."
	a.MainPhoto.Alt = "Boder"
	_, err = s.UpdateEvent(ctx, a, a.UpdatedAt)
	require.NoError(t, err)

	// A caller that does not know the version (an older app) is not checked.
	clock.t = clock.t.Add(time.Minute)
	a, err = s.Event(ctx, a.ID)
	require.NoError(t, err)
	a.MainPhoto.Alt, a.MainPhoto.UpdatedAt = "Låven", time.Time{}
	_, err = s.UpdateEvent(ctx, a, a.UpdatedAt)
	require.NoError(t, err)
}

func TestStoreSetCrops(t *testing.T) {
	s, _ := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()
	p := addPhoto(t, s, 'a', "x")

	good := images.Rect{X: 0, Y: 0, W: 2400, H: 3000}
	_, err := s.SetCrops(ctx, p.ID, map[images.Kind]images.Rect{images.Card: good, images.Banner: {X: 0, Y: 0, W: 5000, H: 100}})
	require.ErrorIs(t, err, images.ErrCropOutside)

	var fe *FieldError
	require.ErrorAs(t, err, &fe)
	require.Equal(t, Field("banner"), fe.Field)

	got, err := s.Photo(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, p.Crops, got.Crops, "one bad crop writes none")

	want := maps.Clone(p.Crops)
	want[images.Card] = good
	saved, err := s.SetCrops(ctx, p.ID, map[images.Kind]images.Rect{images.Card: good})
	require.NoError(t, err)
	require.Equal(t, want, saved.Crops)
	got, err = s.Photo(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, saved, got)

	_, err = s.SetCrops(ctx, 999, map[images.Kind]images.Rect{images.Card: good})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestStoreRemovePhoto(t *testing.T) {
	s, _ := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()

	inGallery := addPhoto(t, s, 'a', "Hagen")
	inEvent := addPhoto(t, s, 'b', "Boder")
	require.NoError(t, s.SetGallery(ctx, []int64{inGallery.ID, inEvent.ID}))

	e, err := s.CreateEvent(ctx, Event{Title: "Julemarked", MainPhoto: &inEvent})
	require.NoError(t, err)

	usedBy, err := s.RemovePhoto(ctx, inEvent.ID)
	require.ErrorIs(t, err, ErrPhotoInUse)
	require.Equal(t, []Event{{ID: e.ID, Title: "Julemarked"}}, usedBy)

	gallery, err := s.Gallery(ctx)
	require.NoError(t, err)
	require.Len(t, gallery, 2, "a refused delete leaves the gallery alone")

	usedBy, err = s.RemovePhoto(ctx, inGallery.ID)
	require.NoError(t, err)
	require.Empty(t, usedBy)

	gallery, err = s.Gallery(ctx)
	require.NoError(t, err)
	require.Equal(t, []int64{inEvent.ID}, photoIDs(gallery))

	_, err = s.RemovePhoto(ctx, inGallery.ID)
	require.ErrorIs(t, err, ErrNotFound)
}

// Deleting a gallery photo changes the gallery page, so its lastmod moves,
// though no row left in the gallery was edited.
func TestStoreRemovePhotoMovesGalleryLastmod(t *testing.T) {
	start := oslo("2026-10-10 12:00")
	s, clock := newTestStore(t, start)
	ctx := t.Context()

	a := addPhoto(t, s, 'a', "Hagen")
	b := addPhoto(t, s, 'b', "Låven")
	other := addPhoto(t, s, 'c', "Tunet")
	require.NoError(t, s.SetGallery(ctx, []int64{a.ID, b.ID}))

	clock.t = start.Add(time.Hour)
	_, err := s.RemovePhoto(ctx, other.ID)
	require.NoError(t, err)
	at, err := s.GalleryUpdatedAt(ctx)
	require.NoError(t, err)
	require.Equal(t, start.UTC(), at, "a photo outside the gallery is no change of it")

	clock.t = start.Add(2 * time.Hour)
	_, err = s.RemovePhoto(ctx, a.ID)
	require.NoError(t, err)
	at, err = s.GalleryUpdatedAt(ctx)
	require.NoError(t, err)
	require.Equal(t, clock.t.UTC(), at)
}

func TestStoreDeleteUnusedPhotos(t *testing.T) {
	const grace = 7 * 24 * time.Hour

	start := oslo("2026-10-01 12:00")
	s, clock := newTestStore(t, start)
	ctx := t.Context()

	unused := addPhoto(t, s, 'a', "")
	renewed := addPhoto(t, s, 'b', "")
	inGallery := addPhoto(t, s, 'c', "Hagen")
	inEvent := addPhoto(t, s, 'd', "Boder")
	require.NoError(t, s.SetGallery(ctx, []int64{inGallery.ID}))
	_, err := s.CreateEvent(ctx, Event{Title: "Kladd", MainPhoto: &inEvent})
	require.NoError(t, err)

	// Uploaded again on day six: someone is using it right now.
	clock.t = start.Add(6 * 24 * time.Hour)
	addPhoto(t, s, 'b', "")

	clock.t = start.Add(grace - time.Second)
	gone, err := s.DeleteUnusedPhotos(ctx, grace)
	require.NoError(t, err)
	require.Empty(t, gone, "nothing is older than the grace period yet")

	clock.t = start.Add(grace + time.Second)
	gone, err = s.DeleteUnusedPhotos(ctx, grace)
	require.NoError(t, err)
	require.Equal(t, []string{unused.SHA256}, gone)

	_, err = s.Photo(ctx, unused.ID)
	require.ErrorIs(t, err, ErrNotFound)

	for _, p := range []Photo{renewed, inGallery, inEvent} {
		_, err := s.Photo(ctx, p.ID)
		require.NoError(t, err, "photo %d", p.ID)
	}

	clock.t = start.Add(6*24*time.Hour + grace + time.Second)
	gone, err = s.DeleteUnusedPhotos(ctx, grace)
	require.NoError(t, err)
	require.Equal(t, []string{renewed.SHA256}, gone)
}

// The gallery's own photo route may leave a description empty, but never on
// a photo a published event shows: no edit breaks a live page.
func TestStoreUpdatePhotoKeepsLivePagesDescribed(t *testing.T) {
	s, _ := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()

	live := addPhoto(t, s, 'a', "Boder i låven")
	inDraft := addPhoto(t, s, 'b', "Kladd")
	inGallery := addPhoto(t, s, 'c', "Hagen")

	e, err := s.CreateEvent(ctx, Event{Title: "Julemarked", Start: d("2026-11-14"), MainPhoto: &live})
	require.NoError(t, err)
	_, err = s.Publish(ctx, e.ID, e.UpdatedAt)
	require.NoError(t, err)
	_, err = s.CreateEvent(ctx, Event{Title: "Kladd", MainPhoto: &inDraft})
	require.NoError(t, err)
	require.NoError(t, s.SetGallery(ctx, []int64{live.ID, inGallery.ID}))

	live.Alt = " "
	_, err = s.UpdatePhoto(ctx, live)
	require.ErrorIs(t, err, ErrAltRequired)

	var fe *FieldError
	require.ErrorAs(t, err, &fe)
	require.Equal(t, FieldError{Field: FieldAlt, PhotoID: live.ID}, FieldError{Field: fe.Field, PhotoID: fe.PhotoID})

	stored, err := s.Photo(ctx, live.ID)
	require.NoError(t, err)
	require.Equal(t, "Boder i låven", stored.Alt)

	for _, p := range []Photo{inDraft, inGallery} {
		p.Alt = ""
		_, err = s.UpdatePhoto(ctx, p)
		require.NoError(t, err, "photo %d is on no live event page", p.ID)
	}
}

// A photo taken out of use gets its whole week from that moment, however
// old it is, so a slip in the gallery or an event can still be undone.
func TestStoreUnusedGraceStartsWhenTakenOutOfUse(t *testing.T) {
	const grace = 7 * 24 * time.Hour

	start := oslo("2026-09-01 12:00")
	s, clock := newTestStore(t, start)
	ctx := t.Context()

	fromGallery := addPhoto(t, s, 'a', "Hagen")
	replaced := addPhoto(t, s, 'b', "Boder")
	ofDeleted := addPhoto(t, s, 'c', "Låven")
	shared := addPhoto(t, s, 'd', "Tunet")
	newer := addPhoto(t, s, 'e', "Fjøset")

	require.NoError(t, s.SetGallery(ctx, []int64{fromGallery.ID, shared.ID}))
	kept, err := s.CreateEvent(ctx, Event{Title: "Kurs", MainPhoto: &replaced})
	require.NoError(t, err)
	deleted, err := s.CreateEvent(ctx, Event{Title: "Konsert", MainPhoto: &ofDeleted, ExtraPhotos: []Photo{shared}})
	require.NoError(t, err)

	released := start.Add(30 * 24 * time.Hour)
	clock.t = released
	require.NoError(t, s.SetGallery(ctx, []int64{shared.ID}))
	kept.MainPhoto = &newer
	_, err = s.UpdateEvent(ctx, kept, kept.UpdatedAt)
	require.NoError(t, err)
	require.NoError(t, s.DeleteEvent(ctx, deleted.ID))

	clock.t = released.Add(grace - time.Second)
	gone, err := s.DeleteUnusedPhotos(ctx, grace)
	require.NoError(t, err)
	require.Empty(t, gone, "a week to undo it")

	still, err := s.Photo(ctx, shared.ID)
	require.NoError(t, err)
	require.Equal(t, shared.UpdatedAt, still.UpdatedAt, "a photo still in use is not touched: its pages' lastmod stays")

	clock.t = released.Add(grace + time.Second)
	gone, err = s.DeleteUnusedPhotos(ctx, grace)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{fromGallery.SHA256, replaced.SHA256, ofDeleted.SHA256}, gone)
}

// An upload is stored with its centred crops in one step, so no photo is
// ever without them; the same bytes again are the photo already there.
func TestStoreAddPhoto(t *testing.T) {
	start := oslo("2026-10-10 12:00")
	s, clock := newTestStore(t, start)
	ctx := t.Context()

	p, created, err := s.AddPhoto(ctx, Photo{SHA256: sha('a'), MIME: "image/jpeg", Width: 1000, Height: 1250, Alt: "Låven"})
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, map[images.Kind]images.Rect{
		images.Card:   {X: 0, Y: 0, W: 1000, H: 1250},
		images.Banner: {X: 0, Y: 344, W: 1000, H: 562},
	}, p.Crops, "no share crop: the photo is narrower than a share image")

	clock.t = start.Add(time.Hour)
	again, created, err := s.AddPhoto(ctx, Photo{SHA256: sha('a'), MIME: "image/jpeg", Width: 1000, Height: 1250, Alt: "ignored"})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, clock.t.UTC(), again.UpdatedAt, "the grace period starts again")
	again.UpdatedAt = p.UpdatedAt
	require.Equal(t, p, again, "the stored photo is kept as it is")
}
