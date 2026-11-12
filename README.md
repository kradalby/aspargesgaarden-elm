# asparges

The web server for [aspargesgaarden.no](https://aspargesgaarden.no): one Go
binary that renders the public site and serves a small admin app to the
farm's owners over Tailscale.

- **Public pages** are HTML rendered in Go with
  [elem-go](https://github.com/chasefleming/elem-go) components. They work
  without JavaScript; the only scripts are the gallery lightbox and the
  booking calendar on the contact page (both Elm), each loaded only where it
  is used. Pages, the 404 page, `/sitemap.xml` and `/llms.txt`
  are kept in memory after the first request and thrown away on any change;
  only the admin preview is rendered every time.
- **Content** (events, the Christmas market, the gallery, photographers)
  lives in SQLite. Photos are uploaded originals, cropped and resized by
  [libvips](https://www.libvips.org/) into AVIF, WebP and JPEG.
- **The admin** (`/admin/`, an Elm app and its JSON API) is reachable only on
  the service's own tsnet node, for tailnet users granted the app capability
  `kradalby.no/cap/asparges`.
- **Deployment** is a NixOS module, `services.asparges`; see
  [docs/deploy.md](docs/deploy.md).

Open work is in [TODO.md](TODO.md). Rules for changing the code are in
[AGENTS.md](AGENTS.md).

## Layout

| Path                       | What                                                                                                                         |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `cmd/asparges`             | The binary: `serve`, `import`. Flags also read `ASPARGES_*` environment variables.                                           |
| `cmd/imagesbuild`          | Builds the embedded photos' variants at build time, with the same `images` code.                                             |
| `cmd/dashboard`            | Prints the Grafana dashboard JSON (`nix build .#grafanaDashboards`).                                                         |
| `cmd/vendorhash`           | Keeps `flakehashes.json` in step with `go.mod`.                                                                              |
| `ui/`                      | The component kit: one file per component, typed props, golden tests.                                                        |
| `pages/`                   | Pages composed from `ui/`, and the admin's HTML document.                                                                    |
| `markdown/`                | Event text (Markdown) to elem-go nodes. Raw HTML is off.                                                                     |
| `seo/`                     | Head tags, JSON-LD, sitemap, robots, IndexNow, `llms.txt`, crawler metrics.                                                  |
| `render/`                  | The lazy page store: render once, keep bytes, gzip and ETag.                                                                 |
| `server/`                  | Routes, redirects from the old addresses, the admin API and its capability check.                                            |
| `site/`                    | Domain logic: events, gallery, slugs, Oslo dates, publish rules.                                                             |
| `db/`                      | `schema.sql`, `queries/`, sqlc output in `dbsqlc/`.                                                                          |
| `images/`                  | Every image file: originals, crops, variants, URLs, the vips pipeline, the build worker.                                     |
| `static/`                  | Embedded assets: Tailwind input, font sources, logo and icons, fixed-page photo masters, and `build.sh` that builds `dist/`. |
| `elm/<app>`                | The Elm apps: `admin`, `gallery` (the lightbox), `calendar` (the contact page's booking calendar).                           |
| `golden/`                  | The golden-file helper.                                                                                                      |
| `nix/`                     | The NixOS module, its eval test, two VM tests, the hardening smoke, the slim vips.                                           |
| `e2e/admin`, `e2e/gallery` | Browser suites.                                                                                                              |
| `docs/`                    | [Admin API contract](docs/admin-api.md), [deploy runbook](docs/deploy.md).                                                   |

## Build

```bash
nix build              # ./result/bin/asparges, vips on its PATH
nix flake check        # every check below, including two NixOS VM tests (need KVM)
```

The Nix build is offline: Go modules, Elm packages and the admin's npm
packages are pinned (`flakehashes.json`, `elm/<app>/elm-srcs.nix`,
`elm/admin/yarn.lock`) and everything the pages use is embedded in the binary.

## Local loop

```bash
nix develop
go generate ./static   # static/dist: CSS, WOFF2 fonts, photo variants, Elm apps
go test ./...          # needs static/dist and vips (the dev shell has it)
go run ./cmd/asparges import --data-dir /tmp/asparges cmd/asparges/testdata/oldsite
go run ./cmd/asparges serve --dev --data-dir /tmp/asparges \
  --listen-addr 127.0.0.1:8090 --base-url http://127.0.0.1:8090
```

Open <http://127.0.0.1:8090/> and the admin at
<http://127.0.0.1:8090/admin/>. `--dev` runs without Tailscale: the admin and
metrics share the loopback listener and everyone is "Utvikler". The import
fills an empty data directory with the test fixture (events and a gallery);
without it the site starts empty and the admin fills it.

- `go generate ./static` again after changing classes in Go or Elm code
  (Tailwind emits only the classes it finds), a font, a photo master or an
  Elm app; then restart `serve`. Fonts and photo variants are rebuilt only
  when their source changed. `cd static && ./build.sh dist admin` builds one
  step.
- Changed markup or an admin API example on purpose:
  `UPDATE_GOLDEN=1 go test ./...`, then review the diff of the
  `testdata/*.html` golden files (JSON in them indented) and of
  `elm/admin/tests/ApiExamples.elm`.
- Changed `db/schema.sql` or `db/queries/`: `sqlc generate`. A schema change
  also needs an update rule in `db/db.go` and the replaced schema in
  `db/testdata/` (see AGENTS.md).
- Changed `go.mod`: `go run ./cmd/vendorhash update`.
- Changed an Elm app's `elm.json`: `cd elm/<app> && elm2nix convert > elm-srcs.nix && elm2nix snapshot`.
- Changed `elm/admin/yarn.lock`: put the new hash under `yarn.admin` in
  `flakehashes.json` (the build's error prints it).

`nix fmt` formats everything; the pre-commit hooks (`prek install`) run it,
golangci-lint, `go test -race -short`, `vendorhash check` and the stock file
checks (whitespace, large files, private keys).

## Running it

`asparges serve` takes these settings (flag, `ASPARGES_*` variable, and
`services.asparges` option are the same setting):

| Flag                        | Module option      | Default                      |                                                                                 |
| --------------------------- | ------------------ | ---------------------------- | ------------------------------------------------------------------------------- |
| `--data-dir`                | `dataDir`          | `/var/lib/asparges`          | Database, photos, tsnet identity.                                               |
| `--listen-addr`             | `listenAddr`       | `127.0.0.1:8090`             | Public site; nginx proxies to it.                                               |
| `--base-url`                | `baseURL`          | `https://aspargesgaarden.no` | Origin for canonical and absolute links.                                        |
| `--hostname`                | `hostname`         | `asparges`                   | The admin's tsnet node name.                                                    |
| `--tailscale-auth-key-path` | `tailscaleKeyPath` | none                         | Auth key file for the first start; `TS_AUTHKEY` also works.                     |
| `--noindex`                 | `noindex`          | off                          | Beta: `X-Robots-Tag: noindex`, `robots.txt` disallows all, no IndexNow.         |
| `--umami-site-id`           | `umamiSiteId`      | none                         | Umami site on umami.kradalby.no. Unset: no tracker, no Umami origin in the CSP. |
| `--calendar-url`            | `calendarURL`      | none                         | Public iCal feed of the booking calendar. Unset: no calendar on `/kontakt`.     |
| `--log-level`               | `logLevel`         | `info`                       |                                                                                 |

`--dev`, `--e2e-fixtures` and `--e2e-clock` are for local work and tests
only: no module option, and `serve` refuses to start when `ASPARGES_DEV`,
`ASPARGES_E2E_FIXTURES` or `ASPARGES_E2E_CLOCK` is set.

State lives in the data directory: `asparges.db` (WAL mode),
`originals/` (uploads as received), `variants/` (built sizes), `tmp/`,
`tsnet/`, and `calendar.json` (the last whole read of the booking calendar
and its time: read every 5 minutes, shown up to 30 minutes after it). A
restore needs
only the database and `originals/`: at start
`serve` queues every photo and builds the sizes that are missing, one photo at
a time; until a size is built, its picture is missing on the page.

Old addresses (`/events`, `/christmas/2026`, `/gallery`, …) redirect to the
Norwegian ones; a table test in `server/` covers every row.

### Admin access

The admin and its API answer only on the tsnet node, and only to callers
whose tailnet policy grants:

```json
{
  "grants": [
    {
      "src": ["group:asparges-admin"],
      "dst": ["tag:asparges"],
      "ip": ["tcp:80", "tcp:443"],
      "app": { "kradalby.no/cap/asparges": [{ "admin": true }] }
    }
  ]
}
```

Reaching the node is not enough: Prometheus reaches it for `/metrics` and
gets `403` on `/admin/`. The API contract is
[docs/admin-api.md](docs/admin-api.md).

## Tests

- `go test ./...` runs the unit, golden and HTTP tests, and the image tests
  against the real vips.
- `nix flake check` runs every check: `build`, `gotest` (race, shuffled),
  `golangci-lint`, `formatting`, `generate` (sqlc drift), `photo-variants`,
  `elm-test-admin`, `elm-test-gallery`, `module-eval`, and on x86_64-linux
  the VM tests `vm-basic` (the unit, import, restore) and `vm-tsnet` (the
  grant against headscale). CI runs each as its own job on x86_64 and
  aarch64, the lists read from the flake, and on aarch64 also the unit's
  sandbox on a real kernel (`nix build .#hardening-smoke`, run as root).
- `cd elm/<app> && elm-test-rs` runs one Elm app's tests.

### Browser suites

Two Playwright suites, from the pinned nixpkgs, against a local server.
Neither touches the network once its tools are built.

```bash
nix develop -c e2e/admin/run.sh    # admin in Chromium and WebKit at 1280 and 390 px, against serve --dev
nix develop -c e2e/gallery/run.sh  # gallery and lightbox in Chromium, Firefox and WebKit
```

The admin suite starts `serve --dev --e2e-fixtures e2e/admin/fixtures` on a
fresh data directory and also checks the public pages the admin changes.
The gallery suite imports `cmd/asparges/testdata/oldsite` and serves it with
the server's wiring. Both write screenshots to `e2e/<suite>/out/`;
`[playwright or pytest args]` narrow them (`-g crop`, `-k swipe`).

Run them after changing the admin (`elm/admin`, `server/admin*`,
`docs/admin-api.md`) or the gallery (`ui/gallery.go`, `elm/gallery`), and both
before a deploy. They are not in CI: they run the local loop, whose first
`go generate` downloads Elm and npm packages that the offline Nix build
pins, and both suites need several browser engines and software GL.
That is a lot of setup on every push for checks that only matter when those
parts change.
