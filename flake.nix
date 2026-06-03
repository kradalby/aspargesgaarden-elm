{
  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";

    # Image toolchain (imagemagick, fd, optimizt via yarnPkgs) is pinned to a
    # fixed rev so routine `nix flake update nixpkgs` does not reprocess every
    # image. Bump deliberately with `nix flake update nixpkgs-img`.
    nixpkgs-img.url = "github:NixOS/nixpkgs/4df1b885d76a54e1aa1a318f8d16fd6005b6401f";
  };

  outputs = {
    self,
    nixpkgs,
    nixpkgs-img,
    flake-utils,
    ...
  }: let
    aspargesgaardenVersion =
      if (self ? shortRev)
      then self.shortRev
      else "dev";
  in
    {
      overlays.default = _: prev: let
        pkgs = nixpkgs.legacyPackages.${prev.stdenv.hostPlatform.system};
        # Pinned toolchain for the expensive image pipeline (see input note).
        imgPkgs = nixpkgs-img.legacyPackages.${prev.stdenv.hostPlatform.system};

        # node_modules depends only on the dependency graph, not on the dev
        # scripts in package.json. Feed yarnPkgs a normalised, deps-only
        # package.json built with builtins.toFile (content-addressed: same
        # content -> same store path). Editing the `scripts` field then leaves
        # yarnPkgs — and therefore processedImages — untouched, so changing a
        # dev script never reprocesses every image.
        depsPackageJson = let
          p = builtins.fromJSON (builtins.readFile ./package.json);
        in
          builtins.toFile "package.json" (builtins.toJSON ({
              inherit (p) name version dependencies devDependencies;
            }
            // pkgs.lib.optionalAttrs (p ? optionalDependencies) {inherit (p) optionalDependencies;}
            // pkgs.lib.optionalAttrs (p ? resolutions) {inherit (p) resolutions;}));
        yarnSrc = imgPkgs.runCommand "yarn-src" {} ''
          mkdir -p $out
          cp ${depsPackageJson} $out/package.json
          cp ${./yarn.lock} $out/yarn.lock
        '';
      in rec {
        processImagesScript = pkgs.writeShellScriptBin "process-images" ''
          set -euo pipefail

          # Default values
          INPUT_DIR="public"
          OUTPUT_DIR=""
          WIDTHS=(320 640 768 1024 1280 1536 2048)
          VERBOSE=false

          usage() {
              echo "Usage: process-images [-i input_dir] [-o output_dir] [-w \"width1 width2 ...\"] [-v]"
              echo "  -i: Input directory (default: public)"
              echo "  -o: Output directory (default: same as input)"
              echo "  -w: Space-separated list of widths (default: 320 640 768 1024 1280 1536 2048)"
              echo "  -v: Verbose output"
              echo "  -h: Show this help message"
              exit 1
          }

          # Parse command line arguments
          while getopts "i:o:w:vh" opt; do
              case $opt in
                  i)
                      INPUT_DIR="$OPTARG"
                      ;;
                  o)
                      OUTPUT_DIR="$OPTARG"
                      ;;
                  w)
                      IFS=' ' read -ra WIDTHS <<< "$OPTARG"
                      ;;
                  v)
                      VERBOSE=true
                      ;;
                  h)
                      usage
                      ;;
                  \?)
                      echo "Invalid option: -$OPTARG" >&2
                      usage
                      ;;
              esac
          done

          # Set output directory to input directory if not specified
          if [ -z "$OUTPUT_DIR" ]; then
              OUTPUT_DIR="$INPUT_DIR"
          fi

          # Check if input directory exists
          if [ ! -d "$INPUT_DIR" ]; then
              echo "Error: Input directory '$INPUT_DIR' does not exist"
              exit 1
          fi

          # Verify Nix store tool paths exist
          for tool in "${imgPkgs.imagemagick}/bin/convert" "${imgPkgs.fd}/bin/fd" "${yarnPkgs}/bin/optimizt"; do
              if [ ! -x "$tool" ]; then
                  echo "Error: $tool does not exist or is not executable"
                  exit 1
              fi
          done

          echo "Processing images in '$INPUT_DIR'..."
          echo "Output directory: '$OUTPUT_DIR'"
          echo "Widths: ''${WIDTHS[*]}"

          # Create output directory if it doesn't exist
          mkdir -p "$OUTPUT_DIR"

          # Copy input to output if they're different
          if [ "$INPUT_DIR" != "$OUTPUT_DIR" ]; then
              echo "Copying files from '$INPUT_DIR' to '$OUTPUT_DIR'..."
              cp -r "$INPUT_DIR"/* "$OUTPUT_DIR"/
          fi

          # Function to check if resize files need processing
          needs_resize_processing() {
              local source_file="$1"
              local base_name="''${source_file%.*}"

              # Check if source file is newer than any of the generated resize files
              for width in "''${WIDTHS[@]}"; do
                  local resize_file="''${base_name}_''${width}w_resize.jpeg"
                  if [ ! -f "$resize_file" ] || [ "$source_file" -nt "$resize_file" ]; then
                      return 0  # needs processing
                  fi
              done

              return 1  # no processing needed
          }

          # Function to determine which conversion formats are needed
          # Returns space-separated flags for optimizt (e.g. "--webp --avif" or "--webp")
          needed_conversion_flags() {
              local jpeg_file="$1"
              local base_name="''${jpeg_file%.*}"
              local flags=""

              local webp_file="''${base_name}.webp"
              if [ ! -f "$webp_file" ] || [ "$jpeg_file" -nt "$webp_file" ]; then
                  flags="--webp"
              fi

              local avif_file="''${base_name}.avif"
              if [ ! -f "$avif_file" ] || [ "$jpeg_file" -nt "$avif_file" ]; then
                  flags="$flags --avif"
              fi

              echo "$flags"
          }

          # Resize images for different viewport widths
          echo "Checking which images need resizing..."
          images_to_resize=()

          while IFS= read -r -d $'\0' img; do
              if needs_resize_processing "$img"; then
                  images_to_resize+=("$img")
                  if [ "$VERBOSE" = true ]; then
                      echo "Will resize: $img"
                  fi
              else
                  if [ "$VERBOSE" = true ]; then
                      echo "Skipping resize (up to date): $img"
                  fi
              fi
          done < <(${imgPkgs.fd}/bin/fd -e jpeg -e jpg . "$OUTPUT_DIR"/ -0)

          if [ ''${#images_to_resize[@]} -gt 0 ]; then
              echo "Resizing ''${#images_to_resize[@]} images..."
              for img in "''${images_to_resize[@]}"; do
                  base_name="''${img%.*}"
                  for width in "''${WIDTHS[@]}"; do
                      resize_file="''${base_name}_''${width}w_resize.jpeg"
                      if [ ! -f "$resize_file" ] || [ "$img" -nt "$resize_file" ]; then
                          if [ "$VERBOSE" = true ]; then
                              echo "Creating ''${width}w version of $img"
                          fi
                          ${imgPkgs.imagemagick}/bin/convert "$img" -resize "''${width}x>" "$resize_file"
                      fi
                  done
              done
          else
              echo "All resize versions are up to date"
          fi

          # Now check ALL JPEG files (original + resized) for WebP/AVIF conversion
          echo "Checking which images need WebP/AVIF conversion..."

          # Set library path early so optimizt can find vips
          export HOME=''${HOME:-$TMPDIR}
          if [ -n "''${VIPS_LIB_PATH:-}" ]; then
              export LD_LIBRARY_PATH="''${VIPS_LIB_PATH}:''${LD_LIBRARY_PATH:-}"
              export DYLD_LIBRARY_PATH="''${VIPS_LIB_PATH}:''${DYLD_LIBRARY_PATH:-}"
          fi

          convert_count=0
          while IFS= read -r -d $'\0' jpeg_file; do
              flags=$(needed_conversion_flags "$jpeg_file")
              if [ -n "$flags" ]; then
                  convert_count=$((convert_count + 1))
                  if [ "$VERBOSE" = true ]; then
                      echo "Converting $jpeg_file ($flags)"
                  fi
                  ${yarnPkgs}/bin/optimizt $flags "$jpeg_file"
              else
                  if [ "$VERBOSE" = true ]; then
                      echo "Skipping conversion (up to date): $jpeg_file"
                  fi
              fi
          done < <(${imgPkgs.fd}/bin/fd --no-ignore -e jpeg -e jpg . "$OUTPUT_DIR"/ -0)

          if [ "$convert_count" -eq 0 ]; then
              echo "All WebP and AVIF versions are up to date"
          else
              echo "Converted $convert_count images"
          fi

          echo "Image processing complete"
        '';

        # Offline dependency mirror — fixed-output, only refetches when
        # yarn.lock changes, so it caches independently of the node_modules build.
        yarnOfflineCache = imgPkgs.fetchYarnDeps {
          yarnLock = ./yarn.lock;
          hash = "sha256-FN2sCxXNy1EoHMjCDOD1pCQrlpV3Nxfvld/ranP26x0=";
        };

        # node_modules + published CLIs, built from the offline mirror with the
        # modern yarn-v1 hooks (yarn2nix/mkYarnPackage was removed from nixpkgs).
        # sharp (via @343dev/optimizt) ships prebuilt @img/* binaries, so no
        # native build / system vips is required here.
        yarnPkgs = imgPkgs.stdenv.mkDerivation {
          name = "yarnPkgs";
          version = aspargesgaardenVersion;
          # Deps-only source (see depsPackageJson) so dev-script edits don't
          # rebuild node_modules or reprocess images.
          src = yarnSrc;

          inherit yarnOfflineCache;

          nativeBuildInputs = with imgPkgs; [
            yarnConfigHook # populates ./node_modules from yarnOfflineCache
            nodejs
            makeWrapper
          ];

          # We only need node_modules (incl. devDeps) + the CLIs; no JS build,
          # no production prune — so we skip yarnBuildHook / yarnInstallHook.
          dontBuild = true;

          installPhase = ''
            runHook preInstall

            mkdir -p $out
            cp -R node_modules $out/node_modules

            mkdir -p $out/bin
            for b in elm-review elm-pages elm-json elm-optimize-level-2 optimizt; do
              target=$(readlink -f "$out/node_modules/.bin/$b")
              makeWrapper ${imgPkgs.nodejs}/bin/node "$out/bin/$b" --add-flags "$target"
            done

            runHook postInstall
          '';
        };

        # Separate derivation for processed images
        processedImages = imgPkgs.stdenv.mkDerivation {
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

          buildInputs = [
            processImagesScript
            yarnPkgs
            imgPkgs.imagemagick
            imgPkgs.fd
            imgPkgs.pkg-config
            imgPkgs.python3
          ];

          nativeBuildInputs = [
            imgPkgs.pkg-config
            imgPkgs.python3
          ];

          dontBuild = true;

          installPhase = ''
            mkdir -p $out
            cp -r public $out/

            # Set environment variables for the script
            export HOME=$TMPDIR
            export VIPS_LIB_PATH="${imgPkgs.vips}/lib:${imgPkgs.glib}/lib"

            # Run the image processing script
            process-images -i $out/public -o $out/public
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
        overlays = [self.overlays.default];
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
          vips
          glib
          processImagesScript
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
      devShells.default = pkgs.mkShell {
        buildInputs = devDeps;
        shellHook = ''
          export VIPS_LIB_PATH="${pkgs.vips}/lib:${pkgs.glib}/lib"
          ln -fs ${pkgs.yarnPkgs}/node_modules .
        '';
      };
      packages = with pkgs; {
        default = aspargesgaarden;
        inherit aspargesgaarden processedImages processImagesScript;
      };
    });
}
