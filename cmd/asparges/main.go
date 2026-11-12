// Command asparges serves aspargesgaarden.no. Public pages go out on a local
// listener that nginx exposes to the internet; metrics, debug and the
// admin app are served only on the service's own tsnet node.
//
// Flags may also be set through ASPARGES_-prefixed environment variables
// (e.g. ASPARGES_DATA_DIR). TS_AUTHKEY enrols the tsnet node unattended.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kradalby/kra/web"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"golang.org/x/sync/errgroup"
	"tailscale.com/net/tsaddr"

	"github.com/kradalby/asparges/booking"
	"github.com/kradalby/asparges/db"
	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/server"
	"github.com/kradalby/asparges/site"
	"github.com/kradalby/asparges/static"
)

const envPrefix = "ASPARGES"

func main() { os.Exit(run()) }

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	lvl := new(slog.LevelVar)
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
	slog.SetDefault(log)

	a := &app{log: log, lvl: lvl}
	cmd := newRootCmd(a)

	err := parse(cmd, os.Args[1:])
	if err == nil {
		err = cmd.Run(ctx)
	}

	switch {
	case err == nil:
		return 0
	case errors.Is(err, ff.ErrHelp):
		fmt.Fprintln(os.Stderr, ffhelp.Command(cmd.GetSelected()))
		return 0
	case errors.Is(err, ff.ErrNoExec):
		fmt.Fprintln(os.Stderr, ffhelp.Command(cmd.GetSelected()))
		return 2
	default:
		log.Error("asparges failed", "err", err)
		return 1
	}
}

// devOnlyFlags are for local work and the browser tests, never a deploy:
// the NixOS module does not pass them and the environment cannot set them,
// so an EnvironmentFile line cannot put the admin on the listener nginx
// exposes.
var devOnlyFlags = map[string]bool{
	"dev":          true, // no Tailscale, the admin on the local listener
	"e2e-fixtures": true, // hooks that delete everything
	"e2e-clock":    true, // a fixed day
}

var errDevFromEnv = errors.New("dev-only flags must be given on the command line")

func parse(cmd *ff.Command, args []string) error {
	for name := range devOnlyFlags {
		key := envPrefix + "_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
		if _, ok := os.LookupEnv(key); ok {
			return fmt.Errorf("%w: %s is set", errDevFromEnv, key)
		}
	}

	return cmd.Parse(args, ff.WithEnvVarPrefix(envPrefix))
}

type app struct {
	log      *slog.Logger
	lvl      *slog.LevelVar
	logLevel string
	serve    serveConfig
}

type serveConfig struct {
	dataDir     string
	listenAddr  string
	hostname    string
	baseURL     string
	authKeyPath string
	dev         bool
	noindex     bool
	umamiSiteID string
	calendarURL string
	e2eFixtures string
	e2eClock    bool
}

func newRootCmd(a *app) *ff.Command {
	fs := ff.NewFlagSet("asparges")
	fs.StringVar(&a.logLevel, 0, "log-level", "info", "log level: debug, info, warn or error")

	return &ff.Command{
		Name:        "asparges",
		Usage:       "asparges <SUBCOMMAND> [FLAGS]",
		Flags:       fs,
		Subcommands: []*ff.Command{newServeCmd(a, fs), newImportCmd(a, fs)},
	}
}

func newServeCmd(a *app, parent *ff.FlagSet) *ff.Command {
	cfg := &a.serve
	fs := ff.NewFlagSet("serve").SetParent(parent)
	fs.StringVar(&cfg.dataDir, 0, "data-dir", "/var/lib/asparges", "state directory: database, photos, tsnet identity")
	fs.StringVar(&cfg.listenAddr, 0, "listen-addr", "127.0.0.1:8090", "local listener for the public site (nginx proxies to it)")
	fs.StringVar(&cfg.hostname, 0, "hostname", "asparges", "tailnet hostname of the admin node")
	fs.StringVar(&cfg.baseURL, 0, "base-url", "https://aspargesgaarden.no", "public origin used for absolute links, without a trailing slash")
	fs.StringVar(&cfg.authKeyPath, 0, "tailscale-auth-key-path", "", "file holding the Tailscale auth key (TS_AUTHKEY takes precedence)")
	fs.BoolVar(&cfg.dev, 0, "dev", "local development: no Tailscale, metrics and admin on the loopback-only local listener")
	fs.BoolVar(&cfg.noindex, 0, "noindex", "keep the site out of search engines (beta): noindex on every response, robots.txt disallows all, no IndexNow")
	fs.StringVar(&cfg.umamiSiteID, 0, "umami-site-id", "", "the site's id on the Umami server; empty puts no tracker on the pages")
	fs.StringVar(&cfg.calendarURL, 0, "calendar-url", "", "public iCal address of the booking calendar, read every quarter hour; empty shows no calendar")
	fs.BoolVar(&cfg.e2eClock, 0, "e2e-clock", "with --dev: run the clock from the browser tests' fixed day, as --e2e-fixtures does")
	fs.StringVar(&cfg.e2eFixtures, 0, "e2e-fixtures", "", "with --dev: the admin browser tests' photo folder; mounts their hooks POST /__reset (deletes everything), /__touch/{id} and /__crop/{id}")

	return &ff.Command{
		Name:      "serve",
		Usage:     "asparges serve [FLAGS]",
		ShortHelp: "serve the public site and the tailnet admin",
		Flags:     fs,
		Exec: func(ctx context.Context, _ []string) error {
			err := a.lvl.UnmarshalText([]byte(a.logLevel))
			if err != nil {
				return fmt.Errorf("invalid --log-level %q: %w", a.logLevel, err)
			}

			return serve(ctx, a.log, *cfg)
		},
	}
}

var (
	errBaseURL     = errors.New("--base-url must be an http(s) origin without path, query or trailing slash")
	errDevLoopback = errors.New("--dev serves metrics on the local listener, so --listen-addr must be a loopback address")
	errDevOnly     = errors.New("--e2e-fixtures and --e2e-clock need --dev")
	errCalendarURL = errors.New("--calendar-url must be an http(s) address")
)

func (c serveConfig) validate() error {
	u, err := url.Parse(c.baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return fmt.Errorf("%w: %q", errBaseURL, c.baseURL)
	}

	if c.dev && !isLoopback(c.listenAddr) {
		return fmt.Errorf("%w: %q", errDevLoopback, c.listenAddr)
	}

	if !c.dev && (c.e2eFixtures != "" || c.e2eClock) {
		return errDevOnly
	}

	if c.calendarURL != "" {
		u, err := url.Parse(c.calendarURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: %q", errCalendarURL, c.calendarURL)
		}
	}

	return nil
}

// indexNowEndpoint is where page changes are announced: nowhere from a
// laptop or from the beta site, which must stay out of search engines.
func (c serveConfig) indexNowEndpoint() string {
	if c.dev || c.noindex {
		return ""
	}

	return seo.IndexNowEndpoint
}

func isLoopback(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return false
	}

	if host == "localhost" {
		return true
	}

	ip, err := netip.ParseAddr(host)

	return err == nil && ip.IsLoopback()
}

func serve(ctx context.Context, log *slog.Logger, cfg serveConfig) error {
	err := cfg.validate()
	if err != nil {
		return err
	}

	srv, err := web.NewServer(web.ServerConfig{
		Hostname:        cfg.hostname,
		LocalAddr:       cfg.listenAddr,
		AuthKey:         os.Getenv("TS_AUTHKEY"),
		AuthKeyPath:     cfg.authKeyPath,
		EnableTailscale: !cfg.dev,
	}, web.WithLogger(log),
		// tsnet defaults its state to $HOME/.config, which is /var/empty under
		// the hardened unit; keep it in the writable state directory instead.
		web.WithTailscaleStateDir(filepath.Join(cfg.dataDir, "tsnet")))
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	d, err := openData(ctx, cfg.dataDir)
	if err != nil {
		return err
	}
	defer d.Close()

	now := time.Now

	var e2eClk *e2eClock
	if cfg.e2eFixtures != "" || cfg.e2eClock {
		e2eClk = &e2eClock{}
		now = e2eClk.now
	}

	store := site.NewStore(d, now)

	imgs, err := images.New(cfg.dataDir, store.Catalog(), images.WithLogger(log))
	if err != nil {
		return fmt.Errorf("photo store: %w", err)
	}

	key, err := store.IndexNowKey(ctx)
	if err != nil {
		return err
	}

	admin := server.AdminConfig{Photos: imgs}

	if !cfg.dev {
		lc := srv.TailscaleLocalClient()
		if lc == nil {
			return errors.New("tailscale local client unavailable: the admin cannot identify callers")
		}

		admin.Tailnet = lc
	}

	var public *server.Server

	var cal *booking.Feed
	if cfg.calendarURL != "" {
		cal = booking.Open(booking.Config{
			URL:  cfg.calendarURL,
			Path: filepath.Join(cfg.dataDir, booking.FileName),
			Now:  now,
			Log:  log,
			// Run, the only caller, starts once public is set.
			Changed: func() { public.Changed() },
		})
	}

	public = server.New(server.Config{
		BaseURL:          cfg.baseURL,
		Dev:              cfg.dev,
		Store:            store,
		Images:           imgs,
		Now:              now,
		Log:              log,
		NoIndex:          cfg.noindex,
		IndexNowKey:      key,
		IndexNowEndpoint: cfg.indexNowEndpoint(),
		Admin:            admin,
		UmamiSiteID:      cfg.umamiSiteID,
		Calendar:         cal,
	})
	srv.Handle("/", adminFilesOnTailnet(public.Public()))

	// In --dev, Public carries the admin on the loopback listener.
	if !cfg.dev {
		srv.HandleTSOnly("/admin/", public.Admin())
	}

	if cfg.e2eFixtures != "" {
		hooks := e2eHooks(store, imgs, public, cfg.e2eFixtures, e2eClk)
		srv.Handle("POST /__reset", hooks)
		srv.Handle("POST /__touch/{id}", hooks)
		srv.Handle("POST /__crop/{id}", hooks)
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return srv.ListenAndServe(ctx) })
	g.Go(func() error { return imgs.Run(ctx) })
	g.Go(func() error { return public.Run(ctx) })
	g.Go(func() error { return sweepUnusedPhotos(ctx, log, store, public) })

	if cal != nil {
		g.Go(func() error { return cal.Run(ctx) })
	}

	return g.Wait()
}

// adminFilesOnTailnet keeps h to /static/ and /img/, what the admin app
// loads, for requests that arrived over the tailnet. kra's Handle mounts h
// on the tsnet listener too, and a public page there, with its third-party
// script, would share the admin's origin.
func adminFilesOnTailnet(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if overTailnet(r) && !strings.HasPrefix(p, static.Prefix) && !strings.HasPrefix(p, "/img/") {
			http.NotFound(w, r)
			return
		}

		h.ServeHTTP(w, r)
	})
}

// overTailnet reports whether r was sent to one of the node's tailnet
// addresses: tsnet's listener has only those, the local one never.
// shortcut: assumes the tailnet uses Tailscale's own ranges (100.64.0.0/10,
// fd7a:115c:a1e0::/48); drop for a local-only Handle once kra has one.
func overTailnet(r *http.Request) bool {
	a, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if a == nil {
		return false
	}

	ap, err := netip.ParseAddrPort(a.String())

	return err == nil && tsaddr.IsTailscaleIP(ap.Addr().Unmap())
}

// unusedPhotoAge is how long a photo no event or gallery uses is kept after
// its last upload or edit, or after it was taken out of use: a form left
// unsaved over a week's break still finds its photos, and a photo removed
// by mistake can be put back.
const unusedPhotoAge = 7 * 24 * time.Hour

// sweepEvery spaces the unused-photo sweeps after the one at start.
const sweepEvery = 24 * time.Hour

// sweepUnusedPhotos deletes unused photos at start and every sweepEvery.
// Changed hands their sha256s to the photo store, which removes their files.
func sweepUnusedPhotos(ctx context.Context, log *slog.Logger, store *site.Store, srv *server.Server) error {
	t := time.NewTicker(sweepEvery)
	defer t.Stop()

	for {
		sweepOnce(ctx, log, store, srv)

		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func sweepOnce(ctx context.Context, log *slog.Logger, store *site.Store, srv *server.Server) {
	gone, err := store.DeleteUnusedPhotos(ctx, unusedPhotoAge)

	switch {
	case err != nil && ctx.Err() == nil:
		log.ErrorContext(ctx, "delete unused photos", "err", err)
	case len(gone) > 0:
		log.InfoContext(ctx, "deleted unused photos", "count", len(gone), "sha256", gone)
		srv.Changed(gone...)
	}
}

// openData opens the database in the state directory, creating both.
func openData(ctx context.Context, dataDir string) (*db.DB, error) {
	// The unit's StateDirectoryMode; group access is for litestream.
	if err := os.MkdirAll(dataDir, 0o770); err != nil { //nolint:gosec // see above
		return nil, fmt.Errorf("creating %s: %w", dataDir, err)
	}

	// SQLite would create the database 0644, and gives -wal and -shm the
	// database's mode; litestream, in the group, writes all three. Chmod,
	// not the create mode alone: the caller's umask cuts that, and a seeded
	// or restored database arrives with whatever mode it had.
	path := filepath.Join(dataDir, "asparges.db")

	f, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o660) //nolint:gosec // see above
	if err != nil {
		return nil, fmt.Errorf("creating database: %w", err)
	}

	err = f.Chmod(0o660) //nolint:gosec // see above
	_ = f.Close()

	if err != nil {
		return nil, fmt.Errorf("making the database group-writable: %w", err)
	}

	d, err := db.Open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	return d, nil
}
