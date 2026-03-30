# Aspargesgaarden

Elm/elm-pages 2 static site for [aspargesgaarden.no](https://aspargesgaarden.no).

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
