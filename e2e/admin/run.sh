#!/usr/bin/env bash
# Browser tests of the admin app (elm/admin) in Chromium at desktop and phone
# size, with Playwright's test runner from the pinned nixpkgs.
#
#   nix develop -c e2e/admin/run.sh [playwright args]   e.g. -g crop --project phone
#
# Builds static/dist (go generate) and, by default, runs the specs against
# the real thing: `asparges serve --dev --e2e-fixtures` on a fresh data
# directory, so they also see the public pages the admin changes. FAKE=1 is
# the fast mode: the fake API (fake-server.mjs) behind a Go server that
# serves the real admin document and files (server/); the specs skip what
# only a real site has. BASE points them at a server already running (with
# the test hooks) instead. Screenshots of every screen go to $OUT.
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

# wait_ready LOG PID URL: until URL answers, or the process is gone.
wait_ready() {
  until curl -sf -o /dev/null "$3"; do
    if ! kill -0 "$2" 2>/dev/null; then
      cat "$1" >&2
      exit 1
    fi
    sleep 0.2
  done
}

if [ -z "${BASE:-}" ]; then
  go generate ./static

  work=$(mktemp -d) pids=()
  trap 'kill "${pids[@]}" 2>/dev/null || true; wait; rm -rf "$work"' EXIT

  if [ -n "${FAKE:-}" ]; then
    go build -o "$work/server" ./e2e/admin/server
    "$node" "$here/fake-server.mjs" --port $((port + 1)) >"$work/fake.log" 2>&1 &
    pids+=($!)
    wait_ready "$work/fake.log" $! "http://127.0.0.1:$((port + 1))/admin/api/me"
    "$work/server" -addr "127.0.0.1:$port" -fake "http://127.0.0.1:$((port + 1))" >"$work/server.log" 2>&1 &
    pids+=($!)
    wait_ready "$work/server.log" $! "http://127.0.0.1:$port/admin/"
  else
    go build -o "$work/asparges" ./cmd/asparges
    "$work/asparges" serve --dev --listen-addr "127.0.0.1:$port" --base-url "http://127.0.0.1:$port" \
      --data-dir "$work/data" --e2e-fixtures "$here/fixtures" >"$work/serve.log" 2>&1 &
    pids+=($!)
    wait_ready "$work/serve.log" $! "http://127.0.0.1:$port/admin/"
    export ASP_REAL=1
  fi

  export BASE=http://127.0.0.1:$port
fi

cd "$here"
OUT=$out "$playwright" test "$@"
