# Playwright's test runner with the browsers that version drives, from the
# nixpkgs the flake pins (no npm install, no browser download).
let
  lock = builtins.fromJSON (builtins.readFile ../../flake.lock);
  pkgs = import (builtins.fetchTree lock.nodes.${lock.nodes.root.inputs.nixpkgs}.locked) { };
in
{
  playwright = pkgs.playwright-test;
  # WebKit needs an EGL display even headless; a host without GL drivers
  # (no /run/opengl-driver) gets Mesa's software one.
  mesa = pkgs.mesa;
}
