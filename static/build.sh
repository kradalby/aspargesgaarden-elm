#!/usr/bin/env bash
# Builds everything static.go embeds from dist/: the stylesheet, the WOFF2
# fonts it names and the photo variants. The Nix build and `go generate` both
# run this, so the binary and the local loop cannot drift apart.
#
#   build.sh [OUT [css] [fonts] [photos]]    default: dist, all three
#
# Needs tailwindcss (v4), woff2_compress, vips, and imagesbuild or go on PATH.
# Run from static/. Fonts are rebuilt when their source is newer, photo
# variants when missing. Nix runs the photo step in its own derivation so a
# Go edit never re-encodes photos.
set -euo pipefail

out=${1:-dist}
shift || true
steps=${*:-css fonts photos}

css() {
  tailwindcss --input input.css --output "$out/site.css" --minify 2>/dev/null
}

# Convert exactly the fonts input.css references, so the weight mapping lives
# in one place.
fonts() {
  mkdir -p "$out/fonts"
  for name in $(grep -o '/static/fonts/[A-Za-z0-9-]*\.woff2' input.css | sed 's|.*/||; s|\.woff2$||' | sort -u); do
    local src=fonts-src/$name.otf dst=$out/fonts/$name.woff2
    [ "$dst" -nt "$src" ] && continue
    local tmp
    tmp=$(mktemp -d)
    cp "$src" "$tmp/"
    woff2_compress "$tmp/$name.otf" >/dev/null
    mv "$tmp/$name.woff2" "$dst"
    rm -r "$tmp"
  done
}

# The images package's pipeline, the one that builds uploaded photos too:
# Full variants of every master plus the manifest static.Photo reads. Nix
# puts imagesbuild on PATH; locally it runs from source.
photos() {
  local ib=(imagesbuild)
  command -v imagesbuild >/dev/null || ib=(go run ../cmd/imagesbuild)
  "${ib[@]}" --src photos-src --out "$out/photos" --url-prefix /static/photos
}

mkdir -p "$out"
for step in $steps; do
  case $step in
    css | fonts | photos) "$step" ;;
    *)
      echo "build.sh: unknown step $step" >&2
      exit 2
      ;;
  esac
done
