# The admin's grant end to end on a real tailnet: headscale as control, the
# service on its own tsnet node, and three peers. Only the peer whose user
# holds kradalby.no/cap/asparges gets in; the monitoring peer reaches
# /metrics and nothing else.
{
  pkgs,
  module,
  headscaleTestkit,
  headscaleTestkitPeer,
}:
let
  peer = {
    imports = [ headscaleTestkitPeer ];
    environment.systemPackages = [
      pkgs.curl
      pkgs.jq
    ];
  };
in
{
  name = "asparges-tsnet";

  nodes = {
    headscale.imports = [ headscaleTestkit ];

    server =
      { lib, ... }:
      {
        imports = [ module ];
        services.asparges = {
          enable = true;
          baseURL = "https://beta.aspargesgaarden.no";
          # Root-only and written by the test: the unit reads it only as a
          # credential.
          tailscaleKeyPath = "/root/asparges-tskey";
        };
        systemd.services.asparges = {
          # Started by the test once the key exists.
          wantedBy = lib.mkForce [ ];
          environment = {
            TS_CONTROL_URL = "http://headscale";
            TS_NO_LOGS_NO_SUPPORT = "1";
          };
        };
        environment.systemPackages = [ pkgs.curl ];
      };

    # kristine is in group:asparges-admin; anders is on the tailnet without
    # the grant; prometheus scrapes metrics.
    admin = peer;
    guest = peer;
    monitor = peer;
  };

  testScript = ''
    import json, re, shlex

    photo = "${../../images/testdata/upright.jpg}"

    def call(m, method, url, *args):
        """One request from a peer: status code and body."""
        code = m.succeed(
            f"curl -sS -o /tmp/body -w '%{{http_code}}' --globoff --max-time 60 -X {method} "
            + " ".join(shlex.quote(a) for a in args) + " " + shlex.quote(url)
        )
        return int(code), m.succeed("cat /tmp/body")

    def api(m, method, url, body=None, *args):
        if body is not None:
            args = ("-H", "Content-Type: application/json", "--data", json.dumps(body)) + args
        return call(m, method, url, *args)

    def public(path):
        code = server.succeed(
            f"curl -sS -o /tmp/body -w '%{{http_code}}' http://127.0.0.1:8090{path}")
        return int(code), server.succeed("cat /tmp/body")

    start_all()

    keys = {u: headscale.succeed(f"hs-authkey {u}").strip()
            for u in ["asparges", "kristine", "anders", "prometheus"]}

    headscale.succeed("cat > /tmp/policy.json <<'EOF'\n" + json.dumps({
        "groups": {"group:asparges-admin": ["kristine@"]},
        "acls": [{"action": "accept", "src": ["*"], "dst": ["*:*"]}],
        "grants": [{
            "src": ["group:asparges-admin"],
            "dst": ["asparges@"],
            "app": {"kradalby.no/cap/asparges": [{"admin": True}]},
        }],
    }) + "\nEOF")
    headscale.succeed("headscale policy set -f /tmp/policy.json")

    with subtest("the service joins with its key as a credential"):
        server.succeed(f"install -m 0400 /dev/null /root/asparges-tskey && echo -n {keys['asparges']} > /root/asparges-tskey")
        # Joining below proves the key arrived through LoadCredential.
        server.fail("runuser -u asparges -- cat /root/asparges-tskey")
        server.systemctl("start asparges.service")
        server.wait_for_unit("asparges.service")
        server.wait_for_open_port(8090)
        for m, user in [(admin, "kristine"), (guest, "anders"), (monitor, "prometheus")]:
            m.succeed(f"hs-join {keys[user]}")
        ip4 = admin.wait_until_succeeds("tailscale ip -4 asparges", timeout=120).strip()
        ip6 = admin.succeed("tailscale ip -6 asparges").strip()
        for m in (guest, monitor):
            m.wait_until_succeeds("tailscale ip -4 asparges", timeout=120)
        # tsnet keeps its identity in the state directory, owned by the unit.
        server.succeed("test -s /var/lib/asparges/tsnet/tailscaled.state")
        server.succeed("test $(stat -c %U /var/lib/asparges/tsnet) = asparges")

    with subtest("the names a tailnet peer reaches the node by"):
        me = json.loads(admin.succeed("tailscale status --json"))
        node = next(p for p in me["Peer"].values() if p["HostName"] == "asparges")
        fqdn = node["DNSName"].rstrip(".")
        print(f"asparges on the tailnet: {fqdn} {node['TailscaleIPs']}")
        assert fqdn == "asparges.tailnet", fqdn
        # MagicDNS answers the full name, and the short one through the
        # search domain tailscaled sets.
        for name in [fqdn, "asparges"]:
            assert admin.succeed(f"getent hosts {name}").split()[0] == ip4, name

    base = f"http://{fqdn}"

    with subtest("a peer with the grant uses the admin"):
        for host in [fqdn, "asparges", ip4, f"[{ip6}]"]:
            code, body = call(admin, "GET", f"http://{host}/admin/api/me")
            assert code == 200, (host, code, body)
        code, body = call(admin, "GET", f"{base}/admin/")
        assert code == 200, (code, body)
        # The app's shell, whose script comes from /static/ on the same node.
        scripts = re.findall(r'src="(/static/admin\.js[^"]*)"', body)
        assert scripts, body
        assert call(admin, "GET", base + scripts[0])[0] == 200, scripts
        code, body = call(admin, "GET", f"{base}/admin/api/me")
        me = json.loads(body)
        assert "kristine" in me["name"], me

        code, body = call(admin, "POST", f"{base}/admin/api/photos", "-F", f"file=@{photo};type=image/jpeg")
        assert code == 201, (code, body)
        p = json.loads(body)

        code, body = api(admin, "POST", f"{base}/admin/api/events", {
            "title": "Konsert i låven",
            "summary": "Konsert med lokale musikere.",
            "body": "Velkommen til **konsert**.",
            "start": "2030-06-01",
            "startTime": "19:00",
            "mainPhoto": {"id": p["id"], "alt": "Låven pyntet til konsert"},
        })
        assert code == 201, (code, body)
        e = json.loads(body)
        assert e["status"] == "draft", e
        assert public(e["path"])[0] == 404

        code, body = api(admin, "POST", f"{base}/admin/api/events/{e['id']}/publish",
                         {"updatedAt": e["updatedAt"]})
        assert code == 200, (code, body)
        e = json.loads(body)
        assert e["status"] == "published", e

    with subtest("the public listener shows what the admin published"):
        code, html = public(e["path"])
        assert code == 200, code
        assert "Konsert i låven" in html and "Låven pyntet til konsert" in html
        assert e["path"] in public("/sitemap.xml")[1]
        assert "Konsert i låven" in public("/arrangementer")[1]
        # The uploaded photo, a 404 until the unit's queue has built it.
        url = re.findall(r"/img/[^\" ,]+\.avif", html)[0]
        server.wait_until_succeeds(f"curl -fsS -o /dev/null http://127.0.0.1:8090{url}", timeout=300)
        for path in ["/admin/", "/admin/api/me"]:
            assert public(path)[0] == 404, path
        # The tailnet listener is the admin's origin: its files, no pages.
        assert call(admin, "GET", base + url)[0] == 200, url
        for path in ["/", e["path"], "/robots.txt"]:
            assert call(admin, "GET", base + path)[0] == 404, path

    with subtest("the admin refuses other hosts and cross-origin writes"):
        # A page elsewhere whose name was rebound to the node's address.
        code, body = call(admin, "GET", f"{base}/admin/api/me", "-H", "Host: asparges.evil.example")
        assert code == 403 and "wrong_host" in body, (code, body)
        code, _ = call(admin, "GET", f"{base}/admin/", "-H", "Host: evil.example")
        assert code == 403, code
        for header in ["Origin: http://evil.example", "Sec-Fetch-Site: cross-site"]:
            code, body = api(admin, "POST", f"{base}/admin/api/events", {"title": "Falsk"}, "-H", header)
            assert code == 403 and "cross_origin" in body, (header, code, body)
        # The admin's own origin may write.
        code, body = api(admin, "POST", f"{base}/admin/api/events", {"title": "Ekte"},
                         "-H", f"Origin: http://{fqdn}", "-H", f"Host: {fqdn}")
        assert code == 201, (code, body)

    with subtest("a tailnet peer without the grant is refused"):
        for method, path, body in [("GET", "/admin/", None), ("GET", "/admin/api/me", None),
                                   ("GET", "/admin/api/events", None),
                                   ("POST", "/admin/api/events", {"title": "Nei"}),
                                   ("POST", f"/admin/api/events/{e['id']}/unpublish", {"updatedAt": e["updatedAt"]})]:
            code, out = api(guest, method, base + path, body)
            assert code == 403, (method, path, code, out)
            assert "Konsert" not in out, out
        assert public(e["path"])[0] == 200

    with subtest("monitoring reaches /metrics and not the admin"):
        code, body = call(monitor, "GET", f"{base}/metrics")
        assert code == 200 and "asparges_" in body, code
        for path in ["/admin/", "/admin/api/me"]:
            assert call(monitor, "GET", base + path)[0] == 403, path

    with subtest("a restart keeps the node's identity"):
        server.systemctl("restart asparges.service")
        server.wait_for_open_port(8090)
        admin.wait_until_succeeds(f"curl -fsS --max-time 10 {base}/admin/api/me", timeout=120)
        nodes = json.loads(headscale.succeed("headscale nodes list -o json"))
        assert sum(n.get("given_name") == "asparges" or n.get("name") == "asparges" for n in nodes) == 1, nodes

    with subtest("HTTPS needs a control server that issues certificates"):
        # headscale issues none, so here the admin is plain HTTP on :80; the
        # handshake on :443 fails rather than serving anything.
        admin.fail(f"curl -sS -o /dev/null --max-time 20 https://{fqdn}/admin/")
  '';
}
