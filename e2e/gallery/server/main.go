// Command server serves the gallery's browser tests (../run.sh) with the
// wiring of `asparges serve` (site.Store, images.Store, server.New) over a
// data directory `asparges import` filled from the old site, plus /e2e/two:
// a page with two galleries, which no real page has yet.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chasefleming/elem-go"
	"golang.org/x/sync/errgroup"

	"github.com/kradalby/asparges/db"
	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/server"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/ui"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8095", "listen address")
	data := flag.String("data-dir", "", "state directory `asparges import` filled")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, *addr, *data)

	cancel()

	if err != nil {
		slog.Error("e2e server", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, addr, dataDir string) error {
	d, err := db.Open(ctx, filepath.Join(dataDir, "asparges.db"))
	if err != nil {
		return err
	}
	defer d.Close()

	store := site.NewStore(d, time.Now)

	imgs, err := images.New(dataDir, store.Catalog(), images.WithJobs(8))
	if err != nil {
		return err
	}

	gallery, err := store.Gallery(ctx)
	if err != nil {
		return err
	}

	market, err := store.PublishedEvent(ctx, 2026, "julemarked")
	if err != nil {
		return fmt.Errorf("the import's 2026 market: %w", err)
	}

	err = warm(ctx, imgs, append(gallery, market.ExtraPhotos...))
	if err != nil {
		return err
	}

	two := twoGalleries(gallery[:3], market.ExtraPhotos)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /e2e/two", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Security-Policy", ui.CSP(ui.Umami{}))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(two))
	})
	mux.Handle("/", server.New(server.Config{BaseURL: "http://" + addr, Store: store, Images: imgs, Now: time.Now}).Public())

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	fmt.Println("ready", len(gallery), "gallery photos") //nolint:forbidigo // run.sh waits for this line

	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

// warm builds the full variants (the kind galleries show) before the tests
// start, so a first run's encoding does not race their timeouts. Built
// files stay in the data directory for the next run.
func warm(ctx context.Context, imgs *images.Store, photos []site.Photo) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, p := range photos {
		ref, err := images.RefFor(p.Image(), images.Full)
		if err != nil {
			return err
		}

		for _, v := range ref.Variants {
			spec, err := images.ParseURL(v.URL)
			if err != nil {
				return err
			}

			g.Go(func() error {
				_, err := imgs.Ensure(ctx, spec)

				return err
			})
		}
	}

	return g.Wait()
}

// twoGalleries is a page with a gallery of each layout.
func twoGalleries(a, b []site.Photo) string {
	return ui.Page(ui.PageProps{BaseURL: "http://localhost", Meta: ui.Meta{Title: "To gallerier"}},
		ui.Container(
			elem.Div(nil, ui.Headline("To gallerier")),
			ui.Gallery(ui.GalleryPage, shown(a)...),
			elem.P(nil, elem.Text("Tekst mellom galleriene.")),
			ui.EventColumn(ui.Gallery(ui.GalleryColumn, shown(b)...)),
		),
	).Render()
}

func shown(photos []site.Photo) []ui.GalleryPhoto {
	out := make([]ui.GalleryPhoto, len(photos))

	for i, p := range photos {
		ref, _ := images.RefFor(p.Image(), images.Full) // warm has built these
		out[i] = ui.GalleryPhoto{Photo: ref, Alt: p.Alt}

		if out[i].Alt == "" {
			out[i].Alt = "Bilde"
		}
	}

	return out
}
