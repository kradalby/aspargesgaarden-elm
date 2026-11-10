package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/tailcfg/peercap"

	"github.com/kradalby/asparges/images"
	"github.com/kradalby/asparges/pages"
	"github.com/kradalby/asparges/ui"
)

// CapName is the Tailscale app capability that lets a tailnet user into the
// admin. Reaching the node is not enough: Prometheus reaches it for
// /metrics. A grant in the tailnet policy gives it, see Cap.
const CapName peercap.Cap = "kradalby.no/cap/asparges"

// Cap is the capability's value. Only {"admin": true} lets a caller in; the
// policy file grants it as
//
//	"app": {"kradalby.no/cap/asparges": [{"admin": true}]}
type Cap struct {
	Admin bool `json:"admin"`
}

// DevName is who every caller is in --dev, where there is no tailnet.
const DevName = "Utvikler"

// Tailnet is the local Tailscale node as the admin asks it about callers and
// itself; *local.Client, or a fake in tests.
type Tailnet interface {
	WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)
	StatusWithoutPeers(ctx context.Context) (*ipnstate.Status, error)
}

// Photos is the photo store (images.Store) as the admin uses it.
type Photos interface {
	Intake(ctx context.Context, r io.Reader) (images.Original, error)
	Status(ctx context.Context, sha256 string) (images.Status, error)
	Retry(ctx context.Context, sha256 string) error
}

// AdminConfig is what the admin needs beyond the public site.
type AdminConfig struct {
	// Tailnet identifies callers on the tsnet listener. Required outside
	// --dev; in --dev the loopback listener stands in and every caller is
	// DevName.
	Tailnet Tailnet
	Photos  Photos
}

// user is who made an admin request.
type user struct {
	// Name is shown in the app; Login and Node go in the log.
	Name, Login, Node string
}

type userKey struct{}

func userOf(ctx context.Context) user {
	u, _ := ctx.Value(userKey{}).(user)
	return u
}

// Admin returns the admin: the JSON API under /admin/api/ and the app under
// /admin/. Mount it on the tsnet listener only; in --dev, Public carries it.
//
// Every request must come from a caller holding CapName, and name this node
// as its Host, so a page on another site cannot reach it through a
// rebound DNS name. Writes must also come from the admin's own origin
// (Sec-Fetch-Site and Origin), so a page elsewhere cannot make a tailnet
// browser send them; with no cookies there is nothing else to forge.
func (s *Server) Admin() http.Handler {
	mux := http.NewServeMux()
	s.adminAPI(mux)
	mux.HandleFunc("/admin/api/", func(w http.ResponseWriter, r *http.Request) {
		s.writeProblems(w, r, http.StatusNotFound, []problem{{Code: codeNotFound}}, nil)
	})
	// Every other address under /admin/ is the document the app runs in; its
	// files come from /static/, which the listener serves beside it. No
	// method in the pattern: "GET /admin/" would conflict with the
	// method-less "/admin/api/".
	mux.Handle("/admin/", pages.AdminHandler())

	csrf := http.NewCrossOriginProtection()
	csrf.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.deny(w, r, codeCrossOrigin, "cross-origin write refused")
	}))
	h := csrf.Handler(mux)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := w.Header()
		hdr.Set("Content-Security-Policy", ui.AdminCSP())
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("Referrer-Policy", "same-origin")
		hdr.Set("Cache-Control", "no-store")
		// As every other response; the admin is not counted as a visit.
		if s.cfg.NoIndex {
			hdr.Set("X-Robots-Tag", "noindex")
		}

		u, code, reason := s.identify(r)
		if code != "" {
			s.deny(w, r, code, reason)
			return
		}

		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, u)))
	})
}

// identify says who is calling, or why nobody may: an empty code lets the
// request in.
func (s *Server) identify(r *http.Request) (u user, code, reason string) {
	ctx := r.Context()
	host := requestHost(r)

	if s.cfg.Dev {
		if !isLoopbackHost(host) {
			return user{}, codeWrongHost, "host " + host + " is not loopback"
		}

		return user{Name: DevName, Login: "dev"}, "", ""
	}

	tn := s.cfg.Admin.Tailnet
	if tn == nil {
		return user{}, codeForbidden, "no tailnet to identify callers"
	}

	st, err := tn.StatusWithoutPeers(ctx)
	if err != nil || st == nil || st.Self == nil {
		s.cfg.Log.ErrorContext(ctx, "admin: tailnet status", "err", err)
		return user{}, codeForbidden, "own tailnet names unknown"
	}

	if !ownHost(host, st) {
		return user{}, codeWrongHost, "host " + host + " is not this node"
	}

	who, err := tn.WhoIs(ctx, r.RemoteAddr)
	// The local API answers JSON null as (nil, nil): nobody, not a crash.
	if err != nil || who == nil || who.UserProfile == nil {
		return user{}, codeForbidden, "caller not identified"
	}

	u = user{Name: who.UserProfile.DisplayName, Login: who.UserProfile.LoginName}
	if u.Name == "" {
		u.Name = u.Login
	}

	if who.Node != nil {
		u.Node = who.Node.ComputedName
	}

	if !hasAdminCap(who.CapMap) {
		return u, codeForbidden, "no " + string(CapName) + " grant"
	}

	return u, "", ""
}

// hasAdminCap reports whether the caller's grants hold {"admin": true}.
// A missing capability or a malformed value is no.
func hasAdminCap(cm tailcfg.PeerCapMap) bool {
	caps, err := tailcfg.UnmarshalCapJSON[Cap](cm, CapName)
	if err != nil {
		return false
	}

	return slices.ContainsFunc(caps, func(c Cap) bool { return c.Admin })
}

func requestHost(r *http.Request) string {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else {
		// An IPv6 address without a port keeps its brackets in Host.
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}

	return strings.ToLower(strings.TrimSuffix(host, "."))
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}

	ip, err := netip.ParseAddr(host)

	return err == nil && ip.IsLoopback()
}

// ownHost reports whether host names this node: its MagicDNS name, the
// short name before the first dot, or one of its tailnet addresses.
func ownHost(host string, st *ipnstate.Status) bool {
	if ip, err := netip.ParseAddr(host); err == nil {
		return slices.Contains(st.TailscaleIPs, ip)
	}

	fqdn := strings.ToLower(strings.TrimSuffix(st.Self.DNSName, "."))
	short, _, _ := strings.Cut(fqdn, ".")

	return fqdn != "" && (host == fqdn || host == short)
}

func (s *Server) deny(w http.ResponseWriter, r *http.Request, code, reason string) {
	u := userOf(r.Context())
	s.cfg.Log.WarnContext(r.Context(), "admin: refused", "code", code, "reason", reason,
		"user", u.Login, "remote", r.RemoteAddr, "method", r.Method, "path", logPath(r.URL.Path))

	if strings.HasPrefix(r.URL.Path, "/admin/api/") {
		s.writeProblems(w, r, http.StatusForbidden, []problem{{Code: code}}, nil)
		return
	}

	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
}

// audit logs who changed what, for every admin write that committed.
func (s *Server) audit(r *http.Request, action string, attrs ...any) {
	u := userOf(r.Context())
	s.cfg.Log.InfoContext(r.Context(), "admin: "+action,
		append([]any{slog.String("user", u.Login), slog.String("node", u.Node)}, attrs...)...)
}

// maxLogPath bounds a request path in the log: the caller chooses it.
const maxLogPath = 128

func logPath(p string) string {
	if len(p) > maxLogPath {
		return p[:maxLogPath] + "…"
	}

	return p
}
