{
  description = "asparges — aspargesgaarden.no: public site rendered in Go, tailnet-only admin";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    flake-checks.url = "github:kradalby/flake-checks";
    flake-checks.inputs.nixpkgs.follows = "nixpkgs";
    flake-checks.inputs.flake-utils.follows = "flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      flake-checks,
    }:
    let
      hashes = builtins.fromJSON (builtins.readFile ./flakehashes.json);
    in
    {
      overlays.default = _final: prev: {
        asparges = self.packages.${prev.stdenv.hostPlatform.system}.default;
      };
      nixosModules.default = import ./nix/module.nix self;
    }
    # eachDefaultSystem still lists x86_64-darwin, which nixpkgs 26.11 dropped:
    # evaluating any output for it throws.
    // flake-utils.lib.eachSystem [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" ] (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        inherit (pkgs) lib;
        fc = flake-checks.lib;

        # The old elm-pages site stays in the tree as the reference for the
        # port and the source for the import, until the cleanup phase removes
        # it. Formatters and linters must leave it exactly as it is.
        legacy = [
          "AGENTS.md"
          "CLAUDE.md"
          "README.md"
          "content"
          "elm-srcs.nix"
          "elm-tooling.json"
          "elm.json"
          "grafikk"
          "nix/images.nix"
          "package.json"
          "plugins"
          "public"
          "registry.dat"
          "src"
          "tailwind.config.js"
          "yarn.lock"
        ];

        common = {
          inherit pkgs;
          root = ./.;
          pname = "asparges";
          version = "0.1.0";
          vendorHash = hashes.vendor.sri;
          goPkg = pkgs.go_latest;
          subPackages = [ "cmd/asparges" ];

          prettier = true;
          # Pulls .elm into the formatting check's source, so elm-format sees
          # the Elm apps once they exist instead of silently checking nothing.
          fmtExts = [ "elm" ];
          fmtExclude = map (p: ./. + "/${p}") legacy;
          treefmtExtra = {
            programs.elm-format.enable = true;
            # prettier reads .editorconfig and walks past the repo root to find
            # one, so a developer's ~/.editorconfig would restyle files the
            # sandboxed check sees differently. Pin it off.
            settings.formatter.prettier.options = [ "--no-editorconfig" ];
            # fmtExclude only trims the check's source; `nix fmt` walks the
            # working tree, so it needs the same paths as treefmt excludes.
            settings.global.excludes = lib.concatMap (p: [
              p
              "${p}/**"
            ]) legacy;
          };
        };

        asparges = (fc.goBuild common).overrideAttrs (_: {
          meta = {
            description = "aspargesgaarden.no web server";
            homepage = "https://github.com/kradalby/aspargesgaarden-elm";
            mainProgram = "asparges";
          };
        });
      in
      {
        packages.default = asparges;
        formatter = fc.formatter common;
        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go_latest
            pkgs.gopls
            pkgs.golangci-lint
            pkgs.gofumpt
            pkgs.govulncheck
            pkgs.sqlc
            (fc.formatter common)
            pkgs.prek
            pkgs.gnumake

            pkgs.elmPackages.elm
            pkgs.elmPackages.elm-format
            pkgs.elmPackages.elm-json
            pkgs.elmPackages.elm-test-rs
            pkgs.elm2nix
            pkgs.nodejs
            pkgs.yarn
            pkgs.esbuild
            pkgs.tailwindcss_4

            pkgs.vips
          ];
          shellHook = ''
            # Never fetch a toolchain: a go.mod ahead of nixpkgs' Go must be a
            # loud error, not a silent download from go.dev outside the store.
            export GOTOOLCHAIN=local
          '';
        };
        checks = {
          build = asparges;
          gotest = fc.goTest (common // { goRace = true; });
          golangci-lint = fc.goLint common;
          formatting = fc.goFormat common;
        }
        # NixOS module evaluation needs a Linux system.
        // lib.optionalAttrs pkgs.stdenv.hostPlatform.isLinux {
          module-eval = import ./nix/module-eval.nix {
            inherit
              pkgs
              self
              nixpkgs
              system
              ;
          };
        };
      }
    );
}
