package images

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	// reconcileEvery spaces the cleanup runs after the one at start.
	reconcileEvery = 24 * time.Hour
	// previewWidth is the crop tool's picture: sharp enough to place a crop,
	// small enough to load at once.
	previewWidth = 1200
	// firstPassWidth is the size the admin's page preview most likely asks
	// for; the request path builds anything else it wants.
	firstPassWidth = 800
)

// fifo is a queue of photos without duplicates.
type fifo struct {
	items []string
	in    map[string]bool
}

func (q *fifo) push(sha string) {
	if q.in == nil {
		q.in = map[string]bool{}
	}

	if !q.in[sha] {
		q.in[sha] = true
		q.items = append(q.items, sha)
	}
}

func (q *fifo) pop() (string, bool) {
	if len(q.items) == 0 {
		return "", false
	}

	sha := q.items[0]
	q.items = q.items[1:]
	// Off before the build, so a crop change during it queues again.
	delete(q.in, sha)

	return sha, true
}

// Enqueue asks for every variant of a photo to be built in the background,
// and marks the catalogue view stale. Call it after an upload, a crop change
// and a publish. Only missing files are built, so after a crop change that
// is the changed kind alone.
//
// The photo goes to the first pass: what someone is waiting to look at (see
// firstPass) is built for it before the long tail of any earlier photo.
func (s *Store) Enqueue(sha string) {
	s.Changed()
	s.enqueue(sha, true)
}

func (s *Store) enqueue(sha string, first bool) {
	s.qmu.Lock()
	if first {
		s.first.push(sha)
	} else {
		s.rest.push(sha)
	}
	metricQueue.Set(float64(len(s.first.items) + len(s.rest.items)))
	s.qmu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Store) next() (string, bool, bool) {
	s.qmu.Lock()
	defer s.qmu.Unlock()
	defer func() { metricQueue.Set(float64(len(s.first.items) + len(s.rest.items))) }()

	if sha, ok := s.first.pop(); ok {
		return sha, true, true
	}

	sha, ok := s.rest.pop()

	return sha, false, ok
}

// Run works the queue until ctx ends, one build at a time, so requests keep
// the other job slots. At start it reconciles and queues every photo behind
// any upload, so work lost in a restart, a new pipeline version or a restore
// is rebuilt without anyone asking. It then reconciles every reconcileEvery.
func (s *Store) Run(ctx context.Context) error {
	s.reconcileAndLog(ctx)

	photos, err := s.cat.Photos(ctx)
	if err != nil {
		s.log.Error("images: list photos to warm", "err", err)
	}

	for _, p := range photos {
		s.enqueue(p.SHA256, false)
	}

	t := time.NewTicker(reconcileEvery)
	defer t.Stop()

	for {
		for ctx.Err() == nil {
			sha, first, ok := s.next()
			if !ok {
				break
			}

			// Failures are logged and counted by the build, and show in Status.
			if first {
				_ = s.warm(ctx, sha, firstPass)
				s.enqueue(sha, false)
			} else {
				_ = s.warm(ctx, sha, allSpecs)
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			s.reconcileAndLog(ctx)
		case <-s.wake:
		}
	}
}

func (s *Store) reconcileAndLog(ctx context.Context) {
	rep, err := s.Reconcile(ctx, false)
	if err != nil {
		s.log.Error("images: reconcile", "err", err)
		return
	}

	s.log.Info("images: reconciled", rep.Attrs()...)
}

// firstPass is what the admin is waiting for: the crop tool's preview, the
// share image, and one AVIF per kind at the page preview's size.
func firstPass(p Photo, g group) []VariantSpec {
	if g.kind == Share {
		return g.specs
	}

	var out []VariantSpec
	if pv, ok := previewSpec(p); ok && g.kind == Full {
		out = append(out, pv)
	}

	w := 0
	for _, s := range g.specs {
		if s.Format == AVIF && s.Width <= firstPassWidth {
			w = max(w, s.Width)
		}
	}

	for _, s := range g.specs {
		if s.Format == AVIF && s.Width == w {
			out = append(out, s)
		}
	}

	return out
}

func allSpecs(_ Photo, g group) []VariantSpec { return g.specs }

// previewSpec is the Full JPEG the crop tool shows. JPEG because the
// cropper draws it on a canvas in any browser.
func previewSpec(p Photo) (VariantSpec, bool) {
	gs := groups(p)
	if len(gs) == 0 || gs[0].kind != Full {
		return VariantSpec{}, false
	}

	var best VariantSpec
	for _, s := range gs[0].specs {
		if s.Format == JPEG && s.Width <= previewWidth && s.Width > best.Width {
			best = s
		}
	}

	return best, best.Width > 0
}

// warm builds the missing variants pick chooses from each group of a photo.
func (s *Store) warm(ctx context.Context, sha string, pick func(Photo, group) []VariantSpec) error {
	p, err := s.cat.Photo(ctx, sha)
	if errors.Is(err, ErrNotFound) {
		return nil // deleted since it was queued
	}

	if err != nil {
		return fmt.Errorf("warm %s: %w", sha, err)
	}

	var errs []error
	for _, g := range groups(p) {
		errs = append(errs, s.ensure(ctx, p, g, pick(p, g)))
	}

	return errors.Join(errs...)
}

// WarmAll builds every missing variant of every photo, in the foreground.
func (s *Store) WarmAll(ctx context.Context) error {
	photos, err := s.cat.Photos(ctx)
	if err != nil {
		return fmt.Errorf("list photos: %w", err)
	}

	var errs []error
	for i, p := range photos {
		err := s.warm(ctx, p.SHA256, allSpecs)
		errs = append(errs, err)
		s.log.Info("images: warmed", "photo", p.SHA256, "n", i+1, "of", len(photos), "ok", err == nil)
	}

	return errors.Join(errs...)
}

// State is where a photo's variants are.
type State string

const (
	StateQueued    State = "queued"    // waiting for the queue
	StatePreparing State = "preparing" // a build for it is running
	StateReady     State = "ready"     // every variant exists
	StateFailed    State = "failed"    // a build failed; not retried for failTTL
)

// Status is what the admin polls while a photo is prepared. It shows
// "Bildene klargjøres …" until State is StateReady, and on StateFailed turns
// Reason into a sentence. Publishing need not wait: the request path builds
// whatever a visitor asks for first.
type Status struct {
	State        State
	Ready, Total int    // variant files present and expected
	Reason       Reason // set when State is StateFailed
	// Preview is the URL of the crop tool's picture once it exists; Width
	// and Height are the upright size crops are given in.
	Preview       string
	Width, Height int
}

// Status reports how far a photo's variants are. It reads memory and file
// metadata only, so it is cheap to poll.
func (s *Store) Status(ctx context.Context, sha string) (Status, error) {
	p, err := s.photo(ctx, sha)
	if err != nil {
		return Status{}, fmt.Errorf("status: %w", err)
	}

	st := Status{Width: p.Width, Height: p.Height}
	if pv, ok := previewSpec(p); ok && exists(s.path(pv)) {
		st.Preview = pv.URL()
	}

	var (
		now      = s.now()
		building bool
		failed   error
	)

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sp := range Expected(p) {
		st.Total++

		path := s.path(sp)
		f, ok := s.failed[path]

		switch {
		case exists(path):
			st.Ready++
		case s.inflight[path] != nil:
			building = true
		case ok && now.Before(f.until):
			failed = f.err
		}
	}

	switch {
	case st.Ready == st.Total:
		st.State = StateReady
	case failed != nil:
		st.State, st.Reason = StateFailed, ReasonOf(failed)
	case building:
		st.State = StatePreparing
	default:
		st.State = StateQueued
	}

	return st, nil
}
