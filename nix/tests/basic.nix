# The service as a deployer runs it, through the module and its hardening:
# public pages, old addresses, the import, photos built by vips inside the
# unit, noindex, a restore from database and originals alone, and a clean
# stop. Tailscale is the tsnet test's business.
{
  pkgs,
  vips,
  module,
}:
let
  testdata = ../../images/testdata;

  # A checkout of the old site in miniature, shaped as `asparges import`
  # reads it: event files with the old frontmatter quirks, gallery.yaml and
  # the photos under public/. The photos are the images tests' licence-free
  # fixtures; one is scaled up to a camera-sized photo, so the unit's memory
  # cap meets a real AVIF encode.
  oldSite = pkgs.runCommand "asparges-old-site" { nativeBuildInputs = [ (pkgs.lib.getBin vips) ]; } ''
    mkdir -p $out/content/events $out/public/arrangementer $out/public/galleri
    cp ${testdata}/upright.jpg $out/public/arrangementer/julemarked.jpeg
    cp ${testdata}/p3.jpg $out/public/arrangementer/vinsmaking.jpeg
    cp ${testdata}/gps.jpg $out/public/galleri/tunet.jpeg
    cp ${testdata}/orient6.jpg $out/public/galleri/laven.jpeg
    vips resize ${testdata}/upright.jpg "$out/public/galleri/stor.jpeg[Q=90]" 5

    cat > $out/content/events/2030-11-16-julemarked.md <<'EOF'
    ---
    {
      "title": "Julemarked",
      "eventDate": "2030-11-16",
      "eventTime": "12:00",
      "eventEndTime": "16:00",
      "publishedFrom": "2030-10-01",
      "photo": { "path": "arrangementer/julemarked", "description": "Julemarked på tunet", },
    }
    ---

    Kom og handle julegaver på **Aspargesgården**.
    EOF

    cat > $out/content/events/2030-03-14-vinsmaking.md <<'EOF'
    ---
    {
      "title": "Vinsmaking på Aspargesgården",
      "eventDate": "2030-03-14",
      "eventTime": "ca. 19:00",
      "publishedFrom": "2030-02-01",
      "photo":
        {
          "path": "arrangementer/vinsmaking",
          "photographer": { "name": "Testfotograf", "website": "https://example.com/" },
          "description": "",
        },
    }
    ---

    Smak på vin i låven.
    EOF

    cat > $out/content/gallery.yaml <<'EOF'
    photographers:
      test:
        name: Testfotograf
        website: https://example.com/
    photos:
      - path: galleri/tunet
        photographer: test
        description: Tunet om sommeren
      - path: galleri/laven
        description: Låven
      - path: galleri/stor
        photographer: test
        description: Stort bilde
    EOF
  '';

  node = noindex: {
    imports = [ module ];
    services.asparges = {
      enable = true;
      baseURL = "https://beta.aspargesgaarden.no";
      inherit noindex;
    };
    # No control server here: tsnet just keeps retrying in the background.
    systemd.services.asparges.environment.TS_NO_LOGS_NO_SUPPORT = "1";
    # Nothing for vips: the module's CLI has to find it by itself.
    environment.systemPackages = [ pkgs.curl ];
  };
in
{
  name = "asparges-basic";

  nodes = {
    site = {
      imports = [ (node false) ];
      # More than the unit's MemoryMax, so the cap is what limits vips.
      virtualisation.memorySize = 3072;
    };
    beta = node true;
  };

  testScript = ''
    import re, shlex, time

    def get(m, path, *args, body=True):
        """GET (or whatever args say) on the public listener: code, headers, body."""
        code = m.succeed(
            "curl -sS -o /tmp/body -D /tmp/head -w '%{http_code}' "
            + " ".join(shlex.quote(a) for a in args)
            + " " + shlex.quote("http://127.0.0.1:8090" + path)
        )
        headers = {}
        for line in m.succeed("cat /tmp/head").splitlines()[1:]:
            k, sep, v = line.partition(":")
            if sep:
                headers[k.strip().lower()] = v.strip()
        return int(code), headers, (m.succeed("cat /tmp/body") if body else None)

    def page(m, path, *needles):
        code, _, html = get(m, path)
        assert code == 200, (path, code)
        for n in needles:
            assert n in html, (path, n)
        return html

    def as_service(m, cmd):
        return m.succeed("runuser -u asparges -- " + cmd)

    def no_errors(m):
        m.fail("journalctl -u asparges.service -o cat | grep -E 'level=ERROR|panic'")

    start_all()
    for m in (site, beta):
        m.wait_for_unit("asparges.service")
        m.wait_for_open_port(8090)

    with subtest("public pages answer with their content"):
        for path in ["/", "/lokaler", "/arrangementer", "/galleri", "/om-oss", "/kontakt"]:
            html = page(site, path, "<html lang=\"nb\"", "Aspargesgården", "<h1")
            assert "/admin" not in html, path

    with subtest("every old address redirects permanently"):
        for old, new in {
            "/facilities": "/lokaler",
            "/about": "/om-oss",
            "/contact": "/kontakt",
            "/events": "/arrangementer",
            "/christmas": "/julemarked",
            "/christmas/2026": "/arrangementer/2026/julemarked",
            "/gallery": "/galleri",
            "/lokaler/": "/lokaler",
        }.items():
            code, h, _ = get(site, old, body=False)
            assert (code, h.get("location")) == (301, new), (old, code, h.get("location"))

    with subtest("an unknown address is a real 404"):
        code, _, html = get(site, "/finnes-ikke")
        assert code == 404, code
        assert "Aspargesgården" in html
        # No julemarked is published yet, so it leads nowhere.
        assert get(site, "/julemarked", body=False)[0] == 404

    with subtest("sitemap, robots and llms.txt"):
        page(site, "/sitemap.xml", "<urlset", "https://beta.aspargesgaarden.no/lokaler")
        robots = page(site, "/robots.txt", "Sitemap: https://beta.aspargesgaarden.no/sitemap.xml")
        assert "Disallow: /\n" not in robots, robots
        page(site, "/llms.txt", "Aspargesgården")
        _, h, html = get(site, "/")
        assert "x-robots-tag" not in h, h
        assert 'content="max-image-preview:large"' in html

    with subtest("admin, metrics and debug are not on the public listener"):
        for path in ["/admin", "/admin/", "/admin/api/me", "/admin/api/events", "/metrics",
                     "/debug/", "/debug/pprof/", "/debug/vars", "/who"]:
            code, _, _ = get(site, path, body=False)
            assert code == 404, (path, code)
        for path in ["/admin/api/events", "/quitquitquit"]:
            code, _, _ = get(site, path, "-X", "POST", "-H", "Content-Type: application/json",
                             "--data", "{}", body=False)
            assert code in (404, 405), (path, code)
        site.succeed("systemctl is-active asparges.service")

    with subtest("noindex keeps the beta site out of search engines"):
        _, h, html = get(beta, "/")
        assert h.get("x-robots-tag") == "noindex", h
        assert "max-image-preview" not in html
        robots = page(beta, "/robots.txt")
        assert "User-agent: *\nDisallow: /\n" in robots, robots
        _, h, _ = get(beta, "/finnes-ikke", body=False)
        assert h.get("x-robots-tag") == "noindex", h

    with subtest("import an old checkout as the service user"):
        # vips comes with the package, not from the system.
        site.fail("command -v vips")
        # import wants serve stopped: a running one keeps its rendered pages.
        site.systemctl("stop asparges.service")
        as_service(site, "asparges import ${oldSite}")
        # Intake builds each photo's preview to prove it decodes; drop those
        # so that every variant below comes from the unit.
        site.succeed("rm -r /var/lib/asparges/variants")
        site.systemctl("start asparges.service")
        site.wait_for_open_port(8090)

    def variants(m, path):
        """The event page's photo URLs per format, each fetched and checked."""
        html = page(m, path)
        urls = sorted(set(re.findall(r"/img/v\d+/[0-9a-f]{64}/[a-z]+-[0-9a-f]{8}-\d+\.(?:avif|webp|jpg)", html)))
        found = {}
        for url in urls:
            ext = url.rsplit(".", 1)[1]
            # A 404 until the unit's start-up queue has built it.
            m.wait_until_succeeds("curl -sf -o /dev/null " + shlex.quote("http://127.0.0.1:8090" + url), timeout=600)
            code, h, _ = get(m, url, body=False)
            assert code == 200, (url, code)
            assert h["cache-control"] == "public, max-age=31536000, immutable", (url, h)
            ctype = {"avif": "image/avif", "webp": "image/webp", "jpg": "image/jpeg"}[ext]
            assert h["content-type"] == ctype, (url, h)
            # The bytes are what the name says.
            magic = bytes.fromhex("".join(m.succeed("od -An -tx1 -N12 /tmp/body").split()))
            assert {
                "avif": magic[4:12] == b"ftypavif",
                "webp": magic[:4] == b"RIFF" and magic[8:12] == b"WEBP",
                "jpg": magic[:3] == b"\xff\xd8\xff",
            }[ext], (url, magic)
            found.setdefault(ext, []).append(url)
        assert set(found) == {"avif", "webp", "jpg"}, found
        return found

    with subtest("imported events render with photos vips built inside the unit"):
        page(site, "/arrangementer", "Julemarked", "Vinsmaking")
        page(site, "/arrangementer/2030/julemarked", "Julemarked", "Kom og handle julegaver")
        page(site, "/arrangementer/2030/vinsmaking", "Vinsmaking", "Testfotograf", "ca. kl. 19:00")
        code, h, _ = get(site, "/julemarked", body=False)
        assert (code, h.get("location")) == (302, "/arrangementer/2030/julemarked"), (code, h)
        page(site, "/sitemap.xml", "/arrangementer/2030/julemarked", "/arrangementer/2030/vinsmaking")
        before = variants(site, "/arrangementer/2030/julemarked")
        # Holds the camera-sized photo, up to its widest AVIF.
        gallery = variants(site, "/galleri")
        no_errors(site)
        # Peak memory of the unit's cgroup, children included, against its cap.
        peak = site.succeed("systemctl show -P MemoryPeak asparges.service").strip()
        cap = site.succeed("systemctl show -P MemoryMax asparges.service").strip()
        print(f"asparges MemoryPeak={peak} MemoryMax={cap}")
        assert int(peak) < int(cap), (peak, cap)

    with subtest("SIGTERM stops the unit cleanly, well inside its timeout"):
        t0 = time.monotonic()
        site.systemctl("stop asparges.service")
        took = time.monotonic() - t0
        print(f"asparges stopped in {took:.1f}s")
        assert took < 30, took
        result = site.succeed("systemctl show -P Result asparges.service").strip()
        status = site.succeed("systemctl show -P ExecMainStatus asparges.service").strip()
        assert (result, status) == ("success", "0"), (result, status)
        # A clean close folds the WAL back into the database file.
        site.fail("test -s /var/lib/asparges/asparges.db-wal")

    with subtest("a restart keeps the data"):
        site.systemctl("start asparges.service")
        site.wait_for_open_port(8090)
        page(site, "/arrangementer/2030/julemarked", "Julemarked")
        site.systemctl("restart asparges.service")
        site.wait_for_open_port(8090)
        page(site, "/arrangementer/2030/vinsmaking", "Vinsmaking")

    with subtest("restore from the database and originals alone"):
        # What litestream (asparges.db) and restic (originals/) give back.
        site.systemctl("stop asparges.service")
        site.succeed(
            "mv /var/lib/asparges /var/lib/asparges.lost",
            "mkdir /var/lib/asparges",
            "cp -a /var/lib/asparges.lost/asparges.db /var/lib/asparges.lost/originals /var/lib/asparges/",
            "chown -R asparges:asparges /var/lib/asparges",
        )
        site.systemctl("start asparges.service")
        site.wait_for_open_port(8090)
        after = variants(site, "/arrangementer/2030/julemarked")
        assert after == before, (before, after)
        assert variants(site, "/galleri") == gallery
        no_errors(site)
  '';
}
