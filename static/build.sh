#!/usr/bin/env bash
# Builds everything static.go embeds from dist/: the stylesheet, the WOFF2
# fonts it names and the photo variants. The Nix build and `go generate` both
# run this, so the binary and the local loop cannot drift apart.
#
#   build.sh [OUT [css] [fonts] [photos]]    default: dist, all three
#
# Needs tailwindcss (v4), woff2_compress and vips on PATH. Run from static/.
# Fonts and photos are only rebuilt when their source is newer. Nix runs the
# photo step in its own derivation so a Go edit never re-encodes photos.
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

# shortcut: a fixed width ladder per photo; the images package's build-time
# manifest replaces this step together with photos.go.
photos() {
  local widths="480 800 1200 1600 2000 2500"
  find photos-src -name '*.jpeg' | sort | while read -r src; do
    local rel=${src#photos-src/}
    local base=$out/photos/${rel%.jpeg}
    mkdir -p "$(dirname "$base")"
    local master
    master=$(vipsheader -f width "$src")
    for w in $widths; do
      [ "$w" -le "$master" ] || continue
      for fmt in "avif[Q=50,effort=4,keep=none]" "webp[Q=75,keep=none]" "jpeg[Q=80,optimize_coding,interlace,keep=none]"; do
        local ext=${fmt%%\[*}
        local dst=$base-$w.$ext
        [ "$dst" -nt "$src" ] && continue
        vips thumbnail "$src" "$dst${fmt#"$ext"}" "$w" --height 100000 --size down --export-profile srgb
      done
    done
  done
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
