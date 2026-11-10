#!/usr/bin/env bash
# Browser tests of the admin app (elm/admin) in Chromium at desktop and phone
# size, with Playwright's test runner from the pinned nixpkgs.
#
#   nix develop -c e2e/admin/run.sh [playwright args]   e.g. -g crop --project phone
#
# Builds static/dist (go generate), starts the fake API (fake-server.mjs)
# and the Go server that serves the real admin document and files in front
# of it (server/), then runs the specs. BASE points them at another server
# instead (the real API in --dev, with the same test hooks), which skips
# both. Screenshots of every screen go to $OUT.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
here=$root/e2e/admin
out=${OUT:-$here/out}
port=${PORT:-18096}
cd "$root"

tools=$(nix build --impure --no-link --print-out-paths -f "$here/tools.nix" playwright node)
playwright=$(sed -n 1p <<<"$tools")/bin/playwright
node=$(sed -n 2p <<<"$tools")/bin/node
# The specs are ES modules, which find "@playwright/test" through a
# node_modules folder only (NODE_PATH does not apply); git ignores it.
ln -sfn "$(sed -n 1p <<<"$tools")/lib/node_modules" "$here/node_modules"
# The dev shell's libraries (newer glibc) break the browsers' own.
unset LD_LIBRARY_PATH
ulimit -c 0
mkdir -p "$out"

if [ -z "${BASE:-}" ]; then
  go generate ./static

  work=$(mktemp -d) fake="" server=""
  trap 'kill "$fake" "$server" 2>/dev/null || true; rm -rf "$work"' EXIT

  go build -o "$work/server" ./e2e/admin/server
  "$node" "$here/fake-server.mjs" --port $((port + 1)) >"$work/fake.log" 2>&1 &
  fake=$!
  "$work/server" -addr "127.0.0.1:$port" -fake "http://127.0.0.1:$((port + 1))" >"$work/server.log" 2>&1 &
  server=$!

  for p in fake server; do
    until grep -q '^ready' "$work/$p.log"; do
      if ! kill -0 "${!p}" 2>/dev/null; then
        cat "$work/$p.log" >&2
        exit 1
      fi
      sleep 0.2
    done
  done

  export BASE=http://127.0.0.1:$port
fi

cd "$here"
OUT=$out "$playwright" test "$@"
