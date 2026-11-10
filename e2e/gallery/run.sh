#!/usr/bin/env bash
# Browser tests of the gallery (ui.Gallery and the elm/gallery lightbox) in
# Chromium, Firefox and WebKit, with Playwright from the pinned nixpkgs.
#
#   nix develop -c e2e/gallery/run.sh [pytest args]   e.g. -k swipe
#
# Builds static/dist (go generate), runs `asparges import` of the import
# tests' miniature old site (cmd/asparges/testdata/oldsite) into $DATA, a
# fresh directory unless set, serves it with `asparges serve --dev`, waits
# for the pictures the tested pages show, and runs test_gallery.py.
# Screenshots go to $OUT. BROWSERS narrows the browsers ("chromium webkit").
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
here=$root/e2e/gallery
out=${OUT:-$here/out}
port=${PORT:-8095}
cd "$root"

go generate ./static

tools=$(nix build --impure --no-link --print-out-paths -f "$here/tools.nix" python browsers mesa)
py=$(sed -n 1p <<<"$tools")/bin/python3
export PLAYWRIGHT_BROWSERS_PATH
PLAYWRIGHT_BROWSERS_PATH=$(sed -n 2p <<<"$tools")
export PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=true
# The dev shell's libraries (newer glibc) break the browsers' own.
unset LD_LIBRARY_PATH
# A crashing browser would drop gigabyte core files into the tree.
ulimit -c 0
# Headless WebKit still needs an EGL display; without the host's GL drivers
# it gets Mesa's software one.
if [ ! -d /run/opengl-driver ]; then
  mesa=$(sed -n 3p <<<"$tools")
  export __EGL_VENDOR_LIBRARY_FILENAMES=$mesa/share/glvnd/egl_vendor.d/50_mesa.json
  export LIBGL_DRIVERS_PATH=$mesa/lib/dri GBM_BACKENDS_PATH=$mesa/lib/gbm EGL_PLATFORM=surfaceless
fi

work=$(mktemp -d)
trap 'kill "$server" 2>/dev/null || true; rm -rf "$work"' EXIT
mkdir -p "$out"
data=${DATA:-$work/data}

go build -o "$work/asparges" ./cmd/asparges
"$work/asparges" import --data-dir "$data" "$root/cmd/asparges/testdata/oldsite" >"$work/import.log" 2>&1 || {
  cat "$work/import.log" >&2
  exit 1
}

"$work/asparges" serve --dev --listen-addr "127.0.0.1:$port" --base-url "http://127.0.0.1:$port" \
  --data-dir "$data" >"$work/log" 2>&1 &
server=$!
base=http://127.0.0.1:$port
until curl -sf -o /dev/null "$base/galleri"; do
  if ! kill -0 "$server" 2>/dev/null; then
    cat "$work/log" >&2
    exit 1
  fi
  sleep 0.2
done

# serve builds every size in the background; a picture is a 404 until then.
pages=$(curl -sf "$base/galleri" "$base/arrangementer/2026/julemarked")
for u in $(grep -o '/img/[^"&, ]*' <<<"$pages" | sort -u); do
  curl -sf --retry 300 --retry-delay 1 --retry-all-errors -o /dev/null "$base$u"
done

photos=$(curl -sf "$base/galleri" | grep -o 'data-index="[0-9]*"' | sort -u | wc -l)
BASE=$base OUT=$out PHOTOS=$photos "$py" -I -m pytest -p no:cacheprovider "$here/test_gallery.py" "$@"
