#!/usr/bin/env bash
# Builds the admin front end into elm/admin/out for the fake server and the
# browser tests, the same steps as nix/admin.nix. Run inside `nix develop`.
set -euo pipefail
cd "$(dirname "$0")/.."
out=elm/admin/out
mkdir -p "$out"

(cd js && yarn install --frozen-lockfile --silent)
esbuild js/src/admin.js --bundle --minify --format=iife --target=es2020 \
  --loader:.css=text --log-level=warning --outfile="$out/elements.js"

(cd elm/admin && elm make src/Main.elm --optimize --output="out/elm.js" >/dev/null)
esbuild "$out/elm.js" --minify --log-level=warning --outfile="$out/app.js"
rm "$out/elm.js"

tailwindcss --input elm/admin/admin.css --output "$out/admin.css" --minify 2>/dev/null
