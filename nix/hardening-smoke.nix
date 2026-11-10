# The service's own sandbox on a real kernel: import, then serve --dev building
# every variant, under systemd-run with the module's hardening, so a syscall,
# W^X mapping or decoder path that only one architecture takes fails here and
# not on deploy. It needs root and a running systemd, so it is a program CI
# runs on the runner, not a check inside the nix sandbox.
{
  pkgs,
  self,
  nixpkgs,
  system,
}:
let
  inherit (pkgs) lib;

  svc =
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
          system.stateVersion = lib.trivial.release;
          services.asparges.enable = true;
        }
      ];
    }).config.systemd.services.asparges;

  # What makes a run a run; the rest of serviceConfig is the sandbox under
  # test. DynamicUser stands in for the module's static user.
  runOwn = [
    "ExecStart"
    "User"
    "Group"
    "StateDirectory"
    "Restart"
    "RestartSec"
  ];
  value =
    v:
    if lib.isList v then
      lib.concatStringsSep " " v
    else if lib.isBool v then
      lib.boolToString v
    else
      toString v;
  props = lib.escapeShellArgs (
    lib.mapAttrsToList (k: v: "--property=${k}=${value v}") (removeAttrs svc.serviceConfig runOwn)
    ++ [
      "--property=DynamicUser=yes"
      "--property=StateDirectory=asparges-smoke"
      "--setenv=PATH=${svc.environment.PATH}"
    ]
  );

  # A minimal old-site checkout: an event and a gallery whose photos cover
  # the decoders an upload reaches (HEIC, Display-P3 JPEG, EXIF rotation).
  fixtures = ../images/testdata;
  tree = pkgs.runCommand "asparges-smoke-tree" { } ''
    mkdir -p $out/content/events $out/public/smoke
    cp ${fixtures}/upright.jpg $out/public/smoke/upright.jpeg
    cp ${fixtures}/rotated.heic $out/public/smoke/heic.jpeg
    cp ${fixtures}/p3.jpg $out/public/smoke/p3.jpeg
    cp ${fixtures}/orient6.jpg $out/public/smoke/orient6.jpeg
    cat > $out/content/events/smoke.md <<'EOF'
    ---
    { "title": "Smoke", "eventDate": "2030-01-01", "publishedFrom": "2029-01-01",
      "photo": { "path": "smoke/upright", "photographer": { "name": "", "website": "" }, "description": "" } }
    ---
    Smoke.
    EOF
    cat > $out/content/gallery.yaml <<'EOF'
    photographers: {}
    photos:
      - { path: smoke/heic, photographer: "", description: "" }
      - { path: smoke/p3, photographer: "", description: "" }
      - { path: smoke/orient6, photographer: "", description: "" }
    EOF
  '';

  asparges = lib.getExe self.packages.${system}.default;
  data = "/var/lib/asparges-smoke";
in
pkgs.writeShellApplication {
  name = "asparges-hardening-smoke";
  runtimeInputs = [
    pkgs.curl
    pkgs.gnugrep
  ];
  # systemd-run is the host's: it has to talk to the running manager.
  text = ''
    systemctl stop asparges-smoke.service 2>/dev/null || true
    rm -rf /var/lib/private/asparges-smoke /var/lib/asparges-smoke

    # Not --quiet: its summary carries the run's time and memory peak.
    run() { systemd-run --wait --pipe --collect ${props} -- "$@"; }
    run ${asparges} import --data-dir=${data} ${tree}

    systemd-run --unit=asparges-smoke --collect --quiet ${props} -- \
      ${asparges} serve --dev --data-dir=${data} --listen-addr=127.0.0.1:8099
    trap 'systemctl stop asparges-smoke.service; journalctl -u asparges-smoke.service --no-pager | tail -n 50' EXIT
    curl -fsS --retry 30 --retry-delay 1 --retry-all-errors -o /dev/null http://127.0.0.1:8099/
    # serve builds every size at start; each gallery picture answers once built.
    for u in $(curl -fsS http://127.0.0.1:8099/galleri | grep -o '/img/[^" ,]*' | sort -u); do
      curl -fsS --retry 120 --retry-delay 1 --retry-all-errors -o /dev/null "http://127.0.0.1:8099$u"
    done
    systemctl is-active --quiet asparges-smoke.service
    echo "hardening smoke passed"
  '';
}
