package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

	post := func(path string, body io.Reader) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, body)
		req.Host = "127.0.0.1:8090"
		hooks.ServeHTTP(rec, req)

		return rec
	}

	// Twice: the second reset starts from the first one's leftovers and an
	// extra event, and names the rows it made.
	var seed e2eSeed

	for range 2 {
		_, err := store.CreateEvent(t.Context(), site.Event{Title: "Rester"})
		require.NoError(t, err)

		rec := post("/__reset", nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &seed))

		events, err := store.Events(t.Context())
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.Equal(t, seed.Course, events[0].ID)
		require.Equal(t, site.Published, events[0].Status)
		require.Equal(t, seed.Portrett, events[0].MainPhoto.ID)

		gallery, err := store.Gallery(t.Context())
		require.NoError(t, err)
		require.Len(t, gallery, 2)
		require.Equal(t, seed.Landskap, gallery[0].ID)
		require.Equal(t, seed.Photographer, gallery[0].Photographer.ID)
		require.Equal(t, "Ola Nordmann", gallery[0].Photographer.Name)

		photographers, err := store.Photographers(t.Context())
		require.NoError(t, err)
		require.Len(t, photographers, 1)

		broken, err := store.Photo(t.Context(), seed.Broken)
		require.NoError(t, err)
		require.Equal(t, e2eBrokenSHA, broken.SHA256)
	}

	course := strconv.FormatInt(seed.Course, 10)

	before, err := store.Event(t.Context(), seed.Course)
	require.NoError(t, err)

	require.Equal(t, http.StatusNoContent, post("/__touch/"+course, nil).Code)

	after, err := store.Event(t.Context(), seed.Course)
	require.NoError(t, err)
	require.GreaterOrEqual(t, after.UpdatedAt.Sub(before.UpdatedAt), time.Second, "a second editor is always seen")
	require.Equal(t, before.PriceText, after.PriceText)

	// An old import's banner, under today's floor of 800.
	rec := post("/__crop/"+strconv.FormatInt(seed.Portrett, 10), strings.NewReader(`{"banner":{"x":0,"y":0,"w":700,"h":394}}`))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	p, err := store.Photo(t.Context(), seed.Portrett)
	require.NoError(t, err)
	require.Equal(t, images.Rect{X: 0, Y: 0, W: 700, H: 394}, p.Crops[images.Banner])

	// The hooks delete everything, so they refuse what the admin refuses: a
	// write from another page in the browser, and a name that is not this
	// machine (a rebound DNS name).
	for name, change := range map[string]func(r *http.Request){
		"cross-site": func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
		"other host": func(r *http.Request) { r.Host = "evil.example:8090" },
	} {
		rec := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/__reset", nil)
		r.Host = "127.0.0.1:8090"
		change(r)
		hooks.ServeHTTP(rec, r)
		require.Equal(t, http.StatusForbidden, rec.Code, name)
	}

	kept, err := store.Event(t.Context(), seed.Course)
	require.NoError(t, err)
	require.Equal(t, after.CreatedAt, kept.CreatedAt, "a refused reset changed nothing")
}

// Without --e2e-fixtures, serve has no hooks: a POST that would wipe the
// data reaches the public site and is not found.
func TestServeHasNoHooksWithoutTheFlag(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() {
		done <- serve(ctx, slog.New(slog.DiscardHandler), serveConfig{
			dataDir: t.TempDir(), listenAddr: addr, baseURL: "http://" + addr, dev: true,
		})
	}()

	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})

	base := "http://" + addr
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
		if err != nil {
			return false
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}

		_ = res.Body.Close()

		return true
	}, 30*time.Second, 50*time.Millisecond)

	for _, path := range []string{"/__reset", "/__touch/1", "/__crop/1"} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+path, nil)
		require.NoError(t, err)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, res.Body.Close())
		require.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, res.StatusCode, path)
	}
}

// Every day the suite runs is the same day to the server: its events are
// dated against e2eEpoch.
func TestE2EClockStartsAtEpoch(t *testing.T) {
	c := &e2eClock{}
	first := c.now()
	require.False(t, first.Before(e2eEpoch))
	require.Less(t, first.Sub(e2eEpoch), time.Minute)

	c.advance(time.Second)
	require.GreaterOrEqual(t, c.now().Sub(first), time.Second)
}
