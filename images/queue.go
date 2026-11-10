package images

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
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

	err := s.prepareSafe(ctx, sha)

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

// prepareSafe is prepare with a panic turned into that photo's failure: a
// bug one photo trips must not stop every later build.
func (s *Store) prepareSafe(ctx context.Context, sha string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("prepare %s panicked: %v\n%s", sha, r, debug.Stack())
		}
	}()

	return s.prepare(ctx, sha)
}

// Run works the queue until ctx ends, one photo at a time. At start it
// queues every photo, so sizes lost to a restart, a new pipeline version or
// a restore without variants/ are built without anyone asking.
func (s *Store) Run(ctx context.Context) error {
	defer close(s.stopped)

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

// prepare brings a photo's files in line with the catalogue: it removes the
// variants the photo no longer has (an old crop), then builds the missing
// ones a group at a time in groups order, so the crop tool's picture comes
// first. A photo the catalogue does not have loses all its files.
func (s *Store) prepare(ctx context.Context, sha string) error {
	if !shaRE.MatchString(sha) {
		return fmt.Errorf("prepare %q: not a sha256", sha)
	}

	p, err := s.cat.Photo(ctx, sha)
	if errors.Is(err, ErrNotFound) {
		return s.forget(sha)
	}

	if err != nil {
		return fmt.Errorf("prepare %s: %w", sha, err)
	}

	s.files.Lock()
	delete(s.fresh, sha)
	s.files.Unlock()

	if err := s.prune(p); err != nil {
		return err
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

	// Once per photo, whatever the groups: each probe is three processes.
	hdr, err := s.vips.preflight(ctx, orig, p.MIME)
	if err != nil {
		return fmt.Errorf("preflight %s: %w", sha, err)
	}

	// Crops are in the frame the catalogue recorded; a different original
	// would put them in the wrong place.
	if hdr.width != p.Width || hdr.height != p.Height {
		return fmt.Errorf("original is %dx%d, the catalogue says %dx%d", hdr.width, hdr.height, p.Width, p.Height)
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
		if err := s.buildGroup(ctx, p, orig, w.g, w.specs, s.path); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", w.g.kind, err))
		}
	}

	if len(errs) == 0 {
		s.log.Info("images: photo prepared", "photo", sha, "variants", n, "took", time.Since(start))
	}

	return errors.Join(errs...)
}

// photoDir is the folder of a photo's variants.
func (s *Store) photoDir(sha string) string {
	return filepath.Join(s.root, "variants", fmt.Sprintf("v%d", PipelineVersion), sha[:2], sha)
}

// prune removes the variant files of p that its current crops do not
// explain. Pages link only to current names, and Changed has dropped every
// kept page before the worker gets here.
func (s *Store) prune(p Photo) error {
	keep := map[string]bool{}
	for _, sp := range Expected(p) {
		keep[sp.file()] = true
	}

	entries, err := os.ReadDir(s.photoDir(p.SHA256))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("prune %s: %w", p.SHA256, err)
	}

	var errs []error

	for _, e := range entries {
		if !keep[e.Name()] {
			errs = append(errs, os.Remove(filepath.Join(s.photoDir(p.SHA256), e.Name())))
		}
	}

	return errors.Join(errs...)
}

// forget removes every file of a photo the catalogue does not have: its
// variants and its original. The photo is named only by a delete or the
// unused-photo sweep, never found by looking at the folders, so a database
// restored without its photos cannot set off a mass delete. An upload of the
// same bytes since (see Intake) keeps everything: its row is on the way.
func (s *Store) forget(sha string) error {
	s.files.Lock()
	defer s.files.Unlock()

	if s.fresh[sha] {
		return nil
	}

	dir := filepath.Join(s.root, "originals", sha[:2])

	errs := []error{os.RemoveAll(s.photoDir(sha))}
	for _, ext := range exts {
		if err := os.Remove(filepath.Join(dir, sha+"."+ext)); !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}

	// Each only if empty.
	_ = os.Remove(filepath.Dir(s.photoDir(sha)))
	_ = os.Remove(dir)

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
