package site

import (
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
	_, err = s.UpdateEventSeen(ctx, theirs, loaded)
	require.NoError(t, err)
	clock.t = clock.t.Add(time.Minute)

	mine := e
	mine.PriceText = "50 kr"
	_, err = s.UpdateEventSeen(ctx, mine, loaded)
	require.ErrorIs(t, err, ErrStale)
	_, err = s.PublishSeen(ctx, e.ID, loaded)
	require.ErrorIs(t, err, ErrStale)
	_, err = s.UnpublishSeen(ctx, e.ID, loaded)
	require.ErrorIs(t, err, ErrStale)

	stored, err := s.Event(ctx, e.ID)
	require.NoError(t, err)
	require.Equal(t, "Gratis", stored.PriceText, "a stale save writes nothing")

	pub, err := s.PublishSeen(ctx, e.ID, stored.UpdatedAt)
	require.NoError(t, err)
	require.Equal(t, Published, pub.Status)

	clock.t = clock.t.Add(time.Minute)
	hidden, err := s.UnpublishSeen(ctx, e.ID, pub.UpdatedAt)
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

func TestStoreSetCrops(t *testing.T) {
	s, _ := newTestStore(t, oslo("2026-10-10 12:00"))
	ctx := t.Context()
	p := addPhoto(t, s, 'a', "x")

	good := images.Rect{X: 0, Y: 0, W: 2400, H: 3000}
	err := s.SetCrops(ctx, p.ID, map[images.Kind]images.Rect{images.Card: good, images.Banner: {X: 0, Y: 0, W: 5000, H: 100}})
	require.ErrorIs(t, err, images.ErrCropOutside)

	var fe *FieldError
	require.ErrorAs(t, err, &fe)
	require.Equal(t, Field("banner"), fe.Field)

	got, err := s.Photo(ctx, p.ID)
	require.NoError(t, err)
	require.Empty(t, got.Crops, "one bad crop writes none")

	require.NoError(t, s.SetCrops(ctx, p.ID, map[images.Kind]images.Rect{images.Card: good}))
	got, err = s.Photo(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, map[images.Kind]images.Rect{images.Card: good}, got.Crops)

	require.ErrorIs(t, s.SetCrops(ctx, 999, map[images.Kind]images.Rect{images.Card: good}), ErrNotFound)
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
	require.NoError(t, s.RenewPhoto(ctx, renewed.ID))

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

func TestStorePhotoBySHA256(t *testing.T) {
	s, _ := newTestStore(t, oslo("2026-10-10 12:00"))
	p := addPhoto(t, s, 'a', "x")

	got, err := s.PhotoBySHA256(t.Context(), p.SHA256)
	require.NoError(t, err)
	require.Equal(t, p.ID, got.ID)

	_, err = s.PhotoBySHA256(t.Context(), sha('f'))
	require.ErrorIs(t, err, ErrNotFound)
}
