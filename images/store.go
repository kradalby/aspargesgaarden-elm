package images

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// defaultJobs is how many groups build at once. Each runs one vips
	// process at a time with vipsThreads threads, and every child counts
	// against the unit's MemoryMax.
	defaultJobs = 2
	// failTTL is how long a failed variant is not retried, so a broken photo
	// cannot start a build on every request.
	failTTL = 10 * time.Minute
	dirPerm = 0o750
	// viewMaxAge bounds how stale the in-memory catalogue view can get if a
	// Changed call is missed.
	viewMaxAge = time.Minute
)

// Store owns the image folders below root:
//
//	originals/<sha[0:2]>/<sha256>.<ext>   uploads as received; never served
//	variants/v<pipeline>/<sha[0:2]>/<sha256>/<kind>-<crop8>-<width>.<ext>
//	tmp/                                  work files; same filesystem as the rest
type Store struct {
	root      string
	cat       Catalog
	log       *slog.Logger
	vips      *vips
	now       func() time.Time
	maxUpload int64
	sweep     func(context.Context) error
	jobs      chan struct{}
	builds    atomic.Int64 // group builds started; tests read it

	mu       sync.Mutex
	inflight map[string]*flight // by variant path
	failed   map[string]failure // by variant path

	vmu       sync.Mutex
	view      map[string]Photo // by sha256
	viewAt    time.Time
	viewStale bool

	qmu    sync.Mutex
	queues [tiers]fifo // see Enqueue
	wake   chan struct{}
}

type flight struct {
	done chan struct{}
	err  error
}

type failure struct {
	err   error
	until time.Time
}

// Option configures a Store.
type Option func(*Store) error

// WithLogger sets the logger; the default discards.
func WithLogger(l *slog.Logger) Option {
	return func(s *Store) error { s.log = l; return nil }
}

// WithClock replaces time.Now for the negative cache and the grace periods.
func WithClock(now func() time.Time) Option {
	return func(s *Store) error { s.now = now; return nil }
}

// WithSweep runs sweep before each scheduled reconcile, so photo rows it
// deletes from the catalogue lose their files in the same pass.
func WithSweep(sweep func(context.Context) error) Option {
	return func(s *Store) error { s.sweep = sweep; return nil }
}

// WithJobs sets how many variant groups may build at once.
func WithJobs(n int) Option {
	return func(s *Store) error {
		if n < 1 {
			return fmt.Errorf("jobs must be at least 1, got %d", n)
		}

		s.jobs = make(chan struct{}, n)

		return nil
	}
}

// New opens the store at root, creating its folders. It fails when the vips
// CLI is missing, so a broken deploy stops at start rather than on the first
// photo.
func New(root string, cat Catalog, opts ...Option) (*Store, error) {
	s := &Store{
		root:      root,
		cat:       cat,
		log:       slog.New(slog.DiscardHandler),
		now:       time.Now,
		maxUpload: MaxUpload,
		jobs:      make(chan struct{}, defaultJobs),
		inflight:  map[string]*flight{},
		failed:    map[string]failure{},
		wake:      make(chan struct{}, 1),
	}

	for _, o := range opts {
		if err := o(s); err != nil {
			return nil, err
		}
	}

	for _, d := range []string{"originals", "variants", "tmp"} {
		if err := os.MkdirAll(filepath.Join(root, d), dirPerm); err != nil {
			return nil, fmt.Errorf("create image folder: %w", err)
		}
	}

	v, err := newVips(filepath.Join(root, "tmp"))
	if err != nil {
		return nil, err
	}

	s.vips = v

	return s, nil
}

func (s *Store) path(spec VariantSpec) string { return filepath.Join(s.root, spec.Path()) }

func (s *Store) originalPath(sha, mime string) string {
	return filepath.Join(s.root, "originals", sha[:2], sha+"."+extOf(mime))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Changed tells the store the catalogue changed: a photo or crop was
// written or deleted. Its in-memory view reloads on next use. Enqueue
// implies it.
func (s *Store) Changed() {
	s.vmu.Lock()
	s.viewStale = true
	s.vmu.Unlock()
}

// photo looks a sha up in the in-memory view of the catalogue, so serving a
// variant costs no query. The view reloads after Changed, and after
// viewMaxAge in case a caller forgot to say.
func (s *Store) photo(ctx context.Context, sha string) (Photo, error) {
	s.vmu.Lock()
	defer s.vmu.Unlock()

	if s.view == nil || s.viewStale || s.now().Sub(s.viewAt) > viewMaxAge {
		ps, err := s.cat.Photos(ctx)
		switch {
		case err != nil && s.view == nil:
			return Photo{}, fmt.Errorf("load catalogue: %w", err)
		case err != nil:
			s.log.Error("images: reload catalogue, serving the previous view", "err", err)
		default:
			s.view = make(map[string]Photo, len(ps))
			for _, p := range ps {
				s.view[p.SHA256] = p
			}

			s.viewAt, s.viewStale = s.now(), false
		}
	}

	p, ok := s.view[sha]
	if !ok {
		return Photo{}, fmt.Errorf("photo %s: %w", sha, ErrNotFound)
	}

	return p, nil
}

// Ref builds the Ref for a photo and kind from the catalogue.
func (s *Store) Ref(ctx context.Context, sha string, k Kind) (Ref, error) {
	p, err := s.photo(ctx, sha)
	if err != nil {
		return Ref{}, fmt.Errorf("ref: %w", err)
	}

	return RefFor(p, k)
}

// Ensure returns the file of a variant, building it first if it is missing.
// Only variants the catalogue currently expects are built; anything else is
// ErrNotFound, so no request can cause free-form work.
func (s *Store) Ensure(ctx context.Context, spec VariantSpec) (string, error) {
	f, _, err := s.open(ctx, spec)
	if err != nil {
		return "", err
	}

	return f.Name(), f.Close()
}

// open opens a variant, building it first if it is missing; built says
// which. A present file costs one open and no query.
func (s *Store) open(ctx context.Context, spec VariantSpec) (f *os.File, built bool, err error) {
	p, g, err := s.resolve(ctx, spec)
	if err != nil {
		return nil, false, err
	}

	path := s.path(spec)

	f, err = os.Open(path)
	if !errors.Is(err, fs.ErrNotExist) {
		return f, false, err
	}

	// Others may be waiting on this build, so a client that goes away must
	// not cancel it; the per-process timeout bounds it instead.
	if err := s.ensure(context.WithoutCancel(ctx), p, g, []VariantSpec{spec}); err != nil {
		return nil, true, err
	}

	f, err = os.Open(path)

	return f, true, err
}

func (s *Store) resolve(ctx context.Context, spec VariantSpec) (Photo, group, error) {
	p, err := s.photo(ctx, spec.Photo)
	if err != nil {
		return Photo{}, group{}, fmt.Errorf("variant %s: %w", spec.Name(), err)
	}

	for _, g := range groups(p) {
		if g.kind == spec.Kind && slices.Contains(g.specs, spec) {
			return p, g, nil
		}
	}

	return Photo{}, group{}, fmt.Errorf("variant %s is not current: %w", spec.Name(), ErrNotFound)
}

// ensure builds whichever of specs (all of group g) are missing. A spec
// already being built is waited for instead of built twice, and one that
// failed within failTTL is not tried again.
func (s *Store) ensure(ctx context.Context, p Photo, g group, specs []VariantSpec) error {
	var missing []VariantSpec
	for _, sp := range specs {
		if !exists(s.path(sp)) {
			missing = append(missing, sp)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	var (
		mine  []VariantSpec
		waits []*flight
		errs  []error
		now   = s.now()
	)

	// One flight per file, ended as each file lands, so a request waiting
	// for one file does not wait for the rest of a group the queue builds.
	flights := map[string]*flight{}

	s.mu.Lock()
	for _, sp := range missing {
		path := s.path(sp)
		if o := s.inflight[path]; o != nil {
			waits = append(waits, o)
			continue
		}

		if fl, ok := s.failed[path]; ok && now.Before(fl.until) {
			errs = append(errs, fmt.Errorf("failed recently, retry after %s: %w", fl.until.Format(time.TimeOnly), fl.err))
			continue
		}

		flights[path] = &flight{done: make(chan struct{})}
		s.inflight[path] = flights[path]
		mine = append(mine, sp)
	}

	for path, fl := range s.failed {
		if now.After(fl.until) {
			delete(s.failed, path)
		}
	}
	s.mu.Unlock()

	if len(mine) > 0 {
		land := func(sp VariantSpec, err error) { s.land(sp, flights[s.path(sp)], err, ctx.Err() != nil) }
		err := s.buildGroup(ctx, p, g, mine, land)

		// Files the build never reached: it failed before encoding.
		for _, sp := range mine {
			land(sp, err)
		}

		errs = append(errs, err)
	}

	for _, w := range waits {
		select {
		case <-w.done:
			errs = append(errs, w.err)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	n := 0
	for _, sp := range missing {
		if !exists(s.path(sp)) {
			n++
		}
	}

	if n > 0 {
		return fmt.Errorf("%d of %d %s variants of %s not built: %w", n, len(missing), g.kind, p.SHA256, errors.Join(errs...))
	}

	return nil
}

// land ends f, the flight of sp, once: it records a failure for a file
// that is not there and wakes whoever waits for it. A build stopped by its
// context is no failure: the next caller builds the file.
func (s *Store) land(sp VariantSpec, f *flight, err error, stopped bool) {
	path := s.path(sp)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.inflight[path] != f {
		return
	}

	delete(s.inflight, path)
	defer close(f.done)

	switch {
	case exists(path):
		delete(s.failed, path)
		return
	case err == nil:
		err = errors.New("build left no file")
	}

	f.err = err
	if !stopped {
		s.failed[path] = failure{err: err, until: s.now().Add(failTTL)}
	}
}

// buildGroup runs one build in a job slot.
func (s *Store) buildGroup(ctx context.Context, p Photo, g group, specs []VariantSpec, landed func(VariantSpec, error)) error {
	select {
	case s.jobs <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.jobs }()

	s.builds.Add(1)
	start := time.Now()
	log := s.log.With("photo", p.SHA256, "kind", g.kind, "crop", g.rect, "variants", len(specs))

	orig := s.originalPath(p.SHA256, p.MIME)
	_, err := os.Stat(orig)
	if err != nil {
		err = fmt.Errorf("original of %s: %w", p.SHA256, err)
	} else {
		err = s.vips.build(ctx, p, orig, g, specs, s.path, landed)
	}

	metricBuildSeconds.WithLabelValues(string(g.kind)).Observe(time.Since(start).Seconds())

	for _, sp := range specs {
		if exists(s.path(sp)) {
			metricBuilt.WithLabelValues(string(sp.Kind), string(sp.Format)).Inc()
		}
	}

	// A shutdown or an interrupted warm kills vips mid-build; that says
	// nothing about the photo, whatever vips's last words were.
	if err != nil && ctx.Err() != nil {
		log.Debug("image build stopped", "err", err)
		return ctx.Err()
	}

	if err != nil {
		metricBuildErrors.WithLabelValues(string(g.kind)).Inc()
		log.Error("image build failed", "orig", orig, "mime", p.MIME, "size", fmt.Sprintf("%dx%d", p.Width, p.Height), "err", err)

		return err
	}

	log.Debug("image build done", "took", time.Since(start))

	return nil
}
