package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/peterbourgon/ff/v4"
	"github.com/stretchr/testify/require"

	"github.com/kradalby/asparges/seo"
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

// The browser tests' hooks delete everything: never on a real deploy.
func TestServeE2EOnlyInDev(t *testing.T) {
	c := serveConfig{baseURL: "https://aspargesgaarden.no", listenAddr: "127.0.0.1:8090", e2eFixtures: "/tmp/fixtures"}
	require.ErrorIs(t, c.validate(), errDevOnly)

	c.dev = true
	require.NoError(t, c.validate())
}
