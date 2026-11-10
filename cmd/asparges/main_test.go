package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/peterbourgon/ff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/db"
	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/seo"
	"github.com/kradalby/asparges/server"
	"github.com/kradalby/asparges/site"
)

func TestServeFlags(t *testing.T) {
	defaults := serveConfig{
		dataDir:    "/var/lib/asparges",
		listenAddr: "127.0.0.1:8090",
		hostname:   "asparges",
		baseURL:    "https://aspargesgaarden.no",
	}

	tests := []struct {
		name string
		args []string
		env  map[string]string
		want func(c *serveConfig)
	}{
		{
			name: "defaults",
			args: []string{"serve"},
			want: func(*serveConfig) {},
		},
		{
			name: "flags",
			args: []string{
				"serve", "--dev", "--data-dir=/tmp/a", "--listen-addr=localhost:1",
				"--hostname=beta", "--base-url=https://beta.aspargesgaarden.no",
				"--tailscale-auth-key-path=/run/key",
			},
			want: func(c *serveConfig) {
				c.dev = true
				c.dataDir = "/tmp/a"
				c.listenAddr = "localhost:1"
				c.hostname = "beta"
				c.baseURL = "https://beta.aspargesgaarden.no"
				c.authKeyPath = "/run/key"
			},
		},
		{
			name: "env",
			args: []string{"serve"},
			env: map[string]string{
				"ASPARGES_DEV":         "true",
				"ASPARGES_DATA_DIR":    "/srv/asparges",
				"ASPARGES_LISTEN_ADDR": "127.0.0.1:2",
			},
			want: func(c *serveConfig) {
				c.dev = true
				c.dataDir = "/srv/asparges"
				c.listenAddr = "127.0.0.1:2"
			},
		},
		{
			name: "flag-beats-env",
			args: []string{"serve", "--hostname=flag"},
			env:  map[string]string{"ASPARGES_HOSTNAME": "env"},
			want: func(c *serveConfig) { c.hostname = "flag" },
		},
		{
			name: "noindex",
			args: []string{"serve"},
			env:  map[string]string{"ASPARGES_NOINDEX": "true"},
			want: func(c *serveConfig) { c.noindex = true },
		},
		{
			name: "umami",
			args: []string{"serve"},
			env:  map[string]string{"ASPARGES_UMAMI_SITE_ID": "11111111-2222-4333-8444-555555555555"},
			want: func(c *serveConfig) { c.umamiSiteID = "11111111-2222-4333-8444-555555555555" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			a := &app{}
			cmd := newRootCmd(a)
			require.NoError(t, cmd.Parse(tt.args, ff.WithEnvVarPrefix(envPrefix)))
			require.Equal(t, "serve", cmd.GetSelected().Name)

			want := defaults
			tt.want(&want)

			if diff := cmp.Diff(want, a.serve, cmp.AllowUnexported(serveConfig{})); diff != "" {
				t.Errorf("serveConfig mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// Search engines hear of changes only from the real site.
func TestIndexNowEndpoint(t *testing.T) {
	require.Equal(t, seo.IndexNowEndpoint, serveConfig{}.indexNowEndpoint())
	require.Empty(t, serveConfig{dev: true}.indexNowEndpoint())
	require.Empty(t, serveConfig{noindex: true}.indexNowEndpoint())
}

// The tsnet listener is the admin's origin: a public page there would run
// with the admin API's privileges.
func TestAdminFilesOnTailnet(t *testing.T) {
	h := adminFilesOnTailnet(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	tests := []struct {
		local string
		path  string
		want  int
	}{
		{"127.0.0.1:8090", "/", http.StatusTeapot},
		{"127.0.0.1:8090", "/arrangementer", http.StatusTeapot},
		{"100.64.0.1:80", "/", http.StatusNotFound},
		{"100.64.0.1:443", "/arrangementer", http.StatusNotFound},
		{"[fd7a:115c:a1e0::1]:80", "/robots.txt", http.StatusNotFound},
		{"100.64.0.1:80", "/static/admin.js", http.StatusTeapot},
		{"[fd7a:115c:a1e0::1]:80", "/img/v1/x.avif", http.StatusTeapot},
	}
	for _, tt := range tests {
		r := httptest.NewRequest(http.MethodGet, tt.path, nil)
		r = r.WithContext(context.WithValue(r.Context(), http.LocalAddrContextKey, net.TCPAddrFromAddrPort(netip.MustParseAddrPort(tt.local))))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assert.Equal(t, tt.want, w.Code, "%s %s", tt.local, tt.path)
	}
}

func TestServeConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		listen  string
		dev     bool
		wantErr error
	}{
		{name: "prod", baseURL: "https://aspargesgaarden.no", listen: "127.0.0.1:8090"},
		{name: "prod-any-addr", baseURL: "https://aspargesgaarden.no", listen: ":8090"},
		{name: "dev-ipv4", dev: true, baseURL: "http://localhost:8090", listen: "127.0.0.1:8090"},
		{name: "dev-ipv6", dev: true, baseURL: "http://localhost:8090", listen: "[::1]:8090"},
		{name: "dev-localhost", dev: true, baseURL: "http://localhost:8090", listen: "localhost:8090"},
		// Dev metrics must never reach a non-loopback interface.
		{name: "dev-all", dev: true, baseURL: "http://localhost:8090", listen: ":8090", wantErr: errDevLoopback},
		{name: "dev-public", dev: true, baseURL: "http://localhost:8090", listen: "0.0.0.0:8090", wantErr: errDevLoopback},
		{name: "dev-lan", dev: true, baseURL: "http://localhost:8090", listen: "192.168.1.2:8090", wantErr: errDevLoopback},
		{name: "dev-garbage", dev: true, baseURL: "http://localhost:8090", listen: "nope", wantErr: errDevLoopback},
		{name: "base-trailing-slash", baseURL: "https://aspargesgaarden.no/", listen: "127.0.0.1:8090", wantErr: errBaseURL},
		{name: "base-path", baseURL: "https://aspargesgaarden.no/x", listen: "127.0.0.1:8090", wantErr: errBaseURL},
		{name: "base-no-scheme", baseURL: "aspargesgaarden.no", listen: "127.0.0.1:8090", wantErr: errBaseURL},
		{name: "base-ftp", baseURL: "ftp://aspargesgaarden.no", listen: "127.0.0.1:8090", wantErr: errBaseURL},
		{name: "base-query", baseURL: "https://aspargesgaarden.no?x=1", listen: "127.0.0.1:8090", wantErr: errBaseURL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := serveConfig{baseURL: tt.baseURL, listenAddr: tt.listen, dev: tt.dev}.validate()
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// devOnlyFlags are the serve flags the NixOS module must never pass: they
// are for local work and the browser tests, not a deploy.
var devOnlyFlags = map[string]bool{
	"dev":          true, // no Tailscale, the admin on the local listener
	"e2e-fixtures": true, // hooks that delete everything
}

// Every setting the server takes is settable through services.asparges
// (owner rule): each serve flag, the root ones included, must be rendered
// into the unit by nix/module.nix, and the dev-only ones never.
func TestEveryServeFlagIsInTheModule(t *testing.T) {
	module, err := os.ReadFile(filepath.Join("..", "..", "nix", "module.nix"))
	require.NoError(t, err)

	var serve *ff.Command
	for _, c := range newRootCmd(&app{}).Subcommands {
		if c.Name == "serve" {
			serve = c
		}
	}
	require.NotNil(t, serve)

	require.NoError(t, serve.Flags.WalkFlags(func(f ff.Flag) error {
		name, _ := f.GetLongName()
		rendered := strings.Contains(string(module), `"--`+name+`=`) || strings.Contains(string(module), `"--`+name+`"`)

		if devOnlyFlags[name] {
			assert.False(t, rendered, "--%s is dev-only: the module must not pass it", name)
		} else {
			assert.True(t, rendered, "--%s has no services.asparges option rendered into the unit in nix/module.nix", name)
		}

		return nil
	}))

	// The module puts the root flags after "serve"; they must parse there.
	a := &app{}
	require.NoError(t, newRootCmd(a).Parse([]string{"serve", "--log-level=debug", "--data-dir=/var/lib/x"}))
	require.Equal(t, "debug", a.logLevel)
}

// The browser tests' hooks delete everything: never on a real deploy.
func TestServeE2EOnlyInDev(t *testing.T) {
	c := serveConfig{baseURL: "https://aspargesgaarden.no", listenAddr: "127.0.0.1:8090", e2eFixtures: "/tmp/fixtures"}
	require.ErrorIs(t, c.validate(), errDevOnly)

	c.dev = true
	require.NoError(t, c.validate())
}

// A photo taken out of the gallery survives the daily sweep for
// unusedPhotoAge from that moment, then goes, files and all.
func TestSweepUnusedPhotos(t *testing.T) {
	dir := t.TempDir()

	d, err := db.Open(t.Context(), filepath.Join(dir, "asparges.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	clock := &e2eClock{}
	store := site.NewStore(d, clock.now)

	imgs, err := images.New(dir, store.Catalog())
	require.NoError(t, err)

	srv := server.New(server.Config{BaseURL: "http://127.0.0.1:8090", Dev: true, Store: store, Images: imgs, Now: clock.now})
	log := slog.New(slog.DiscardHandler)

	f, err := os.Open(filepath.Join("..", "..", "images", "testdata", "upright.jpg"))
	require.NoError(t, err)
	defer f.Close()

	o, err := imgs.Intake(t.Context(), f)
	require.NoError(t, err)

	p, _, err := store.AddPhoto(t.Context(), site.Photo{SHA256: o.SHA256, MIME: o.MIME, Width: o.Width, Height: o.Height})
	require.NoError(t, err)
	require.NoError(t, store.SetGallery(t.Context(), []int64{p.ID}))
	srv.Changed(p.SHA256) // as the upload does

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { done <- imgs.Run(ctx) }()

	// Long in the gallery, then taken out: the grace runs from now.
	clock.advance(30 * 24 * time.Hour)
	require.NoError(t, store.SetGallery(t.Context(), nil))

	clock.advance(unusedPhotoAge - time.Hour)
	sweepOnce(t.Context(), log, store, srv)

	_, err = store.Photo(t.Context(), p.ID)
	require.NoError(t, err, "inside the grace period")

	clock.advance(2 * time.Hour)
	sweepOnce(t.Context(), log, store, srv)

	_, err = store.Photo(t.Context(), p.ID)
	require.ErrorIs(t, err, site.ErrNotFound)

	orig := filepath.Join(dir, "originals", o.SHA256[:2], o.SHA256+".jpg")
	require.Eventually(t, func() bool {
		_, err := os.Stat(orig)
		return errors.Is(err, fs.ErrNotExist)
	}, time.Minute, 20*time.Millisecond, "the photo store removes its files")
	require.NoDirExists(t, filepath.Join(dir, "variants", "v1", o.SHA256[:2], o.SHA256))

	cancel()
	require.NoError(t, <-done)
}
