# Eval-time smoke test for the NixOS module: assemble a minimal system and
# assert the rendered unit, so a broken option or service definition fails
# `nix flake check` without spinning up a VM.
{
  pkgs,
  self,
  nixpkgs,
  system,
}:
let
  inherit (pkgs) lib;

  configFor =
    cfg:
    (import (nixpkgs + "/nixos/lib/eval-config.nix") {
      inherit system;
      modules = [
        self.nixosModules.default
        {
          boot.loader.grub.enable = false;
          fileSystems."/" = {
            device = "/dev/sda1";
            fsType = "ext4";
          };
          # Tracks the pinned nixpkgs so the module is checked against the
          # defaults a fresh install would get, not a frozen old release.
          system.stateVersion = lib.trivial.release;
          services.asparges = cfg;
        }
      ];
    }).config;
  unitFor = cfg: (configFor cfg).systemd.services.asparges.serviceConfig;
  failedAssertions =
    cfg: map (a: a.message) (lib.filter (a: !a.assertion) (configFor cfg).assertions);

  default = unitFor { enable = true; };

  custom = unitFor {
    enable = true;
    dataDir = "/var/lib/asparges-beta";
    hostname = "asparges-beta";
    listenAddr = "127.0.0.1:9000";
    baseURL = "https://beta.aspargesgaarden.no";
    noindex = true;
    umamiSiteId = "11111111-2222-4333-8444-555555555555";
    logLevel = "debug";
    tailscaleKeyPath = "/run/agenix/asparges-tskey";
    environmentFile = "/run/agenix/asparges-env";
  };

  check = cond: msg: if cond then true else throw "module-eval: ${msg}";
in
assert check (
  lib.hasInfix "/bin/asparges serve " default.ExecStart
  && lib.hasInfix "--data-dir=/var/lib/asparges'" default.ExecStart
) "default ExecStart missing serve --data-dir: ${default.ExecStart}";
assert check (
  default.StateDirectory == "asparges"
) "default StateDirectory is ${default.StateDirectory}";
assert check (
  !lib.hasInfix "--tailscale-auth-key-path" default.ExecStart && !(default ? LoadCredential)
) "default must not wire an auth key credential: ${default.ExecStart}";
assert check (
  !lib.hasInfix "--noindex" default.ExecStart
) "the default must be indexable: ${default.ExecStart}";
assert check (lib.hasInfix "--log-level=info" default.ExecStart)
  "default ExecStart missing log-level: ${default.ExecStart}";
assert check (lib.hasInfix "--log-level=debug" custom.ExecStart)
  "custom ExecStart missing log-level: ${custom.ExecStart}";
assert check (
  !lib.hasInfix "--dev" default.ExecStart && !lib.hasInfix "--e2e-fixtures" default.ExecStart
) "the module must never reach the dev-only switches: ${default.ExecStart}";
assert check (
  !lib.hasInfix "--umami-site-id" default.ExecStart
) "the default must load no tracker: ${default.ExecStart}";
assert check (lib.hasInfix "--umami-site-id=11111111-2222-4333-8444-555555555555" custom.ExecStart)
  "custom ExecStart missing umami-site-id: ${custom.ExecStart}";
assert check (lib.hasInfix "--noindex" custom.ExecStart)
  "custom ExecStart missing noindex: ${custom.ExecStart}";
assert check (lib.hasInfix "--hostname=asparges-beta" custom.ExecStart)
  "custom ExecStart missing hostname: ${custom.ExecStart}";
assert check (lib.hasInfix "--listen-addr=127.0.0.1:9000" custom.ExecStart)
  "custom ExecStart missing listen-addr: ${custom.ExecStart}";
assert check (lib.hasInfix "--base-url=https://beta.aspargesgaarden.no" custom.ExecStart)
  "custom ExecStart missing base-url: ${custom.ExecStart}";
assert check (lib.hasInfix "--tailscale-auth-key-path=%d/tskey" custom.ExecStart)
  "custom ExecStart missing auth key path: ${custom.ExecStart}";
assert check (
  custom.LoadCredential == "tskey:/run/agenix/asparges-tskey"
) "custom LoadCredential is ${custom.LoadCredential}";
assert check (
  custom.StateDirectory == "asparges-beta"
) "custom StateDirectory is ${custom.StateDirectory}";
assert check (
  custom.EnvironmentFile == "/run/agenix/asparges-env"
) "custom EnvironmentFile not set";
assert check (default.MemoryMax == "2G") "default MemoryMax is ${default.MemoryMax}";
assert check (
  failedAssertions {
    enable = true;
    dataDir = "/srv/asparges";
  } != [ ]
) "a dataDir outside /var/lib must fail an assertion";
pkgs.runCommand "asparges-module-eval-ok" { } "touch $out"
