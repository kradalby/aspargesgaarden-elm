package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/pages"
	"github.com/kradalby/asparges/ui"
)

// The callers the admin tests make requests as.
const (
	kristine  = "100.64.0.1:41000" // admin user with the grant
	anders    = "100.64.0.2:41000" // tailnet user without it
	monitor   = "100.64.0.3:41000" // tagged monitoring node
	stranger  = "100.64.0.9:41000" // WhoIs knows nobody here
	adminHost = "asparges.tail1234.ts.net"
)

// fakeTailnet answers WhoIs and the node's own names as tsnet would. A nil
// entry in who is the local API's JSON null; statusErr and noSelf are a
// node that cannot say who it is.
type fakeTailnet struct {
	who       map[string]*apitype.WhoIsResponse
	statusErr error
	noSelf    bool
}

func (f *fakeTailnet) WhoIs(_ context.Context, addr string) (*apitype.WhoIsResponse, error) {
	w, ok := f.who[addr]
	if !ok {
		return nil, errors.New("no peer with that address")
	}

	return w, nil
}

func (f *fakeTailnet) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) {
	if f.statusErr != nil {
		return nil, f.statusErr
	}

	if f.noSelf {
		return &ipnstate.Status{}, nil
	}

	return &ipnstate.Status{
		Self:         &ipnstate.PeerStatus{DNSName: adminHost + "."},
		TailscaleIPs: []netip.Addr{netip.MustParseAddr("100.64.0.100"), netip.MustParseAddr("fd7a:115c:a1e0::100")},
	}, nil
}

func peer(login, display string, caps ...string) *apitype.WhoIsResponse {
	w := &apitype.WhoIsResponse{
		Node:        &tailcfg.Node{ComputedName: strings.Split(login, "@")[0] + "-laptop"},
		UserProfile: &tailcfg.UserProfile{LoginName: login, DisplayName: display},
		CapMap:      tailcfg.PeerCapMap{},
	}
	for _, c := range caps {
		w.CapMap[CapName] = append(w.CapMap[CapName], tailcfg.RawMessage(c))
	}

	return w
}

func newFakeTailnet() *fakeTailnet {
	return &fakeTailnet{who: map[string]*apitype.WhoIsResponse{
		kristine: peer("kristine@example.com", "Kristine", `{"admin":true}`),
		anders:   peer("anders@example.com", "Anders"),
		monitor:  peer("tagged-devices", ""),
	}}
}

// fakePhotos stands in for images.Store where the tests need no vips.
type fakePhotos struct {
	mu      sync.Mutex
	status  map[string]images.Status
	retried []string
}

func (f *fakePhotos) Intake(_ context.Context, r io.Reader) (images.Original, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return images.Original{}, err
	}

	sum := sha256.Sum256(b)

	return images.Original{SHA256: hex.EncodeToString(sum[:]), MIME: "image/jpeg", Width: 1600, Height: 1200}, nil
}

func (f *fakePhotos) Status(_ context.Context, sha string) (images.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if st, ok := f.status[sha]; ok {
		return st, nil
	}

	return images.Status{State: images.StateReady, Ready: 40, Total: 40, Preview: "/img/x"}, nil
}

func (f *fakePhotos) Retry(_ context.Context, sha string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.retried = append(f.retried, sha)
	delete(f.status, sha)

	return nil
}

type adminFixture struct {
	*fixture

	admin  http.Handler
	photos *fakePhotos
	logs   *bytes.Buffer
}

// newAdmin is the admin over a temp database, as the tsnet listener has it,
// or in --dev as the local listener does. Its photo store is a fake; see
// withVips for the real one.
func newAdmin(t *testing.T, dev bool) *adminFixture {
	t.Helper()

	f := newFixture(t, dev)
	fp := &fakePhotos{status: map[string]images.Status{}}

	logs := &bytes.Buffer{}
	f.srv.cfg.Log = slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	f.srv.cfg.Admin = AdminConfig{Tailnet: newFakeTailnet(), Photos: fp}

	if dev {
		f.srv.cfg.Admin.Tailnet = nil
		f.h = f.srv.Public()

		return &adminFixture{fixture: f, admin: f.h, photos: fp, logs: logs}
	}

	return &adminFixture{fixture: f, admin: f.srv.Admin(), photos: fp, logs: logs}
}

type req struct {
	from    string // RemoteAddr; kristine when empty
	host    string // adminHost when empty
	body    any    // JSON-encoded unless a string or []byte
	ctype   string // application/json when a body is given
	headers map[string]string
}

func (a *adminFixture) do(t *testing.T, method, path string, r req) *httptest.ResponseRecorder {
	t.Helper()

	var body io.Reader

	switch b := r.body.(type) {
	case nil:
	case string:
		body = strings.NewReader(b)
	case []byte:
		body = bytes.NewReader(b)
	default:
		j, err := json.Marshal(b)
		require.NoError(t, err)

		body = bytes.NewReader(j)
	}

	rq := httptest.NewRequestWithContext(t.Context(), method, path, body)
	rq.RemoteAddr = cmpOr(r.from, kristine)
	rq.Host = cmpOr(r.host, adminHost)

	if body != nil {
		rq.Header.Set("Content-Type", cmpOr(r.ctype, "application/json"))
	}

	for k, v := range r.headers {
		rq.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	a.admin.ServeHTTP(rec, rq)

	return rec
}

func cmpOr(s, def string) string {
	if s == "" {
		return def
	}

	return s
}

// errorsOf decodes an error body.
func errorsOf(t *testing.T, rec *httptest.ResponseRecorder) []problem {
	t.Helper()

	var b struct {
		Errors []problem `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b), rec.Body.String())

	return b.Errors
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v), rec.Body.String())

	return v
}

// routes is every admin address, with a body each would accept.
var routes = []struct{ method, path string }{
	{"GET", "/admin/"},
	{"GET", "/admin/arrangementer/4"},
	{"GET", "/admin/api/me"},
	{"GET", "/admin/api/events"},
	{"POST", "/admin/api/events"},
	{"GET", "/admin/api/events/1"},
	{"PUT", "/admin/api/events/1"},
	{"POST", "/admin/api/events/1/publish"},
	{"POST", "/admin/api/events/1/unpublish"},
	{"POST", "/admin/api/events/1/copy"},
	{"DELETE", "/admin/api/events/1"},
	{"POST", "/admin/api/preview"},
	{"POST", "/admin/api/photos"},
	{"GET", "/admin/api/photos/status?ids=1"},
	{"GET", "/admin/api/photos/1"},
	{"PUT", "/admin/api/photos/1"},
	{"PUT", "/admin/api/photos/1/crops"},
	{"POST", "/admin/api/photos/1/prepare"},
	{"DELETE", "/admin/api/photos/1"},
	{"GET", "/admin/api/photographers"},
	{"POST", "/admin/api/photographers"},
	{"PUT", "/admin/api/photographers/1"},
	{"GET", "/admin/api/gallery"},
	{"PUT", "/admin/api/gallery"},
}

// Reaching the node is not enough: only the grant lets anyone in, on every
// route, and the refusal says nothing about what is behind it.
func TestAdminAccess(t *testing.T) {
	callers := []struct {
		name    string
		from    string
		host    string
		allowed bool
	}{
		{"admin user", kristine, "", true},
		{"admin user by short name", kristine, "asparges", true},
		{"admin user by address", kristine, "100.64.0.100:80", true},
		// What a browser sends for http://[fd7a:115c:a1e0::100]/admin/.
		{"admin user by IPv6 address", kristine, "[fd7a:115c:a1e0::100]", true},
		{"admin user by IPv6 address and port", kristine, "[fd7a:115c:a1e0::100]:80", true},
		{"tailnet user without the grant", anders, "", false},
		{"monitoring node", monitor, "", false},
		{"no identity", stranger, "", false},
		// A page elsewhere whose DNS name was rebound to this node's address.
		{"admin user, foreign host", kristine, "asparges.evil.example", false},
	}

	for _, c := range callers {
		for _, rt := range routes {
			t.Run(c.name+" "+rt.method+" "+rt.path, func(t *testing.T) {
				a := newAdmin(t, false)
				rec := a.do(t, rt.method, rt.path, req{from: c.from, host: c.host, body: "{}"})

				if c.allowed {
					require.NotEqual(t, http.StatusForbidden, rec.Code, rec.Body.String())
					return
				}

				require.Equal(t, http.StatusForbidden, rec.Code)
				require.NotContains(t, rec.Body.String(), "Kristine")
				if strings.HasPrefix(rt.path, "/admin/api/") {
					require.Len(t, errorsOf(t, rec), 1)
				}
			})
		}
	}
}

// Whatever the tailnet cannot tell, the answer is no, never a crash or a way
// in: the node's own names unknown, a caller WhoIs knows nothing of, or no
// tailnet at all outside --dev.
func TestAdminAccessWhenTheTailnetCannotTell(t *testing.T) {
	tests := []struct {
		name    string
		breakIt func(*adminFixture, *fakeTailnet)
	}{
		{"status fails", func(_ *adminFixture, tn *fakeTailnet) { tn.statusErr = errors.New("tailscaled is not running") }},
		{"no self", func(_ *adminFixture, tn *fakeTailnet) { tn.noSelf = true }},
		{"WhoIs answers null", func(_ *adminFixture, tn *fakeTailnet) { tn.who[kristine] = nil }},
		{"no user profile", func(_ *adminFixture, tn *fakeTailnet) { tn.who[kristine].UserProfile = nil }},
		{"no tailnet", func(a *adminFixture, _ *fakeTailnet) { a.srv.cfg.Admin.Tailnet = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAdmin(t, false)
			tn := newFakeTailnet()
			a.srv.cfg.Admin.Tailnet = tn
			tt.breakIt(a, tn)

			rec := a.do(t, "GET", "/admin/api/me", req{})
			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Equal(t, []problem{{Code: "forbidden"}}, errorsOf(t, rec))
		})
	}
}

func TestAdminCapabilityShape(t *testing.T) {
	tests := []struct {
		name string
		caps []string
		want bool
	}{
		{"admin", []string{`{"admin":true}`}, true},
		{"one of several", []string{`{"admin":false}`, `{"admin":true}`}, true},
		{"false", []string{`{"admin":false}`}, false},
		{"empty", []string{`{}`}, false},
		{"malformed", []string{`"admin"`}, false},
		{"none", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, hasAdminCap(peer("x@example.com", "X", tt.caps...).CapMap))
		})
	}
}

func TestAdminMe(t *testing.T) {
	a := newAdmin(t, false)
	got := decode[map[string]string](t, a.do(t, "GET", "/admin/api/me", req{}))
	require.Equal(t, map[string]string{"name": "Kristine", "siteUrl": "https://aspargesgaarden.no"}, got)

	noDisplay := newAdmin(t, false)
	noDisplay.srv.cfg.Admin.Tailnet.(*fakeTailnet).who[kristine].UserProfile.DisplayName = ""
	got = decode[map[string]string](t, noDisplay.do(t, "GET", "/admin/api/me", req{}))
	require.Equal(t, "kristine@example.com", got["name"])
}

// --dev has no tailnet: the loopback listener carries the admin, and every
// caller is the developer, as long as the browser came by a loopback name.
func TestAdminDev(t *testing.T) {
	a := newAdmin(t, true)

	got := decode[map[string]string](t, a.do(t, "GET", "/admin/api/me", req{from: "127.0.0.1:5000", host: "127.0.0.1:8090"}))
	require.Equal(t, DevName, got["name"])

	rec := a.do(t, "GET", "/admin/api/me", req{from: "127.0.0.1:5000", host: "localhost:8090"})
	require.Equal(t, http.StatusOK, rec.Code)

	rec = a.do(t, "GET", "/admin/api/me", req{from: "127.0.0.1:5000", host: "rebound.example:8090"})
	require.Equal(t, http.StatusForbidden, rec.Code)

	// The public pages are still there beside it.
	require.Equal(t, http.StatusOK, a.get(t, "/").Code)
}

// The admin is cookie-less, but a page elsewhere can still make a browser on
// the tailnet send a request: writes must come from the admin's own origin.
func TestAdminCrossOriginWrites(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		ctype   string
		want    int
		code    string
	}{
		{"same origin", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "https://" + adminHost}, "", http.StatusCreated, ""},
		{"no browser headers (curl)", nil, "", http.StatusCreated, ""},
		{"cross site", map[string]string{"Sec-Fetch-Site": "cross-site"}, "", http.StatusForbidden, "cross_origin"},
		{"same site, other host", map[string]string{"Sec-Fetch-Site": "same-site"}, "", http.StatusForbidden, "cross_origin"},
		{"foreign origin, old browser", map[string]string{"Origin": "https://evil.example"}, "", http.StatusForbidden, "cross_origin"},
		{"form post", nil, "application/x-www-form-urlencoded", http.StatusBadRequest, "bad_request"},
		{"text post", nil, "text/plain", http.StatusBadRequest, "bad_request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newAdmin(t, false)
			rec := a.do(t, "POST", "/admin/api/photographers", req{
				body: map[string]string{"name": "Ola"}, ctype: tt.ctype, headers: tt.headers,
			})
			require.Equal(t, tt.want, rec.Code, rec.Body.String())

			if tt.code != "" {
				require.Equal(t, []problem{{Code: tt.code}}, errorsOf(t, rec))
			}
		})
	}

	// Reads stay possible: another site cannot see the answer anyway.
	a := newAdmin(t, false)
	rec := a.do(t, "GET", "/admin/api/me", req{headers: map[string]string{"Sec-Fetch-Site": "cross-site"}})
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminHeaders(t *testing.T) {
	a := newAdmin(t, false)

	// The app's document revalidates instead, so a deploy reaches an open tab.
	for path, cache := range map[string]string{"/admin/": "no-cache", "/admin/api/me": "no-store", "/admin/api/nope": "no-store"} {
		rec := a.do(t, "GET", path, req{})
		h := rec.Header()
		require.Equal(t, ui.AdminCSP(), h.Get("Content-Security-Policy"), path)
		require.Equal(t, cache, h.Get("Cache-Control"), path)
		require.Equal(t, "nosniff", h.Get("X-Content-Type-Options"), path)
	}

	rec := a.do(t, "GET", "/admin/api/nope", req{})
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, []problem{{Code: "not_found"}}, errorsOf(t, rec))
}

// The admin says noindex as every response does on the beta site, and is no
// visit and no crawler request in the site's metrics.
func TestAdminNoIndexAndUncounted(t *testing.T) {
	a := newAdmin(t, true)
	a.srv.cfg.NoIndex = true
	a.h = a.srv.Public()
	a.admin = a.h

	counted := func() float64 {
		mfs, err := prometheus.DefaultGatherer.Gather()
		require.NoError(t, err)

		var n float64
		for _, mf := range mfs {
			if mf.GetName() == "asparges_bot_requests_total" || mf.GetName() == "asparges_referrals_total" {
				for _, m := range mf.GetMetric() {
					n += m.GetCounter().GetValue()
				}
			}
		}

		return n
	}

	before := counted()

	for _, path := range []string{"/admin/", "/admin/api/me"} {
		rec := a.do(t, "GET", path+"?utm_source=chatgpt.com", req{
			from: "127.0.0.1:5000", host: "127.0.0.1:8090",
			headers: map[string]string{"User-Agent": "Mozilla/5.0 (compatible; GPTBot/1.1; +https://openai.com/gptbot)"},
		})
		require.Equal(t, http.StatusOK, rec.Code, path)
		require.Equal(t, "noindex", rec.Header().Get("X-Robots-Tag"), path)
	}

	require.InDelta(t, before, counted(), 0)
}

// Every address under /admin/ but the API is the app's document.
func TestAdminApp(t *testing.T) {
	a := newAdmin(t, false)
	want := pages.Admin().Render()

	for _, path := range []string{"/admin/", "/admin/arrangementer/4", "/admin/galleri"} {
		rec := a.do(t, "GET", path, req{})
		require.Equal(t, http.StatusOK, rec.Code, path)
		require.Equal(t, want, rec.Body.String(), path)
	}

	require.Equal(t, http.StatusMethodNotAllowed, a.do(t, "POST", "/admin/", req{}).Code)

	// The API stays the API, and the app is as closed as the rest.
	require.Equal(t, http.StatusOK, a.do(t, "GET", "/admin/api/me", req{}).Code)
	require.Equal(t, http.StatusForbidden, a.do(t, "GET", "/admin/", req{from: anders}).Code)
}

// Every write says who did what.
func TestAdminAudit(t *testing.T) {
	a := newAdmin(t, false)
	rec := a.do(t, "POST", "/admin/api/photographers", req{body: map[string]string{"name": "Ola"}})
	require.Equal(t, http.StatusCreated, rec.Code)

	require.Contains(t, a.logs.String(), `"msg":"admin: photographer saved","user":"kristine@example.com","node":"kristine-laptop"`)

	rec = a.do(t, "GET", "/admin/api/me", req{from: anders})
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, a.logs.String(), `"msg":"admin: refused","code":"forbidden"`)
}
