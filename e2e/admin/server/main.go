// Command server serves the admin's browser tests (../run.sh): the real admin
// document (pages.AdminHandler) and the embedded files (static.Handler), as
// the tailnet listener will, with everything else passed to the fake API
// (../fake-server.mjs). When the real API exists, the tests point at
// `asparges serve --dev` instead and this goes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kradalby/asparges/pages"
	"github.com/kradalby/asparges/static"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18096", "listen address")
	fake := flag.String("fake", "http://127.0.0.1:18097", "the fake admin API")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, *addr, *fake)

	cancel()

	if err != nil {
		slog.Error("e2e admin server", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, addr, fake string) error {
	api, err := url.Parse(fake)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("GET "+static.Prefix, static.Handler())
	mux.Handle("/admin/", pages.AdminHandler())
	mux.Handle("/admin/api/", httputil.NewSingleHostReverseProxy(api))
	// The fake's photo files and its test hooks.
	mux.Handle("/", httputil.NewSingleHostReverseProxy(api))

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	fmt.Println("ready") //nolint:forbidigo // run.sh waits for this line

	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
