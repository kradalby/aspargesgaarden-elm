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

        # static/dist is built here and copied in before go:embed sees it, for
        # the binary and for every Go check. Locally `go generate ./static`
        # runs the same build.sh. Photos are their own derivation so a Go edit
        # (which can change the class list) never re-encodes them.
        fs = lib.fileset;
        # pre runs in the copied tree before build.sh, e.g. to install an
        # app's pinned npm packages.
        staticStepWith =
          pre: name: step: inputs: tools:
          pkgs.runCommand "asparges-static-${name}"
            {
              src = fs.toSource {
                root = ./.;
                fileset = fs.unions ([ ./static/build.sh ] ++ inputs);
              };
              nativeBuildInputs = tools;
            }
            ''
              cp -r $src/. . && chmod -R u+w .
              ${pre}
              cd static
              bash build.sh $out ${step}
            '';
        staticStep = staticStepWith "";
        staticCSS =
          staticStep "css" "css fonts"
            (
              [
                ./static/input.css
                ./static/admin-input.css
                ./static/theme.css
                ./static/fonts-src
              ]
              # Tailwind scans Go and Elm sources for class names (input.css
              # @source picks the folders); photos stay cached either way.
              ++ [ (fs.fileFilter (f: f.hasExt "go" || f.hasExt "elm") ./.) ]
            )
            [
              pkgs.tailwindcss_4
              pkgs.woff2
            ];
        staticPhotos =
          staticStep "photos" "photos"
            [ ./static/photos-src ]
            [
              (lib.getBin vips)
              imagesbuild
            ];

        # The Elm apps: elm/<app>, each with its own elm.json and the packages
        # elm2nix pinned next to it (elm-srcs.nix, registry.dat), so apps keep
        # their dependencies apart. Adding an app is adding its name here.
        elmApps = [
          "gallery"
          "admin"
        ];
        elmHome =
          app:
          pkgs.elmPackages.fetchElmDeps {
            elmVersion = "0.19.2";
            elmPackages = import (./elm + "/${app}/elm-srcs.nix");
            registryDat = ./elm + "/${app}/registry.dat";
          };
        # build.sh's <app> step, offline: <app>.js for static/dist. An app
        # with a yarn.lock (its boot.js bundles npm packages) gets them from
        # fetchYarnDeps, pinned by its hash in flakehashes.json.
        elmApp =
          app:
          let
            dir = ./elm + "/${app}";
            yarn = builtins.pathExists (dir + "/yarn.lock");
            yarnDeps = pkgs.fetchYarnDeps {
              yarnLock = dir + "/yarn.lock";
              hash = hashes.yarn.${app};
            };
          in
          (staticStepWith
            (lib.optionalString yarn ''
              export HOME=$TMPDIR
              (cd elm/${app} && yarnOfflineCache=${yarnDeps} yarnConfigHook)
            '')
            "elm-${app}"
            app
            [ dir ]
            (
              [
                pkgs.elmPackages.elm
                pkgs.esbuild
              ]
              ++ lib.optionals yarn [
                pkgs.yarnConfigHook
                pkgs.nodejs
              ]
            )
          ).overrideAttrs
            (old: {
              buildCommand = elmHome app + old.buildCommand;
            });
        elmTest =
          app:
          pkgs.runCommand "asparges-elm-test-${app}"
            {
              src = ./elm + "/${app}";
              nativeBuildInputs = [
                pkgs.elmPackages.elm
                pkgs.elmPackages.elm-test-rs
                pkgs.nodejs # elm-test-rs runs the compiled suite on node
              ];
            }
            ''
              ${elmHome app}
              cp -r $src/. . && chmod -R u+w .
              elm-test-rs --offline --compiler ${pkgs.elmPackages.elm}/bin/elm
              touch $out
            '';

        withStatic =
          drv:
          drv.overrideAttrs (old: {
            postPatch = (old.postPatch or "") + ''
              mkdir -p static/dist
              cp -r ${staticCSS}/. ${staticPhotos}/. ${
                lib.concatMapStringsSep " " (app: "${elmApp app}/.") elmApps
              } static/dist/
              chmod -R u+w static/dist
            '';
          });

        common = {
          inherit pkgs;
          root = ./.;
          pname = "asparges";
          version = "0.1.0";
          vendorHash = hashes.vendor.sri;
          goPkg = pkgs.go_latest;
          subPackages = [ "cmd/asparges" ];
          # db/db.go embeds schema.sql; flake-checks' source filter keeps only
          # .go files, so the schema has to be added by hand. cmd/asparges'
          # tests read the module, to keep every serve flag settable there;
          # ui's read the lightbox's widest photo.
          extraSrc = [
            ./db/schema.sql
            ./nix/module.nix
            ./elm/gallery/src/Lightbox.elm
          ];
          # Committed embedded files; static/dist comes from withStatic.
          embedDirs = [ ./static/files ];

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
            # Golden files are byte-exact renderer output, one tag per line.
            settings.global.excludes =
              lib.concatMap (p: [
                p
                "${p}/**"
              ]) legacy
              ++ [ "**/testdata/**" ];
          };
        };

        asparges = (withStatic goBuild).overrideAttrs (_: {
          meta = {
            description = "aspargesgaarden.no web server";
            homepage = "https://github.com/kradalby/aspargesgaarden-elm";
            mainProgram = "asparges";
          };
        });

        # images: the slim vips the service shells out to, and imagesbuild,
        # which runs the images pipeline in staticPhotos. It is its own binary
        # because asparges embeds its output, so it starts from the plain Go
        # build (no withStatic), reuses that vendor tree and sees only images/:
        # unrelated Go edits do not re-encode the photos.
        vips = pkgs.callPackage ./nix/vips.nix { };
        goBuild = fc.goBuild common;
        imagesbuild = goBuild.overrideAttrs (_: {
          pname = "asparges-imagesbuild";
          src = lib.fileset.toSource {
            root = ./.;
            fileset = lib.fileset.unions [
              ./go.mod
              ./go.sum
              ./cmd/imagesbuild
              (lib.fileset.fileFilter (f: f.hasExt "go" && !lib.hasSuffix "_test.go" f.name) ./images)
            ];
          };
          subPackages = [ "cmd/imagesbuild" ];
          inherit (goBuild) goModules;
          meta.mainProgram = "imagesbuild";
        });
      in
      {
        packages.default = asparges;
        packages.vips = vips;
        packages.imagesbuild = imagesbuild;
        packages.photo-variants = staticPhotos;
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
            pkgs.woff2

            (lib.getBin vips)
          ];
          shellHook = ''
            # Never fetch a toolchain: a go.mod ahead of nixpkgs' Go must be a
            # loud error, not a silent download from go.dev outside the store.
            export GOTOOLCHAIN=local
          '';
        };
        checks = {
          build = asparges;
          gotest = withStatic (
            fc.goTest (
              common
              // {
                goRace = true;
                # The images tests run the real vips on testdata/.
                nativeCheckInputs = [ (lib.getBin vips) ];
              }
            )
          );
          photo-variants = staticPhotos;
          golangci-lint = withStatic (fc.goLint common);
          formatting = fc.goFormat common;
          # db/dbsqlc is sqlc output with no //go:generate directive, so the
          # drift check runs sqlc itself over the config, schema and queries.
          generate = fc.goGenerate (
            common
            // {
              extraSrc = common.extraSrc ++ [
                ./sqlc.yaml
                ./db/queries
              ];
              nativeCheckInputs = [ pkgs.sqlc ];
              generateCommand = "sqlc generate";
            }
          );
        }
        // lib.listToAttrs (map (app: lib.nameValuePair "elm-test-${app}" (elmTest app)) elmApps)
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
