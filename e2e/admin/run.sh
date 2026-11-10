#!/usr/bin/env bash
# Browser tests of the admin app (elm/admin) in Chromium at desktop and
# phone size, with Playwright's test runner from the pinned nixpkgs.
#
#   nix develop -c e2e/admin/run.sh [playwright args]   e.g. -g crop --project phone
#
# Builds static/dist (go generate) and runs the specs against
# `asparges serve --dev --e2e-fixtures` on a fresh data directory, so they
# also see the public pages the admin changes. BASE points them at a server
# already running with the test hooks instead. Screenshots of every screen
# go to $OUT.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
here=$root/e2e/admin
out=${OUT:-$here/out}
port=${PORT:-18096}
cd "$root"

tools=$(nix build --impure --no-link --print-out-paths -f "$here/tools.nix" playwright)
playwright=$(sed -n 1p <<<"$tools")/bin/playwright
# The specs are ES modules, which find "@playwright/test" through a
# node_modules folder only (NODE_PATH does not apply); git ignores it.
ln -sfn "$(sed -n 1p <<<"$tools")/lib/node_modules" "$here/node_modules"
# The dev shell's libraries (newer glibc) break the browsers' own.
unset LD_LIBRARY_PATH
ulimit -c 0
mkdir -p "$out"

if [ -z "${BASE:-}" ]; then
  go generate ./static

  work=$(mktemp -d) server=
  trap 'kill "$server" 2>/dev/null || true; wait; rm -rf "$work"' EXIT

  go build -o "$work/asparges" ./cmd/asparges
  "$work/asparges" serve --dev --listen-addr "127.0.0.1:$port" --base-url "http://127.0.0.1:$port" \
    --data-dir "$work/data" --e2e-fixtures "$here/fixtures" >"$work/serve.log" 2>&1 &
  server=$!
  until curl -sf -o /dev/null "http://127.0.0.1:$port/admin/"; do
    if ! kill -0 "$server" 2>/dev/null; then
      cat "$work/serve.log" >&2
      exit 1
    fi
    sleep 0.2
  done

  export BASE=http://127.0.0.1:$port
fi

cd "$here"
OUT=$out "$playwright" test "$@"
