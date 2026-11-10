#!/usr/bin/env bash
# Builds the admin front end and runs the browser checks in Chromium from the
# flake's nixpkgs. Run from anywhere; arguments go to `playwright test`.
#
# @playwright/test in package.json must equal nixpkgs' playwright-driver
# version, or Playwright looks for browsers it was not given.
set -euo pipefail
cd "$(dirname "$0")"

# ASP_SKIP_BUILD=1 reruns the checks against the last build.
if [[ -z ${ASP_SKIP_BUILD:-} ]]; then
  nix develop .. -c ./build.sh
  nix develop .. -c yarn install --frozen-lockfile --silent
fi

PLAYWRIGHT_BROWSERS_PATH=$(nix build --inputs-from .. --no-link --print-out-paths nixpkgs#playwright-driver.browsers)
export PLAYWRIGHT_BROWSERS_PATH
export PLAYWRIGHT_SKIP_VALIDATE_HOST_REQUIREMENTS=true

# The dev shell's LD_LIBRARY_PATH carries libraries built against a newer
# glibc than nixpkgs' browsers, which then fail to start; run outside it.
# The fake server is started by Playwright, so node must be on PATH too.
PATH=$(nix build --inputs-from .. --no-link --print-out-paths nixpkgs#nodejs)/bin:$PATH
exec env -u LD_LIBRARY_PATH node node_modules/@playwright/test/cli.js test "$@"
