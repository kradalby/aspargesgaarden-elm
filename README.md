# Aspargesgaarden

Elm/elm-pages 2 static site for [aspargesgaarden.no](https://aspargesgaarden.no).

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
