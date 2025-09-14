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

          # Check for required tools
          for tool in convert fd optimizt; do
              if ! command -v $tool &> /dev/null; then
                  echo "Error: $tool is not installed or not in PATH"
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

          # Function to check if any JPEG file needs WebP/AVIF conversion
          needs_conversion() {
              local jpeg_file="$1"
              local base_name="''${jpeg_file%.*}"
              local webp_file="''${base_name}.webp"
              local avif_file="''${base_name}.avif"

              if [ ! -f "$webp_file" ] || [ ! -f "$avif_file" ] || \
                 [ "$jpeg_file" -nt "$webp_file" ] || [ "$jpeg_file" -nt "$avif_file" ]; then
                  return 0  # needs conversion
              fi

              return 1  # no conversion needed
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
          done < <(${pkgs.fd}/bin/fd -e jpeg -e jpg . "$OUTPUT_DIR"/ -0)

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
                          ${pkgs.imagemagick}/bin/convert "$img" -resize "''${width}x>" "$resize_file"
                      fi
                  done
              done
          else
              echo "All resize versions are up to date"
          fi

          # Now check ALL JPEG files (original + resized) for WebP/AVIF conversion
          echo "Checking which images need WebP/AVIF conversion..."
          images_to_convert=()

          while IFS= read -r -d $'\0' jpeg_file; do
              if needs_conversion "$jpeg_file"; then
                  images_to_convert+=("$jpeg_file")
                  if [ "$VERBOSE" = true ]; then
                      echo "Will convert: $jpeg_file"
                  fi
              else
                  if [ "$VERBOSE" = true ]; then
                      echo "Skipping conversion (up to date): $jpeg_file"
                  fi
              fi
          done < <(${pkgs.fd}/bin/fd -e jpeg -e jpg . "$OUTPUT_DIR"/ -0)

          if [ ''${#images_to_convert[@]} -gt 0 ]; then
              echo "Converting ''${#images_to_convert[@]} images to WebP and AVIF..."
              export HOME=''${HOME:-$TMPDIR}

              # Set LD_LIBRARY_PATH if vips is available in Nix environment
              if [ -n "''${VIPS_LIB_PATH:-}" ]; then
                  export LD_LIBRARY_PATH="''${VIPS_LIB_PATH}:''${LD_LIBRARY_PATH:-}"
              fi

              # Process each image individually
              for jpeg_file in "''${images_to_convert[@]}"; do
                  if [ "$VERBOSE" = true ]; then
                      echo "Converting $jpeg_file to WebP and AVIF"
                  fi
                  ${yarnPkgs}/bin/optimizt --avif --webp "$jpeg_file"
              done
          else
              echo "All WebP and AVIF versions are up to date"
          fi

          echo "Image processing complete"
        '';

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
            processImagesScript
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

            # Set environment variables for the script
            export HOME=$TMPDIR
            export VIPS_LIB_PATH="${pkgs.vips}/lib:${pkgs.glib}/lib"

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
      devShell = pkgs.mkShell {buildInputs = devDeps;};
      packages = with pkgs; {
        inherit aspargesgaarden processedImages processImagesScript;
      };
      defaultPackage = pkgs.aspargesgaarden;
    });
}
