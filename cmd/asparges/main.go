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
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kradalby/kra/web"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"golang.org/x/sync/errgroup"

	"github.com/kradalby/asparges/db"
	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/pages"
	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/server"
	"github.com/kradalby/asparges/site"
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

	err := cmd.ParseAndRun(ctx, os.Args[1:], ff.WithEnvVarPrefix(envPrefix))
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
	e2eFixtures string
}

func newRootCmd(a *app) *ff.Command {
	fs := ff.NewFlagSet("asparges")
	fs.StringVar(&a.logLevel, 0, "log-level", "info", "log level: debug, info, warn or error")

	return &ff.Command{
		Name:        "asparges",
		Usage:       "asparges <SUBCOMMAND> [FLAGS]",
		Flags:       fs,
		Subcommands: []*ff.Command{newServeCmd(a, fs), newImagesCmd(a, fs), newImportCmd(a, fs)},
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
	errDevOnly     = errors.New("--e2e-fixtures needs --dev: its hooks delete everything")
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

	if !c.dev && c.e2eFixtures != "" {
		return errDevOnly
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
	if cfg.e2eFixtures != "" {
		e2eClk = &e2eClock{}
		now = e2eClk.now
	}

	store := site.NewStore(d, now)

	imgs, err := images.New(cfg.dataDir, store.Catalog(), images.WithLogger(log),
		images.WithSweep(func(ctx context.Context) error { return sweepUnusedPhotos(ctx, log, store) }))
	if err != nil {
		return fmt.Errorf("photo store: %w", err)
	}

	key, err := store.IndexNowKey(ctx)
	if err != nil {
		return err
	}

	admin := server.AdminConfig{Photos: imgs, App: pages.AdminHandler()}

	if !cfg.dev {
		lc := srv.TailscaleLocalClient()
		if lc == nil {
			return errors.New("tailscale local client unavailable: the admin cannot identify callers")
		}

		admin.Tailnet = lc
	}

	public := server.New(server.Config{
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
	})
	srv.Handle("/", public.Public())

	// In --dev, Public carries the admin on the loopback listener.
	if !cfg.dev {
		srv.HandleTSOnly("/admin/", public.Admin())
	}

	if e2eClk != nil {
		hooks := e2eHooks(store, imgs, public, cfg.e2eFixtures, e2eClk)
		srv.Handle("POST /__reset", hooks)
		srv.Handle("POST /__touch/{id}", hooks)
		srv.Handle("POST /__crop/{id}", hooks)
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return srv.ListenAndServe(ctx) })
	g.Go(func() error { return imgs.Run(ctx) })
	g.Go(func() error { return public.Run(ctx) })

	return g.Wait()
}

// unusedPhotoAge is how long a photo no event or gallery uses is kept after
// its last upload, crop or description: a form left unsaved over a week's
// break still finds its photos.
const unusedPhotoAge = 7 * 24 * time.Hour

// sweepUnusedPhotos runs before each reconcile, so the photo store removes
// the deleted photos' files in the same pass.
func sweepUnusedPhotos(ctx context.Context, log *slog.Logger, store *site.Store) error {
	gone, err := store.DeleteUnusedPhotos(ctx, unusedPhotoAge)
	if err != nil {
		return err
	}

	if len(gone) > 0 {
		log.InfoContext(ctx, "deleted unused photos", "count", len(gone), "sha256", gone)
	}

	return nil
}

// openData opens the database in the state directory, creating both.
func openData(ctx context.Context, dataDir string) (*db.DB, error) {
	// The unit's StateDirectoryMode; group access is for litestream.
	if err := os.MkdirAll(dataDir, 0o770); err != nil { //nolint:gosec // see above
		return nil, fmt.Errorf("creating %s: %w", dataDir, err)
	}

	d, err := db.Open(ctx, filepath.Join(dataDir, "asparges.db"))
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	return d, nil
}
