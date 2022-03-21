{
  inputs = {
    nixpkgs.url = "nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    let
      headscaleVersion = if (self ? shortRev) then self.shortRev else "dev";
    in
    {
      overlay = final: prev:
        let
          pkgs = nixpkgs.legacyPackages.${prev.system};
        in
        rec { };
    } // flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs {
            overlays = [ self.overlay ];
            inherit system;
          };
          buildDeps = with pkgs; [
            nodejs
            yarn
            # elmPackages.elm
            # nodePackages.parcel
            git
            gnumake
          ];
          devDeps = with pkgs;
            buildDeps ++
            [
              pkg-config
              libpng
              imagemagick
              fd
            ] ++
            (with elmPackages;
            [
              elm
              elm-format
              elm-json
              elm-analyse
            ]);

        in
        rec {
          # `nix develop`
          devShell = pkgs.mkShell { buildInputs = devDeps; };

        });
}
