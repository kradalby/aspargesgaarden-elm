package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/images"
)

type oneMissingPhoto struct{}

var lostSHA = strings.Repeat("c", 64)

func (oneMissingPhoto) Photos(context.Context) ([]images.Photo, error) {
	return []images.Photo{{SHA256: lostSHA, MIME: "image/jpeg", Width: 1200, Height: 800}}, nil
}

func (c oneMissingPhoto) Photo(ctx context.Context, _ string) (images.Photo, error) {
	ps, err := c.Photos(ctx)
	return ps[0], err
}

func TestImagesCmd(t *testing.T) {
	a := &app{log: slog.New(slog.DiscardHandler), lvl: new(slog.LevelVar), logLevel: "info"}
	run := func(args ...string) error {
		return newRootCmd(a).ParseAndRun(context.Background(), args)
	}

	// The real catalogue is the database: a fresh one has nothing to check.
	require.NoError(t, run("images", "check", "--data-dir", t.TempDir()))
	require.NoError(t, run("images", "gc", "--data-dir", t.TempDir()))

	dir := t.TempDir()
	wired := openImageCatalog
	openImageCatalog = func(context.Context, string) (images.Catalog, func() error, error) {
		return oneMissingPhoto{}, func() error { return nil }, nil
	}
	t.Cleanup(func() { openImageCatalog = wired })

	orphan := filepath.Join(dir, "tmp", "build-old")
	require.NoError(t, os.MkdirAll(filepath.Dir(orphan), 0o750))
	require.NoError(t, os.WriteFile(orphan, nil, 0o600))
	require.NoError(t, os.Chtimes(orphan, time.Unix(0, 0), time.Unix(0, 0)))

	err := run("images", "check", "--data-dir", dir)
	require.ErrorContains(t, err, "1 photos have no original")
	require.FileExists(t, orphan)

	err = run("images", "gc", "--data-dir", dir)
	require.ErrorContains(t, err, lostSHA)
	require.NoFileExists(t, orphan)

	require.ErrorContains(t, run("images", "warm", "--data-dir", dir), "original of "+lostSHA)
}
