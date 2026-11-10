package images

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"
)

// previewWidth is the crop tool's picture: sharp enough to place a crop,
// small enough to load at once.
const previewWidth = 1200

// Enqueue asks the worker to bring a photo's files in line with the
// catalogue: build what is missing, after an upload, a crop change or a
// publish. Someone is waiting for it, so it goes ahead of the backlog Run
// queued at start, behind what others asked for before, and a backlog photo
// being built steps aside after its current group (see yield). Its last
// failure is forgotten: asking again is the retry.
func (s *Store) Enqueue(sha string) { s.push(sha, true) }

// Retry is Enqueue, for the admin's "Prøv igjen" on StateFailed.
func (s *Store) Retry(_ context.Context, sha string) error {
	s.Enqueue(sha)
	return nil
}

func (s *Store) push(sha string, asked bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if asked {
		delete(s.failed, sha)
	}

	if i := slices.Index(s.queue, sha); i >= 0 {
		if !asked || i < s.asked {
			return
		}

		s.queue = slices.Delete(s.queue, i, i+1)
	}

	if asked {
		s.queue = slices.Insert(s.queue, s.asked, sha)
		s.asked++
	} else {
		s.queue = append(s.queue, sha)
	}

	metricQueue.Set(float64(len(s.queue)))

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// next takes the photo at the front of the queue and marks it running.
func (s *Store) next() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) == 0 {
		return "", false
	}

	s.running, s.runningAsked = s.queue[0], s.asked > 0
	s.queue = s.queue[1:]
	s.asked = max(s.asked-1, 0)
	metricQueue.Set(float64(len(s.queue)))

	return s.running, true
}

// yield puts the running photo back at the head of the backlog when it came
// from there and someone has asked for a photo since: a whole photo takes
// several group builds, and they wait for one at most.
func (s *Store) yield(sha string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.runningAsked || s.asked == 0 {
		return false
	}

	if !slices.Contains(s.queue, sha) {
		s.queue = slices.Insert(s.queue, s.asked, sha)
		metricQueue.Set(float64(len(s.queue)))
	}

	return true
}

// step builds the next photo in the queue, if there is one, and remembers
// how it went for Status.
func (s *Store) step(ctx context.Context) bool {
	sha, ok := s.next()
	if !ok {
		return false
	}

	err := s.prepare(ctx, sha)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = ""

	switch {
	case err == nil:
		delete(s.failed, sha)
	case ctx.Err() != nil:
		// Shutdown killed vips; that says nothing about the photo.
	default:
		s.failed[sha] = err
		s.log.Error("images: photo not prepared", "photo", sha, "reason", ReasonOf(err), "err", err)
	}

	return true
}

// Run works the queue until ctx ends, one photo at a time. At start it
// queues every photo, so sizes lost to a restart, a new pipeline version or
// a restore without variants/ are built without anyone asking.
func (s *Store) Run(ctx context.Context) error {
	photos, err := s.cat.Photos(ctx)
	if err != nil {
		s.log.Error("images: list photos to prepare", "err", err)
	}

	for _, p := range photos {
		s.push(p.SHA256, false)
	}

	for {
		for ctx.Err() == nil {
			if !s.step(ctx) {
				break
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-s.wake:
		}
	}
}

// prepare builds every missing variant of a photo, a group at a time in
// groups order, so the crop tool's picture comes first.
func (s *Store) prepare(ctx context.Context, sha string) error {
	p, err := s.cat.Photo(ctx, sha)
	if errors.Is(err, ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("prepare %s: %w", sha, err)
	}

	type work struct {
		g     group
		specs []VariantSpec
	}

	var todo []work

	for _, g := range groups(p) {
		var missing []VariantSpec
		for _, sp := range g.specs {
			if !exists(s.path(sp)) {
				missing = append(missing, sp)
			}
		}

		if len(missing) > 0 {
			todo = append(todo, work{g, missing})
		}
	}

	if len(todo) == 0 {
		return nil
	}

	orig := s.originalPath(p.SHA256, p.MIME)
	if _, err := os.Stat(orig); err != nil {
		return fmt.Errorf("original of %s: %w", sha, err)
	}

	var (
		errs  []error
		n     int
		start = time.Now()
	)

	for i, w := range todo {
		// A group that failed so far is tried again, and reported, on resume.
		if i > 0 && s.yield(sha) {
			return nil
		}

		n += len(w.specs)
		if err := s.buildGroup(ctx, p, orig, w.g, w.specs); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", w.g.kind, err))
		}
	}

	if len(errs) == 0 {
		s.log.Info("images: photo prepared", "photo", sha, "variants", n, "took", time.Since(start))
	}

	return errors.Join(errs...)
}

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

// thumbSpec is the Full JPEG at the ladder's first width.
func thumbSpec(p Photo) (VariantSpec, bool) {
	gs := groups(p)
	if len(gs) == 0 || gs[0].kind != Full {
		return VariantSpec{}, false
	}

	i := slices.IndexFunc(gs[0].specs, func(s VariantSpec) bool { return s.Format == JPEG && s.Width == widths[0] })
	if i < 0 {
		return VariantSpec{}, false
	}

	return gs[0].specs[i], true
}

// PreviewURL is the address of the crop tool's picture of p, and ThumbURL
// of p whole at the smallest width, for the admin's lists. Both are "" for
// a photo with no variants. Intake builds both; Status tells when the
// preview exists.
func PreviewURL(p Photo) string {
	pv, ok := previewSpec(p)
	if !ok {
		return ""
	}

	return pv.URL()
}

// ThumbURL: see PreviewURL.
func ThumbURL(p Photo) string {
	th, ok := thumbSpec(p)
	if !ok {
		return ""
	}

	return th.URL()
}

// State is where a photo's variants are.
type State string

const (
	StateQueued    State = "queued"    // waiting for the worker
	StatePreparing State = "preparing" // the worker is on it
	StateReady     State = "ready"     // every variant exists
	StateFailed    State = "failed"    // files are missing and nothing is building them
)

// Status is what the admin polls while a photo is prepared. It shows
// "Bildene klargjøres …" until State is StateReady, and on StateFailed turns
// Reason into a sentence and offers Retry. Publishing need not wait: a size
// not built yet is missing on the page until the worker gets to it.
type Status struct {
	State        State
	Ready, Total int    // variant files present and expected
	Reason       Reason // set when State is StateFailed
	// Preview is the URL of the crop tool's picture once it exists; Width
	// and Height are the upright size crops are given in.
	Preview       string
	Width, Height int
}

// Status reports how far a photo's variants are, from one catalogue lookup,
// file metadata and memory, so it is cheap to poll.
func (s *Store) Status(ctx context.Context, sha string) (Status, error) {
	p, err := s.cat.Photo(ctx, sha)
	if err != nil {
		return Status{}, fmt.Errorf("status: %w", err)
	}

	st := Status{Width: p.Width, Height: p.Height}
	if pv, ok := previewSpec(p); ok && exists(s.path(pv)) {
		st.Preview = pv.URL()
	}

	for _, sp := range Expected(p) {
		st.Total++
		if exists(s.path(sp)) {
			st.Ready++
		}
	}

	s.mu.Lock()
	running, queued, failed := s.running == sha, slices.Contains(s.queue, sha), s.failed[sha]
	s.mu.Unlock()

	switch {
	case st.Ready == st.Total:
		st.State = StateReady
	case running:
		st.State = StatePreparing
	case queued:
		st.State = StateQueued
	default:
		// Missing with nobody on it: a failure, or anything that left files
		// missing; either way the answer is a retry.
		st.State, st.Reason = StateFailed, ReasonOf(failed)
		if st.Reason == ReasonNone {
			st.Reason = ReasonInternal
		}
	}

	return st, nil
}
