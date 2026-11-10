package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/server"
	"github.com/kradalby/asparges/site"
)

// e2eClock is the wall clock plus a shift the touch hook moves on. Saves are
// stamped in whole seconds, so a "second editor" in the same second as the
// first would go unseen; the browser tests' fake server has strictly
// increasing stamps, and this gives the real one the same.
type e2eClock struct {
	mu    sync.Mutex
	shift time.Duration
}

func (c *e2eClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return time.Now().Add(c.shift)
}

func (c *e2eClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.shift += d
}

// e2eHooks answers the admin browser tests' hooks (the front end's e2e/),
// so they run against the real server: POST /__reset replaces everything
// with their seed data, POST /__touch/{id} saves an event as a second
// editor would, POST /__crop/{id} stores crops unchecked. Only
// `serve --dev --e2e-fixtures` mounts them.
func e2eHooks(store *site.Store, imgs *images.Store, srv *server.Server, fixtures string, clock *e2eClock) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /__reset", func(w http.ResponseWriter, r *http.Request) {
		shas, err := e2eReset(r.Context(), store, imgs, fixtures)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		srv.Changed(shas...)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /__touch/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		clock.advance(time.Second)

		e, err := store.Event(r.Context(), id)
		if err == nil {
			_, err = store.UpdateEvent(r.Context(), e)
		}

		if err != nil && !errors.Is(err, site.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		srv.Changed()
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /__crop/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var crops map[site.CropKind]site.Rect
		if err := json.NewDecoder(r.Body).Decode(&crops); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// The store checks only the frame, so this keeps a crop under today's
		// floor, as an old import may have.
		p, err := store.Photo(r.Context(), id)
		if err == nil {
			err = store.SetCrops(r.Context(), id, crops)
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		srv.Changed(p.SHA256)
		w.WriteHeader(http.StatusNoContent)
	})

	return mux
}

// e2eReset deletes every event, photo and photographer and puts in the
// fake server's seed: photos 1 and 2 (both in the gallery), photographer 3
// on photo 1, and event 4, a published course over on 31 October with
// photo 2. The tests address them by those ids, which the fake hands out
// from one counter; SQLite counts per table from the highest id in use, so
// placeholders hold the lower ids while the seed rows are made.
func e2eReset(ctx context.Context, store *site.Store, imgs *images.Store, fixtures string) ([]string, error) {
	if err := e2eWipe(ctx, store); err != nil {
		return nil, fmt.Errorf("wipe: %w", err)
	}

	landskap, err := e2ePhoto(ctx, store, imgs, filepath.Join(fixtures, "landskap.jpg"), "Boder med julepynt i låven")
	if err != nil {
		return nil, err
	}

	portrett, err := e2ePhoto(ctx, store, imgs, filepath.Join(fixtures, "portrett.jpg"), "Kursholder ved kjøkkenbenken")
	if err != nil {
		return nil, err
	}

	var placeholders []site.Photographer

	for i := range 2 {
		p, err := store.CreatePhotographer(ctx, site.Photographer{Name: fmt.Sprintf("e2e plassholder %d", i)})
		if err != nil {
			return nil, err
		}

		placeholders = append(placeholders, p)
	}

	ola, err := store.CreatePhotographer(ctx, site.Photographer{Name: "Ola Nordmann", URL: "https://example.no"})
	if err != nil {
		return nil, err
	}

	for _, p := range placeholders {
		if err := store.DeletePhotographer(ctx, p.ID); err != nil {
			return nil, err
		}
	}

	landskap.Photographer = &ola
	if _, err := store.UpdatePhoto(ctx, landskap); err != nil {
		return nil, err
	}

	if err := store.SetGallery(ctx, []int64{landskap.ID, portrett.ID}); err != nil {
		return nil, err
	}

	var drafts []site.Event

	for range 3 {
		e, err := store.CreateEvent(ctx, site.Event{})
		if err != nil {
			return nil, err
		}

		drafts = append(drafts, e)
	}

	course, err := store.CreateEvent(ctx, site.Event{
		Title: "Takkebakstkurs", Summary: "Lær å bake lefser og flatbrød.", BodyMD: "Velkommen til kurs!",
		Start: mustDate("2026-10-31"), StartTime: "10:00", EndTime: "15:00", PriceText: "Kr 1650,-",
		MainPhoto: &portrett,
	})
	if err != nil {
		return nil, err
	}

	if _, err := store.Publish(ctx, course.ID); err != nil {
		return nil, err
	}

	for _, e := range drafts {
		if err := store.DeleteEvent(ctx, e.ID); err != nil {
			return nil, err
		}
	}

	return []string{landskap.SHA256, portrett.SHA256}, nil
}

func e2eWipe(ctx context.Context, store *site.Store) error {
	events, err := store.Events(ctx)
	if err != nil {
		return err
	}

	for _, e := range events {
		if err := store.DeleteEvent(ctx, e.ID); err != nil {
			return err
		}
	}

	if err := store.SetGallery(ctx, nil); err != nil {
		return err
	}

	photos, err := store.Photos(ctx)
	if err != nil {
		return err
	}

	for _, p := range photos {
		if err := store.DeletePhoto(ctx, p.ID); err != nil {
			return err
		}
	}

	photographers, err := store.Photographers(ctx)
	if err != nil {
		return err
	}

	for _, p := range photographers {
		if err := store.DeletePhotographer(ctx, p.ID); err != nil {
			return err
		}
	}

	return nil
}

func e2ePhoto(ctx context.Context, store *site.Store, imgs *images.Store, path, alt string) (site.Photo, error) {
	f, err := os.Open(path)
	if err != nil {
		return site.Photo{}, fmt.Errorf("e2e fixture: %w", err)
	}
	defer f.Close()

	o, err := imgs.Intake(ctx, f)
	if err != nil {
		return site.Photo{}, fmt.Errorf("e2e fixture %s: %w", path, err)
	}

	p, err := store.AddPhoto(ctx, site.Photo{SHA256: o.SHA256, MIME: o.MIME, Width: o.Width, Height: o.Height, Alt: alt})
	if err != nil {
		return site.Photo{}, err
	}

	crops := map[site.CropKind]site.Rect{}
	for _, k := range images.CropKinds {
		if r, err := images.DefaultCrop(k, p.Width, p.Height); err == nil {
			crops[site.CropKind(k)] = site.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
		}
	}

	if err := store.SetCrops(ctx, p.ID, crops); err != nil {
		return site.Photo{}, err
	}

	return store.Photo(ctx, p.ID)
}

func mustDate(s string) site.Date {
	d, err := site.ParseDate(s)
	if err != nil {
		panic(err)
	}

	return d
}
