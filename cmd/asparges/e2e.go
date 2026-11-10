package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/server"
	"github.com/kradalby/asparges/site"
)

// e2eEpoch is where the browser tests' clock starts: their events are dated
// against it, so the suite gives the same answer on every day it runs.
var e2eEpoch = time.Date(2026, time.November, 10, 10, 0, 0, 0, site.Oslo)

// e2eClock runs from e2eEpoch at wall-clock speed, plus a shift the touch
// hook moves on. Saves are stamped in whole seconds, so a "second editor" in
// the same second as the first would go unseen; the shift makes every touch
// a later stamp.
type e2eClock struct {
	mu    sync.Mutex
	start time.Time
	shift time.Duration
}

func (c *e2eClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.start.IsZero() {
		c.start = time.Now()
	}

	return e2eEpoch.Add(time.Since(c.start) + c.shift)
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
// `serve --dev --e2e-fixtures` mounts them, and like the admin they answer
// only to a loopback Host and never to a write from another origin, so no
// page open in the developer's browser can wipe the data.
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
			_, err = store.UpdateEvent(r.Context(), e, e.UpdatedAt)
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

		var crops map[images.Kind]images.Rect
		if err := json.NewDecoder(r.Body).Decode(&crops); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// The store checks only the frame, so this keeps a crop under today's
		// floor, as an old import may have.
		p, err := store.SetCrops(r.Context(), id, crops)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		srv.Changed(p.SHA256)
		w.WriteHeader(http.StatusNoContent)
	})

	return http.NewCrossOriginProtection().Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}

		if !isLoopback(net.JoinHostPort(strings.Trim(host, "[]"), "0")) {
			http.Error(w, "not a loopback host", http.StatusForbidden)
			return
		}

		mux.ServeHTTP(w, r)
	}))
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

	if _, err := store.Publish(ctx, course.ID, course.UpdatedAt); err != nil {
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
		if _, err := store.RemovePhoto(ctx, p.ID); err != nil {
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

	p, _, err := store.AddPhoto(ctx, site.Photo{SHA256: o.SHA256, MIME: o.MIME, Width: o.Width, Height: o.Height, Alt: alt})

	return p, err
}

func mustDate(s string) site.Date {
	d, err := site.ParseDate(s)
	if err != nil {
		panic(err)
	}

	return d
}
