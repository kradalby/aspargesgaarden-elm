{ pkgs, processImagesScript, publicDir }:
let
  inherit (pkgs) lib;
  sources = lib.filter
    (path:
      let
        name = builtins.baseNameOf path;
      in
      (lib.hasSuffix ".jpeg" name || lib.hasSuffix ".jpg" name)
      && builtins.match ".*_[0-9]+w_resize\\.jpeg" name == null)
    (lib.filesystem.listFilesRecursive publicDir);

  imageVariants = builtins.listToAttrs (map
    (path:
      let
        relativePath = lib.removePrefix (toString publicDir + "/") (toString path);
        name = builtins.baseNameOf path;
      in
      {
        name = relativePath;
        value = pkgs.runCommand "aspargesgaarden-image-${lib.strings.sanitizeDerivationName name}"
          {
            # Copy just this file into the store so the flake's source path
            # cannot invalidate every image when unrelated files change.
            source = builtins.path {
              inherit path;
              name = lib.strings.sanitizeDerivationName name;
            };
            VIPS_LIB_PATH = "${pkgs.vips}/lib:${pkgs.glib}/lib";
          }
          ''
            imageDir="$out/public/"${lib.escapeShellArg (builtins.dirOf relativePath)}
            mkdir -p "$imageDir"
            cp "$source" "$imageDir/"${lib.escapeShellArg name}
            ${processImagesScript}/bin/process-images -i "$imageDir"
          '';
      })
    sources);
in
pkgs.runCommand "aspargesgaarden-images" { passthru = { inherit imageVariants; }; } ''
  mkdir -p "$out/public"
  # Keep directories writable so images sharing a directory can merge.
  ${lib.concatMapStringsSep "\n" (image: ''cp -r --no-preserve=mode ${image}/public/. "$out/public/"'') (builtins.attrValues imageVariants)}
''
