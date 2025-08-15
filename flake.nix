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
        pkgs = nixpkgs.legacyPackages.${prev.system};
      in rec {
        yarnPkgs = pkgs.yarn2nix-moretea.mkYarnPackage {
          name = "yarnPkgs";
          version = aspargesgaardenVersion;
          src = pkgs.nix-gitignore.gitignoreSource [] ./.;
          publishBinsFor = [
            "elm-review"
            "elm-pages"
            "elm-json"
            "elm-optimize-level-2"
          ];
        };

        aspargesgaarden = pkgs.stdenv.mkDerivation {
          name = "aspargesgaarden";
          src = pkgs.nix-gitignore.gitignoreSource ["Makefile"] ./.;

          buildInputs = with pkgs; [
            yarnPkgs

            elmPackages.elm
            yarn
            nodejs
            nodePackages.sass
            nodePackages.parcel
          ];

          postUnpack = ''
            export HOME="$TMP"
          '';

          patchPhase = ''
            rm -rf elm-stuff
            ln -sf ${yarnPkgs}/node_modules .
          '';

          shellHook = ''
            ln -fs ${yarnPkgs}/node_modules .
          '';

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
            
            # Make plugins directory and its contents writable so we can generate CurrentDate.elm
            chmod -R +w plugins
            
            # Generate CurrentDate.elm with the actual build date
            BUILD_DATE=$(date -u +"%Y-%m-%d")
            cat > plugins/CurrentDate.elm <<EOF
            module CurrentDate exposing (currentDate)

            import DataSource exposing (DataSource)

            currentDate : DataSource String
            currentDate =
                DataSource.succeed "$BUILD_DATE"
            EOF
            
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
        yarnPkgs

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
          elm2nix
          # lamdera
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
