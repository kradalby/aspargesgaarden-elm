package images

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

const (
	// previewWidth is the crop tool's picture: sharp enough to place a crop,
	// small enough to load at once.
	previewWidth = 1200
	// firstPassWidth is the size the admin's page preview most likely asks
	// for; the request path builds anything else it wants.
	firstPassWidth = 800
)

// tier is one of the worker's queues; a lower tier is always taken first.
type tier int

const (
	tierFirst   tier = iota // a photo's first pass after Enqueue
	tierSecond              // the rest of a photo someone just changed
	tierBacklog             // everything Run finds missing at start
	tiers
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

// resume puts back a photo whose build stopped to let another tier go
// first, so it carries on before anything that queued after it.
func (q *fifo) resume(sha string) {
	if q.in == nil {
		q.in = map[string]bool{}
	}

	if !q.in[sha] {
		q.in[sha] = true
		q.items = append([]string{sha}, q.items...)
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
// Its own tail follows ahead of Run's backlog, so its status keeps moving.
func (s *Store) Enqueue(sha string) {
	s.Changed()
	s.enqueue(sha, tierFirst)
}

// Retry forgets the photo's recent build failures and queues it again: the
// admin's answer to StateFailed, for a cause that has gone away since.
func (s *Store) Retry(ctx context.Context, sha string) error {
	p, err := s.photo(ctx, sha)
	if err != nil {
		return fmt.Errorf("retry: %w", err)
	}

	s.mu.Lock()
	for _, sp := range Expected(p) {
		delete(s.failed, s.path(sp))
	}
	s.mu.Unlock()

	s.Enqueue(sha)

	return nil
}

func (s *Store) enqueue(sha string, t tier) {
	s.qmu.Lock()
	s.queues[t].push(sha)
	s.queued()
	s.qmu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// queued updates the gauge; qmu is held.
func (s *Store) queued() {
	n := 0
	for _, q := range s.queues {
		n += len(q.items)
	}

	metricQueue.Set(float64(n))
}

func (s *Store) next() (string, tier, bool) {
	s.qmu.Lock()
	defer s.qmu.Unlock()
	defer s.queued()

	for t := range s.queues {
		if sha, ok := s.queues[t].pop(); ok {
			return sha, tier(t), true
		}
	}

	return "", 0, false
}

// ahead reports whether a photo waits in a tier before t.
func (s *Store) ahead(t tier) bool {
	s.qmu.Lock()
	defer s.qmu.Unlock()

	for _, q := range s.queues[:t] {
		if len(q.items) > 0 {
			return true
		}
	}

	return false
}

// step does the worker's next unit of work and reports whether there was
// any. A tail stops between groups when an earlier tier has work and
// resumes from the front of its tier, so whatever the backlog, a changed
// photo waits for at most one group build.
func (s *Store) step(ctx context.Context) bool {
	sha, t, ok := s.next()
	if !ok {
		return false
	}

	// Failures are logged and counted by the build, and show in Status.
	if t == tierFirst {
		_, _ = s.warmUntil(ctx, sha, firstPass, nil)
		s.enqueue(sha, tierSecond)

		return true
	}

	stopped, _ := s.warmUntil(ctx, sha, allSpecs, func() bool { return s.ahead(t) })
	if stopped {
		s.qmu.Lock()
		s.queues[t].resume(sha)
		s.queued()
		s.qmu.Unlock()
	}

	return true
}

// Run works the queue until ctx ends, one build at a time, so requests keep
// the other job slots. At start it queues every photo behind any upload, so
// work lost in a restart, a new pipeline version or a restore is rebuilt
// without anyone asking.
func (s *Store) Run(ctx context.Context) error {
	photos, err := s.cat.Photos(ctx)
	if err != nil {
		s.log.Error("images: list photos to warm", "err", err)
	}

	for _, p := range photos {
		s.enqueue(p.SHA256, tierBacklog)
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

// PreviewURL is the address of the crop tool's picture of p, and ThumbURL
// of p whole at the smallest width, for the admin's lists. Both are "" for
// a photo with no variants. Status tells when the preview is built; the
// thumb is built on its first request if the queue has not got to it.
func PreviewURL(p Photo) string {
	pv, ok := previewSpec(p)
	if !ok {
		return ""
	}

	return pv.URL()
}

// ThumbURL: see PreviewURL.
func ThumbURL(p Photo) string {
	gs := groups(p)
	if len(gs) == 0 || gs[0].kind != Full {
		return ""
	}

	for _, s := range gs[0].specs {
		if s.Format == JPEG && s.Width == widths[0] {
			return s.URL()
		}
	}

	return ""
}

// warm builds the missing variants pick chooses from each group of a photo.
func (s *Store) warm(ctx context.Context, sha string, pick func(Photo, group) []VariantSpec) error {
	_, err := s.warmUntil(ctx, sha, pick, nil)
	return err
}

// warmUntil is warm that stops before a further group build once stop
// says so; stopped tells whether it did.
func (s *Store) warmUntil(ctx context.Context, sha string, pick func(Photo, group) []VariantSpec, stop func() bool) (stopped bool, err error) {
	p, err := s.cat.Photo(ctx, sha)
	if errors.Is(err, ErrNotFound) {
		return false, nil // deleted since it was queued
	}

	if err != nil {
		return false, fmt.Errorf("warm %s: %w", sha, err)
	}

	var (
		errs  []error
		built bool
	)

	for _, g := range groups(p) {
		specs := pick(p, g)
		if !slices.ContainsFunc(specs, func(sp VariantSpec) bool { return !exists(s.path(sp)) }) {
			continue
		}

		if built && stop != nil && stop() {
			return true, errors.Join(errs...)
		}

		built = true

		errs = append(errs, s.ensure(ctx, p, g, specs))
	}

	return false, errors.Join(errs...)
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
