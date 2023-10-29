{
  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    ...
  }: let
    aspargesgaardenVersion =
      if (self ? shortRev)
      then self.shortRev
      else "dev";
  in
    {
      overlay = _: prev: let
        pkgs = import nixpkgs {
          config = {allowUnfree = true;};
          inherit (prev) system;
        };
      in rec {
        # yarnPkgs = pkgs.yarn2nix-moretea.mkYarnPackage {
        #   name = "yarnPkgs";
        #   version = aspargesgaardenVersion;
        #   src =
        #     pkgs.nix-gitignore.gitignoreSource [
        #       "Makefile"
        #       "flake.*"
        #       "app/"
        #       "plugins/"
        #       "public/"
        #       "gen/"
        #       "grafikk/"
        #     ]
        #     ./.;
        #   publishBinsFor = [
        #     "elm-review"
        #     "elm-pages"
        #     "elm-optimize-level-2"
        #   ];
        # };

        aspargesgaarden = pkgs.stdenv.mkDerivation {
          name = "aspargesgaarden";
          src = pkgs.nix-gitignore.gitignoreSource ["Makefile" "flake.*"] ./.;

          buildInputs = with pkgs; [
            # yarnPkgs

            elmPackages.elm-pages
            # elmPackages.elm
            elmPackages.lamdera
          ];

          postUnpack = ''
            export HOME="$TMP"
          '';

          # patchPhase = ''
          #   rm -rf elm-stuff
          #   ln -sf ${yarnPkgs}/node_modules .
          # '';

          configurePhase = pkgs.elmPackages.fetchElmDeps {
            elmVersion = "0.19.1";
            elmPackages = import ./elm-srcs.nix;
            registryDat = ./registry.dat;
          };

          dontBuild = true;

          installPhase = ''
            mkdir -p $out
            cp -r $src/* $out/.
            cd $out

            elm-pages build
          '';
        };
      };
    }
    // flake-utils.lib.eachDefaultSystem
    (system: let
      pkgs = import nixpkgs {
        overlays = [self.overlay];
        config = {allowUnfree = true;};
        inherit system;
      };
      buildDeps = with pkgs; [
        # yarnPkgs

        nodejs
        yarn
        git
        gnumake
      ];
      devDeps = with pkgs;
        buildDeps
        ++ [
          pkg-config
          libpng
          imagemagick
          fd
        ]
        ++ (with elmPackages; [
          elm
          elm-format
          elm-json
          elm-analyse
          elm-pages
          elm2nix
        ]);
    in {
      # `nix develop`
      devShell = pkgs.mkShell {buildInputs = devDeps;};
      packages = with pkgs; {
        inherit aspargesgaarden;
      };
      defaultPackage = pkgs.aspargesgaarden;
    });
}
