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
          # Only include yarn-related files to avoid unnecessary rebuilds
          src = pkgs.lib.cleanSourceWith {
            src = ./.;
            filter = path: type: let
              baseName = baseNameOf path;
            in
              baseName
              == "package.json"
              || baseName == "yarn.lock"
              || baseName == "elm-tooling.json"
              || baseName == ".yarnrc";
          };
          publishBinsFor = [
            "elm-review"
            "elm-pages"
            "elm-json"
            "elm-optimize-level-2"
            "@343dev/optimizt"
          ];

          # Add vips dependencies for sharp to build properly
          buildInputs = with pkgs; [
            vips
            glib
            pkg-config
            python3
          ];

          # Set environment for sharp to use system vips
          preBuild = ''
            export PKG_CONFIG_PATH="${pkgs.vips.dev}/lib/pkgconfig:${pkgs.glib.dev}/lib/pkgconfig"
            export SHARP_FORCE_GLOBAL_LIBVIPS=1
          '';
        };

        # Separate derivation for processed images
        processedImages = pkgs.stdenv.mkDerivation {
          name = "aspargesgaarden-images";
          version = aspargesgaardenVersion;

          # Only include JPEG files from public directory to avoid unnecessary rebuilds
          src = pkgs.lib.cleanSourceWith {
            src = ./.;
            filter = path: type: let
              relPath = pkgs.lib.removePrefix (toString ./. + "/") (toString path);
              isPublicJpeg =
                pkgs.lib.hasPrefix "public/" relPath
                && (pkgs.lib.hasSuffix ".jpeg" relPath || pkgs.lib.hasSuffix ".jpg" relPath);
              isPublicDir =
                type
                == "directory"
                && (relPath == "public" || pkgs.lib.hasPrefix "public/" relPath);
            in
              isPublicJpeg || isPublicDir;
          };

          buildInputs = with pkgs; [
            yarnPkgs
            imagemagick
            fd
            pkg-config
            python3
          ];

          nativeBuildInputs = with pkgs; [
            pkg-config
            python3
          ];

          dontBuild = true;

          installPhase = ''
            mkdir -p $out
            cp -r public $out/

            # Resize images for different viewport widths
            echo "Resizing images..."
            for width in 320 640 768 1024 1280 1536 2048; do
              echo "Creating ''${width}w versions..."
              fd -e jpeg -e jpg . $out/public/ -x convert {} -resize "''${width}x>" {.}"_''${width}w_resize.jpeg"
            done

            # Use optimizt for WebP and AVIF conversion
            echo "Converting to WebP and AVIF with optimizt..."
            export HOME=$TMPDIR
            export LD_LIBRARY_PATH="${pkgs.vips}/lib:${pkgs.glib}/lib:$LD_LIBRARY_PATH"

            optimizt --avif --webp $out/public/

            echo "Image processing complete"
          '';
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

            # Copy processed images over the original public directory
            # This preserves CSS and other static files while updating images
            chmod -R +w public
            cp -r ${processedImages}/public/* public/

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
          elm-analyse
          elm2nix
          # lamdera
        ]);
    in {
      # `nix develop`
      devShell = pkgs.mkShell {buildInputs = devDeps;};
      packages = with pkgs; {
        inherit aspargesgaarden processedImages;
      };
      defaultPackage = pkgs.aspargesgaarden;
    });
}
