# Python with Playwright, and the browsers that Playwright version drives,
# from the nixpkgs the flake pins (no pip, no npm, no browser download).
let
  lock = builtins.fromJSON (builtins.readFile ../../flake.lock);
  pkgs = import (builtins.fetchTree lock.nodes.${lock.nodes.root.inputs.nixpkgs}.locked) { };
in
{
  python = pkgs.python3.withPackages (ps: [
    ps.playwright
    ps.pytest
  ]);
  browsers = pkgs.playwright-driver.browsers;
  # WebKit needs an EGL display even headless; a host without GL drivers
  # (no /run/opengl-driver) gets Mesa's software one.
  mesa = pkgs.mesa;
}
