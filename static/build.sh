#!/usr/bin/env bash
# Builds everything static.go embeds from dist/: the stylesheet, the WOFF2
# fonts it names, the photo variants and the Elm apps. The Nix build and
# `go generate` both run this, so the binary and the local loop cannot drift
# apart.
#
#   build.sh [OUT [css] [fonts] [photos] [<app>...]]   default: dist, all
#
# <app> is a directory under ../elm. Needs tailwindcss (v4), woff2_compress,
# vips, imagesbuild or go, elm and esbuild on PATH, and yarn for an app with
# a package.json. Run from static/. Fonts are rebuilt when their source is
# newer, photo variants when missing. Nix runs the photo step and each app in
# their own derivations so a Go edit never re-encodes photos.
set -euo pipefail

out=${1:-dist}
shift || true
apps=$(for f in ../elm/*/elm.json; do basename "$(dirname "$f")"; done)
steps=${*:-css fonts photos $apps}

# site.css for the public pages, admin.css for the admin app: one theme
# (theme.css), two class scans, so a visitor never downloads admin styles.
css() {
  tailwindcss --input input.css --output "$out/site.css" --minify 2>/dev/null
  tailwindcss --input admin-input.css --output "$out/admin.css" --minify 2>/dev/null
}

# Convert exactly the fonts input.css references, so the weight mapping lives
# in one place.
fonts() {
  mkdir -p "$out/fonts"
  for name in $(grep -oh '/static/fonts/[A-Za-z0-9-]*\.woff2' ./*.css | sed 's|.*/||; s|\.woff2$||' | sort -u); do
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

# An Elm app ../elm/<app> with main module src/<App>.elm, optimized and
# minified to <app>.js. The app's boot.js, if any, is appended: it mounts the
# app, and pages run no inline script (CSP script-src 'self'). An app with a
# package.json has a boot.js that imports pinned packages (yarn.lock): it is
# bundled on its own and appended after the Elm part, so the Elm-only pure
# markers below never touch the packages' code. Nix provides node_modules;
# locally the first build installs it.
elm_app() {
  local app=$1 tmp dir=../elm/$1
  tmp=$(mktemp -d)
  (cd "$dir" && elm make "src/${app^}.elm" --optimize --output "$tmp/elm.js" >/dev/null)
  if [ -f "$dir/package.json" ]; then
    [ -d "$dir/node_modules" ] || (cd "$dir" && yarn install --frozen-lockfile --silent)
    esbuild "$dir/boot.js" --bundle --minify --format=iife --target=es2020 --loader:.css=text \
      --legal-comments=eof --log-level=warning --outfile="$tmp/boot.js"
  elif [ -f "$dir/boot.js" ]; then
    cat "$dir/boot.js" >>"$tmp/elm.js"
  fi
  # Elm's curried-call helpers are pure; marking them lets esbuild drop the
  # unused ones (the minify recipe in Elm's optimization guide).
  esbuild "$tmp/elm.js" --minify --target=es2020 --log-level=warning --outfile="$out/$app.js" \
    --pure:F2 --pure:F3 --pure:F4 --pure:F5 --pure:F6 --pure:F7 --pure:F8 --pure:F9 \
    --pure:A2 --pure:A3 --pure:A4 --pure:A5 --pure:A6 --pure:A7 --pure:A8 --pure:A9
  if [ -f "$tmp/boot.js" ]; then cat "$tmp/boot.js" >>"$out/$app.js"; fi
  rm -r "$tmp"
}

mkdir -p "$out"
for step in $steps; do
  case $step in
    css | fonts | photos) "$step" ;;
    *)
      if [ ! -f "../elm/$step/elm.json" ]; then
        echo "build.sh: unknown step $step" >&2
        exit 2
      fi
      elm_app "$step"
      ;;
  esac
done
