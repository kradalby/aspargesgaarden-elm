# Aspargesgaarden

Elm/elm-pages 2 static site for [aspargesgaarden.no](https://aspargesgaarden.no).

## asparges (the Go rewrite on this branch)

The new site is one Go binary. Its stylesheet, fonts and fixed-page photos
are built by `static/build.sh` into `static/dist/` (git-ignored) and embedded
with `go:embed`. The Nix build runs the same script, so nothing here differs
from what ships.

```bash
nix develop
go generate ./static        # CSS, WOFF2 fonts, photo variants into static/dist
go test ./...               # needs static/dist; the first generate takes ~45 s
go run ./cmd/asparges serve --dev --listen-addr 127.0.0.1:8090 \
  --base-url http://127.0.0.1:8090 --data-dir /tmp/asparges
```

- Changed Go markup or classes: rerun `go generate ./static` (Tailwind only
  emits classes it finds in `ui/`, `pages/`, `markdown/`, `elm/`; fonts are skipped
  unless their source is newer, photo variants unless missing), then restart
  `go run`.
- Fixed-page photos: one master per photo in `static/photos-src/`
  (`<name>.jpg`, at most ~2400 px and under 1 MB). The photo step runs
  `go run ./cmd/imagesbuild`, the same `images` pipeline that builds uploaded
  photos, into `static/dist/photos/` with a `manifest.json`; pages ask for
  `static.Photo("<name>")`. A replaced master gets new variant names, and the
  old ones are removed on the next generate.
- Events and gallery from the old site: `go run ./cmd/asparges import
  --data-dir /tmp/asparges .` (this checkout; about a minute, mostly the
  gallery photos). Run it before `serve`: a running server keeps its
  rendered pages until the next change. Running it again changes nothing.
- Changed markup on purpose: `UPDATE_GOLDEN=1 go test ./...`, then review
  the diff of the `testdata/*.html` golden files.
- Avenir's 400 weight is Medium; switch to Roman by changing the one
  `@font-face` line in `static/theme.css`.
- Pages may load nothing from outside the binary except the origins in
  `ui/external.go` (Umami, the Google calendar frame). Tests check the
  rendered pages and the stylesheet; every public response carries the
  matching Content-Security-Policy.
- Settings beside `--base-url`: `--noindex` (beta: kept out of search
  engines) and `--umami-site-id` (`ASPARGES_UMAMI_SITE_ID`, module option
  `services.asparges.umamiSiteId`): the site's id on umami.kradalby.no.
  Unset, pages carry no tracker and the CSP no Umami origin; set, the
  tracker reports only from the host of `--base-url`.
- Every `serve` setting is also a `services.asparges` option in
  `nix/module.nix` (`dataDir`, `listenAddr`, `hostname`, `baseURL`,
  `tailscaleKeyPath`, `noindex`, `umamiSiteId`, `logLevel`); a test in
  `cmd/asparges` fails on a flag the module does not pass. Only the
  dev-only `--dev` and `--e2e-fixtures` are left out on purpose.

### Elm apps and the gallery

Each Elm app lives in `elm/<app>/` with its own `elm.json`, its packages
pinned for the offline Nix build in `elm-srcs.nix` and `registry.dat`, and
main module `src/<App>.elm`. `go generate ./static` builds every app
(`elm make --optimize`, then esbuild minifies) into `static/dist/<app>.js`,
with `elm/<app>/boot.js` appended when present; pages run no inline script.

```bash
go generate ./static                    # or, Elm only: cd static && ./build.sh dist gallery
cd elm/gallery && elm-test-rs           # lightbox logic; also `nix flake check`
cd elm/gallery && elm2nix convert > elm-srcs.nix && elm2nix snapshot   # after editing elm.json
nix develop -c e2e/gallery/run.sh       # browser tests: Chromium, Firefox, WebKit
```

A new app: add `elm/<app>/` and its name to `elmApps` in `flake.nix`; the
build, the `elm-test-<app>` check and the Tailwind scan of its classes
follow. An app whose `boot.js` imports npm packages (the admin: Cropper.js,
the Markdown toolbar) pins them in its own `package.json` and `yarn.lock`;
build.sh bundles `boot.js` with esbuild, and Nix fetches the packages with
`fetchYarnDeps` (hash under `yarn.<app>` in `flakehashes.json`).

### Admin

`elm/admin` is the admin app: `/static/admin.js` and its own stylesheet
`/static/admin.css` (`static/admin-input.css`, sharing `static/theme.css`
with `site.css`, so public pages carry no admin classes).
`pages.AdminHandler()` serves the document it runs in, under
`ui.AdminCSP()`. The API it talks to is `docs/admin-api.md`.

```bash
cd elm/admin && elm-test-rs              # decoders, form rules, crop arithmetic
nix develop -c e2e/admin/run.sh         # Chromium at 1280 and 390 px, against the fake API
```

`e2e/admin/run.sh` serves the real admin document and files from Go in
front of `e2e/admin/fake-server.mjs` (an in-memory stand-in for the API)
and writes screenshots of every screen to `e2e/admin/out/`. `BASE=<url>`
points the specs at another server, such as `serve --dev`, which must then
answer the test hooks `POST /__reset` and `POST /__touch/<id>`.

The gallery is `ui.Gallery(layout, photos...)` (`GalleryPage` on
`/galleri`, `GalleryColumn` at the end of an event page): a grid of links to
the large photos that works without JavaScript. `ui.Page` adds
`/static/gallery.js` to any page holding one, and the lightbox opens over
every gallery on the page. `e2e/gallery/run.sh` imports this checkout into
`~/.cache/asparges-e2e/data` (kept with its variants; the first run takes a
few minutes), serves it with `serve`'s wiring and the site's CSP, and writes
screenshots to `e2e/gallery/out/`.

### Admin

The admin (`/admin/`, its JSON API under `/admin/api/`, see
`docs/admin-api.md`) is served only on the service's tsnet node, to tailnet
users the policy file grants the app capability `kradalby.no/cap/asparges`
with `{"admin": true}`. Reaching the node is not enough (Prometheus does):

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

In `--dev` the loopback listener carries the admin and everyone is
"Utvikler". `--e2e-fixtures <the front end's e2e/admin/fixtures>` adds the
browser tests' hooks (`POST /__reset` deletes everything and seeds their
data; `POST /__touch/{id}`), so they run against the real server; it is
refused without `--dev`.

## elm-pages version (pinned to v2)

This site is intentionally pinned to **elm-pages 2** (`elm-pages@2.1.12` on npm,
`dillonkearns/elm-pages@9.0.0` on the Elm side). Do **not** upgrade to elm-pages 3+:
v3 is a ground-up rewrite (different routing, data-fetching and build model) that
would require rewriting the site. Dependency sweeps must keep these two pins; only
the surrounding tooling (Nix inputs, CI actions) and v2-compatible packages are
bumped.

## Getting started

Enter the Nix development shell (provides Elm, Node.js, Yarn, and all build tools):

```
nix develop
```

## Development

```bash
# Start dev server with hot reload
elm-pages dev

# Build production site (outputs to dist/)
elm-pages build

# Format Elm code
elm-format src/ --yes
```

## Images

After adding or replacing images in `public/`, generate resized and optimized versions:

```bash
# Process images (resize + WebP/AVIF conversion, incremental)
process-images

# Verbose output (shows which images are processed/skipped)
process-images -v
```

Images are resized to multiple widths (320-2048px) and converted to WebP and AVIF formats.
Generated files (`*_*w_resize.jpeg`, `*.webp`, `*.avif`) are excluded from git.

Nix builds the variants for each original JPEG separately. Adding or replacing a
photo rebuilds only that photo's variants; unchanged photos reuse their existing
Nix store outputs or the CI binary cache. A lightweight build collects the results
into `processedImages`. Changes to the image processing script or its pinned
toolchain rebuild the affected image outputs.

## Tailwind CSS

Regenerate Tailwind CSS Elm modules after changing `tailwind.config.js`:

```
yarn elm-tailwind-modules --tailwind-config tailwind.config.js --dir ./src
```

## Build with Nix

```bash
# Build the full site as a Nix package
nix build

# Only process images
nix build .#processedImages
```
