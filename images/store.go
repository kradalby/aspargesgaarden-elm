package images

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// jobs is how many groups build at once: the worker's and an upload's.
	// Each runs one vips process at a time with vipsThreads threads, and
	// every child counts against the unit's MemoryMax.
	jobs    = 2
	dirPerm = 0o750
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
	maxUpload int64
	slots     chan struct{} // see jobs
	builds    atomic.Int64  // group builds started; tests read it

	mu      sync.Mutex
	queue   []string // photos for the worker, next first; see Enqueue
	asked   int      // how many at the front of queue someone asked for
	running string   // the photo the worker is on
	// runningAsked says whether someone asked for running; see yield.
	runningAsked bool
	failed       map[string]error // a photo's last failure, until a success or Enqueue
	wake         chan struct{}

	// files orders Intake's storing of an original against forget.
	files sync.Mutex
	fresh map[string]bool // stored by Intake, row not yet seen; see forget
}

// Option configures a Store.
type Option func(*Store) error

// WithLogger sets the logger; the default discards.
func WithLogger(l *slog.Logger) Option {
	return func(s *Store) error { s.log = l; return nil }
}

// New opens the store at root, creating its folders. It fails when the vips
// CLI is missing, so a broken deploy stops at start rather than on the first
// photo.
func New(root string, cat Catalog, opts ...Option) (*Store, error) {
	s := &Store{
		root:      root,
		cat:       cat,
		log:       slog.New(slog.DiscardHandler),
		maxUpload: MaxUpload,
		slots:     make(chan struct{}, jobs),
		failed:    map[string]error{},
		fresh:     map[string]bool{},
		wake:      make(chan struct{}, 1),
	}

	for _, o := range opts {
		if err := o(s); err != nil {
			return nil, err
		}
	}

	if err := clean(root); err != nil {
		return nil, err
	}

	v, err := newVips(filepath.Join(root, "tmp"))
	if err != nil {
		return nil, err
	}

	s.vips = v

	return s, nil
}

// clean creates the folders and empties what no running process can own
// before New returns: work files of a killed one, and variants of another
// pipeline version, which nothing links to.
func clean(root string) error {
	for _, d := range []string{"originals", "variants", "tmp"} {
		if err := os.MkdirAll(filepath.Join(root, d), dirPerm); err != nil {
			return fmt.Errorf("create image folder: %w", err)
		}
	}

	for _, d := range []string{"tmp", "variants"} {
		entries, err := os.ReadDir(filepath.Join(root, d))
		if err != nil {
			return fmt.Errorf("clean image folder: %w", err)
		}

		for _, e := range entries {
			if d == "variants" && e.Name() == fmt.Sprintf("v%d", PipelineVersion) {
				continue
			}

			if err := os.RemoveAll(filepath.Join(root, d, e.Name())); err != nil {
				return fmt.Errorf("clean image folder: %w", err)
			}
		}
	}

	return nil
}

func (s *Store) path(spec VariantSpec) string { return filepath.Join(s.root, spec.Path()) }

func (s *Store) originalPath(sha, mime string) string {
	return filepath.Join(s.root, "originals", sha[:2], sha+"."+extOf(mime))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// buildGroup builds specs, all of group g of photo p, from the original at
// orig, in a job slot.
func (s *Store) buildGroup(ctx context.Context, p Photo, orig string, g group, specs []VariantSpec) error {
	select {
	case s.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.slots }()

	s.builds.Add(1)
	start := time.Now()

	err := s.vips.build(ctx, p, orig, g, specs, s.path)

	metricBuildSeconds.WithLabelValues(string(g.kind)).Observe(time.Since(start).Seconds())

	for _, sp := range specs {
		if exists(s.path(sp)) {
			metricBuilt.WithLabelValues(string(sp.Kind), string(sp.Format)).Inc()
		}
	}

	if err != nil && ctx.Err() == nil {
		metricBuildErrors.WithLabelValues(string(g.kind)).Inc()
	}

	s.log.Debug("images: group built", "photo", p.SHA256, "kind", g.kind, "variants", len(specs), "took", time.Since(start), "err", err)

	return err
}
