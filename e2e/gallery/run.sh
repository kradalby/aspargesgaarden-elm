#!/usr/bin/env bash
# Browser tests of the public pages' scripts in Chromium, Firefox and WebKit,
# with Playwright from the pinned nixpkgs: the gallery (ui.Gallery and the
# elm/gallery lightbox) and the booking calendar (ui.BookingCalendar and
# elm/calendar).
#
#   nix develop -c e2e/gallery/run.sh [pytest args]   e.g. -k swipe
#
# Builds static/dist (go generate), runs `asparges import` of the import
# tests' miniature old site (cmd/asparges/testdata/oldsite) into $DATA, a
# fresh directory unless set, serves it with `asparges serve --dev` on the
# tests' fixed day with calendar.ics as the feed, waits
# for the pictures the tested pages show, and runs test_gallery.py and
# test_calendar.py; then again with a feed that never answers for
# test_calendar_hidden.py.
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

work=$(mktemp -d) server=
feed=
trap 'kill "$server" $feed 2>/dev/null || true; rm -rf "$work"' EXIT
mkdir -p "$out"
data=${DATA:-$work/data}

go build -o "$work/asparges" ./cmd/asparges
"$work/asparges" import --data-dir "$data" "$root/cmd/asparges/testdata/oldsite" >"$work/import.log" 2>&1 || {
  cat "$work/import.log" >&2
  exit 1
}

# Another server on the port would answer the tests in place of this one.
if (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null; then
  echo "port $port is in use; set PORT to a free one" >&2
  exit 1
fi
# calendar.ics is the feed, served as Google would.
feedport=$((port + 1))
"$py" -I -m http.server --bind 127.0.0.1 --directory "$here" "$feedport" >"$work/feed.log" 2>&1 &
feed=$!
until curl -sf -o /dev/null "http://127.0.0.1:$feedport/calendar.ics"; do sleep 0.2; done

serve() {
  "$work/asparges" serve --dev --e2e-clock --listen-addr "127.0.0.1:$port" --base-url "http://127.0.0.1:$port" \
    --data-dir "$data" --calendar-url "$1" >"$work/log" 2>&1 &
  server=$!
}
serve "http://127.0.0.1:$feedport/calendar.ics"
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
status=0
BASE=$base OUT=$out PHOTOS=$photos "$py" -I -m pytest -p no:cacheprovider "$here/test_gallery.py" "$here/test_calendar.py" "$@" || status=$?

# Again with a feed that never answers and no copy kept: the calendar the
# site cannot vouch for.
kill "$server"
wait "$server" 2>/dev/null || true
rm -f "$data/calendar.json"
serve http://127.0.0.1:9/calendar.ics
until curl -sf -o /dev/null "$base/kontakt"; do sleep 0.2; done
# 5 is "no tests": a -k for the other phase.
BASE=$base OUT=$out "$py" -I -m pytest -p no:cacheprovider "$here/test_calendar_hidden.py" "$@" || [ $? = 5 ] || status=1
exit $status
