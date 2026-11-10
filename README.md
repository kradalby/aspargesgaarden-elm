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
  emits classes it finds in `ui/`, `pages/`, `markdown/`; fonts are skipped
  unless their source is newer, photo variants unless missing), then restart
  `go run`.
- Fixed-page photos: one master per photo in `static/photos-src/`
  (`<name>.jpg`, at most ~2400 px and under 1 MB). The photo step runs
  `go run ./cmd/imagesbuild`, the same `images` pipeline that builds uploaded
  photos, into `static/dist/photos/` with a `manifest.json`; pages ask for
  `static.Photo("<name>")`. A replaced master gets new variant names, and the
  old ones are removed on the next generate.
- Changed markup on purpose: `UPDATE_GOLDEN=1 go test ./...`, then review
  the diff of the `testdata/*.html` golden files.
- Avenir's 400 weight is Medium; switch to Roman by changing the one
  `@font-face` line in `static/input.css`.
- Pages may load nothing from outside the binary except the origins in
  `ui/external.go` (Umami, the Google calendar frame). Tests check the
  rendered pages and the stylesheet; every public response carries the
  matching Content-Security-Policy.

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
