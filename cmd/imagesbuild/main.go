// Command imagesbuild runs the images pipeline over the photo masters at
// build time, for the photos embedded in asparges. It is a binary of its own
// because asparges embeds its output, so cannot be the tool that makes it.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/kradalby/asparges/images"
)

func main() { os.Exit(run()) }

func run() int {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	fs := ff.NewFlagSet("imagesbuild")
	src := fs.StringLong("src", "static/photos-src", "folder of masters; a master's file name without extension names it")
	out := fs.StringLong("out", "", "output folder for the variants and "+images.ManifestFile)
	prefix := fs.StringLong("url-prefix", "/static/photos", "URL path the output folder is served at")

	err := ff.Parse(fs, os.Args[1:])
	if err != nil || *out == "" {
		fmt.Fprintln(os.Stderr, ffhelp.Flags(fs))

		if errors.Is(err, ff.ErrHelp) {
			return 0
		}

		return 2
	}

	refs, err := images.BuildStatic(context.Background(), *src, *out, *prefix)
	if err != nil {
		log.Error("imagesbuild failed", "err", err)
		return 1
	}

	log.Info("imagesbuild done", "photos", len(refs), "out", *out)

	return 0
}
