package site

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
)

func TestCatalog(t *testing.T) {
	s, _ := newTestStore(t, time.Date(2026, 11, 10, 12, 0, 0, 0, Oslo))
	ctx := t.Context()

	p := addPhoto(t, s, 'a', "Låven")
	addPhoto(t, s, 'b', "")
	require.NoError(t, s.SetCrop(ctx, p.ID, CropCard, Rect{X: 100, Y: 0, W: 2400, H: 3000}))

	want := images.Photo{
		SHA256: sha('a'), MIME: "image/jpeg", Width: 4000, Height: 3000,
		Crops: map[images.Kind]images.Rect{images.Card: {X: 100, Y: 0, W: 2400, H: 3000}},
	}

	got, err := s.Catalog().Photo(ctx, sha('a'))
	require.NoError(t, err)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Photo mismatch (-want +got):\n%s", diff)
	}

	_, err = s.Catalog().Photo(ctx, sha('c'))
	require.ErrorIs(t, err, images.ErrNotFound)

	all, err := s.Catalog().Photos(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.Equal(t, want, all[0])
}
