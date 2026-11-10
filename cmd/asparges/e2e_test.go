package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/db"
	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/server"
	"github.com/kradalby/asparges/site"
)

func TestE2EHooks(t *testing.T) {
	dir := t.TempDir()
	fixtures := filepath.Join(dir, "fixtures")
	require.NoError(t, os.MkdirAll(fixtures, 0o750))

	for name, src := range map[string]string{"landskap.jpg": "upright.jpg", "portrett.jpg": "orient6.jpg"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "images", "testdata", src))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(fixtures, name), b, 0o600))
	}

	d, err := db.Open(t.Context(), filepath.Join(dir, "asparges.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	clock := &e2eClock{}
	store := site.NewStore(d, clock.now)

	imgs, err := images.New(dir, store.Catalog())
	require.NoError(t, err)

	srv := server.New(server.Config{BaseURL: "http://127.0.0.1:8090", Dev: true, Store: store, Images: imgs, Now: clock.now})
	hooks := e2eHooks(store, imgs, srv, fixtures, clock)

	post := func(path string) int {
		rec := httptest.NewRecorder()
		hooks.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "POST", path, nil))

		return rec.Code
	}

	// Twice: the second reset starts from the first one's leftovers and an
	// extra event, and lands on the same ids.
	for range 2 {
		_, err := store.CreateEvent(t.Context(), site.Event{Title: "Rester"})
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, post("/__reset"))

		events, err := store.Events(t.Context())
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.EqualValues(t, 4, events[0].ID)
		require.Equal(t, site.Published, events[0].Status)
		require.EqualValues(t, 2, events[0].MainPhoto.ID)

		gallery, err := store.Gallery(t.Context())
		require.NoError(t, err)
		require.Len(t, gallery, 2)
		require.EqualValues(t, 1, gallery[0].ID)
		require.EqualValues(t, 3, gallery[0].Photographer.ID)
		require.Equal(t, "Ola Nordmann", gallery[0].Photographer.Name)

		photographers, err := store.Photographers(t.Context())
		require.NoError(t, err)
		require.Len(t, photographers, 1)
	}

	before, err := store.Event(t.Context(), 4)
	require.NoError(t, err)

	require.Equal(t, http.StatusNoContent, post("/__touch/4"))

	after, err := store.Event(t.Context(), 4)
	require.NoError(t, err)
	require.GreaterOrEqual(t, after.UpdatedAt.Sub(before.UpdatedAt), time.Second, "a second editor is always seen")
	require.Equal(t, before.PriceText, after.PriceText)

	// An old import's banner, under today's floor of 800.
	rec := httptest.NewRecorder()
	hooks.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "POST", "/__crop/2", strings.NewReader(`{"banner":{"x":0,"y":0,"w":700,"h":394}}`)))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	p, err := store.Photo(t.Context(), 2)
	require.NoError(t, err)
	require.Equal(t, site.Rect{X: 0, Y: 0, W: 700, H: 394}, p.Crops[site.CropBanner])
}
