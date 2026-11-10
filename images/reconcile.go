package images

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	// tmpMaxAge is far beyond any build (processTimeout per step) or upload.
	tmpMaxAge = time.Hour
	// originalGrace covers the gap between Intake and the photos row, and an
	// admin who uploads, leaves and comes back to finish the event.
	originalGrace = 7 * 24 * time.Hour
	// freshSlack: a variant written after the catalogue was read may belong
	// to a crop newer than that read, so it is not judged this time.
	freshSlack = time.Minute
)

// Report is what Reconcile found and, unless DryRun, removed.
type Report struct {
	DryRun          bool
	OrphanVariants  int      // variant files of no current photo, crop or width
	OldPipelines    []string // variants/<dir> other than the current version
	StaleTmp        int      // tmp/ entries older than tmpMaxAge
	OrphanOriginals []string // sha256 of originals without a row, past the grace period
	GraceOriginals  int      // originals without a row, still inside the grace period
	Freed           int64    // bytes in the removed files
	// MissingOriginals are photos whose original is gone. Only a restore
	// helps; their rows are never touched.
	MissingOriginals []string
	MissingVariants  int      // expected files not built yet; the queue builds them
	Unknown          []string // files in originals/ the naming scheme does not explain; left alone
}

// Attrs returns the report as slog key-value pairs.
func (r Report) Attrs() []any {
	return []any{
		"dry_run", r.DryRun,
		"orphan_variants", r.OrphanVariants,
		"old_pipelines", r.OldPipelines,
		"stale_tmp", r.StaleTmp,
		"orphan_originals", len(r.OrphanOriginals),
		"grace_originals", r.GraceOriginals,
		"freed_bytes", r.Freed,
		"missing_originals", len(r.MissingOriginals),
		"missing_variants", r.MissingVariants,
		"unknown", len(r.Unknown),
	}
}

var originalRE = regexp.MustCompile(`^([0-9a-f]{2})/([0-9a-f]{64})\.(jpg|png|webp|heic|avif)$`)

// Reconcile makes the folders match the catalogue: it deletes variant files
// the catalogue does not expect (old crops, deleted photos, old pipeline
// versions), work files older than tmpMaxAge, and originals without a photo
// past originalGrace. Photos whose original is missing are reported, never
// fixed. With dryRun nothing is deleted.
func (s *Store) Reconcile(ctx context.Context, dryRun bool) (Report, error) {
	start := s.now()
	rep := Report{DryRun: dryRun}

	// Never act on a partial view: no list, no deletes.
	photos, err := s.cat.Photos(ctx)
	if err != nil {
		return rep, fmt.Errorf("reconcile: list photos: %w", err)
	}

	known := map[string]bool{}
	expected := map[string]bool{}
	for _, p := range photos {
		known[p.SHA256] = true
		for _, sp := range Expected(p) {
			expected[sp.Path()] = true
		}
	}

	remove := func(dir, path string, size int64) {
		rep.Freed += size
		if dryRun {
			return
		}

		if err := os.RemoveAll(path); err != nil {
			s.log.Error("images: remove", "path", path, "err", err)
			return
		}

		metricOrphansRemoved.WithLabelValues(dir).Inc()
	}

	if err := s.reconcileVariants(start, expected, &rep, remove); err != nil {
		return rep, err
	}

	if err := s.reconcileTmp(start, &rep, remove); err != nil {
		return rep, err
	}

	held, err := s.reconcileOriginals(start, known, len(photos) == 0, &rep, remove)
	if err != nil {
		return rep, err
	}

	for _, p := range photos {
		if orig := s.originalPath(p.SHA256, p.MIME); !exists(orig) {
			rep.MissingOriginals = append(rep.MissingOriginals, p.SHA256)
			s.log.Error("images: original missing, restore it from backup", "photo", p.SHA256, "path", orig)
		}
	}

	if !dryRun {
		metricMissingOriginals.Set(float64(len(rep.MissingOriginals)))
	}

	if held > 0 {
		return rep, fmt.Errorf("reconcile: the catalogue lists no photos but %d originals are past the grace period; kept them, check the database", held)
	}

	return rep, nil
}

type removeFunc func(dir, path string, size int64)

func (s *Store) reconcileVariants(start time.Time, expected map[string]bool, rep *Report, remove removeFunc) error {
	root := filepath.Join(s.root, "variants")
	cur := fmt.Sprintf("v%d", PipelineVersion)

	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}

	for _, e := range entries {
		if e.Name() == cur {
			continue
		}

		path := filepath.Join(root, e.Name())
		rep.OldPipelines = append(rep.OldPipelines, e.Name())
		remove("variants", path, du(path))
	}

	var (
		files, size int64
		found       int
		dirs        []string
	)

	err = filepath.WalkDir(filepath.Join(root, cur), func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		if err != nil {
			return err
		}

		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}

		switch {
		case expected[rel]:
			found++
		case info.ModTime().After(start.Add(-freshSlack)):
		default:
			rep.OrphanVariants++
			remove("variants", path, info.Size())

			return nil
		}

		files++
		size += info.Size()

		return nil
	})
	if err != nil {
		return fmt.Errorf("reconcile variants: %w", err)
	}

	// Deepest first; removing a folder that still holds files fails, which
	// is the point. The version folder itself stays.
	if !rep.DryRun {
		for i := len(dirs) - 1; i > 0; i-- {
			_ = os.Remove(dirs[i])
		}

		metricFiles.WithLabelValues("variants").Set(float64(files))
		metricBytes.WithLabelValues("variants").Set(float64(size))
	}

	rep.MissingVariants = len(expected) - found

	return nil
}

func (s *Store) reconcileTmp(start time.Time, rep *Report, remove removeFunc) error {
	root := filepath.Join(s.root, "tmp")

	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}

	var files, size int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue // removed by its build meanwhile
		}

		path := filepath.Join(root, e.Name())
		n := du(path)

		if info.ModTime().Before(start.Add(-tmpMaxAge)) {
			rep.StaleTmp++
			remove("tmp", path, n)

			continue
		}

		files++
		size += n
	}

	if !rep.DryRun {
		metricFiles.WithLabelValues("tmp").Set(float64(files))
		metricBytes.WithLabelValues("tmp").Set(float64(size))
	}

	return nil
}

// reconcileOriginals returns how many orphans it kept because the catalogue
// was empty: that is far more likely a wiring fault than a deliberate wipe,
// and originals cannot be rebuilt.
func (s *Store) reconcileOriginals(start time.Time, known map[string]bool, emptyCatalog bool, rep *Report, remove removeFunc) (int, error) {
	root := filepath.Join(s.root, "originals")

	var (
		files, size int64
		held        int
	)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		m := originalRE.FindStringSubmatch(filepath.ToSlash(rel))
		if m == nil || m[1] != m[2][:2] {
			rep.Unknown = append(rep.Unknown, rel)
			return nil
		}

		files++
		size += info.Size()

		switch {
		case known[m[2]]:
		case info.ModTime().After(start.Add(-originalGrace)):
			rep.GraceOriginals++
		case emptyCatalog:
			held++
		default:
			rep.OrphanOriginals = append(rep.OrphanOriginals, m[2])
			remove("originals", path, info.Size())
			files--
			size -= info.Size()
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("reconcile originals: %w", err)
	}

	if !rep.DryRun {
		metricFiles.WithLabelValues("originals").Set(float64(files))
		metricBytes.WithLabelValues("originals").Set(float64(size))
	}

	return held, nil
}

// du is the size of a file or everything below a folder.
func du(path string) int64 {
	var n int64

	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}

		return nil
	})

	return n
}
