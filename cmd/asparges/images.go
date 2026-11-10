package main

import (
	"context"
	"fmt"
	"time"

	"github.com/peterbourgon/ff/v4"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/site"
)

// openImageCatalog opens the database's view of photos for the images
// commands. Replaced in tests.
var openImageCatalog = func(ctx context.Context, dataDir string) (images.Catalog, func() error, error) {
	d, err := openData(ctx, dataDir)
	if err != nil {
		return nil, nil, err
	}

	return site.NewStore(d, time.Now).Catalog(), d.Close, nil
}

func newImagesCmd(a *app, parent *ff.FlagSet) *ff.Command {
	var dataDir string

	fs := ff.NewFlagSet("images").SetParent(parent)
	fs.StringVar(&dataDir, 0, "data-dir", "/var/lib/asparges", "state directory: database and photos")

	sub := func(name, help string, run func(context.Context, *images.Store) error) *ff.Command {
		return &ff.Command{
			Name:      name,
			Usage:     "asparges images " + name + " [FLAGS]",
			ShortHelp: help,
			Flags:     ff.NewFlagSet(name).SetParent(fs),
			Exec: func(ctx context.Context, _ []string) error {
				if err := a.lvl.UnmarshalText([]byte(a.logLevel)); err != nil {
					return fmt.Errorf("invalid --log-level %q: %w", a.logLevel, err)
				}

				cat, closeCat, err := openImageCatalog(ctx, dataDir)
				if err != nil {
					return err
				}
				defer closeCat() //nolint:errcheck // read-only use

				s, err := images.New(dataDir, cat, images.WithLogger(a.log))
				if err != nil {
					return err
				}

				return run(ctx, s)
			},
		}
	}

	return &ff.Command{
		Name:      "images",
		Usage:     "asparges images <check|gc|warm> [FLAGS]",
		ShortHelp: "inspect, clean and fill the photo folders",
		Flags:     fs,
		Subcommands: []*ff.Command{
			sub("check", "report what gc would delete and what is missing; change nothing",
				func(ctx context.Context, s *images.Store) error { return reconcile(ctx, a, s, true) }),
			sub("gc", "delete variants, work files and originals the database does not explain",
				func(ctx context.Context, s *images.Store) error { return reconcile(ctx, a, s, false) }),
			sub("warm", "build every missing variant now", func(ctx context.Context, s *images.Store) error {
				return s.WarmAll(ctx)
			}),
		},
	}
}

func reconcile(ctx context.Context, a *app, s *images.Store, dryRun bool) error {
	rep, err := s.Reconcile(ctx, dryRun)
	if err != nil {
		return err
	}

	a.log.Info("images reconciled", rep.Attrs()...)

	if n := len(rep.MissingOriginals); n > 0 {
		return fmt.Errorf("%d photos have no original; restore originals/ from backup: %v", n, rep.MissingOriginals)
	}

	return nil
}
