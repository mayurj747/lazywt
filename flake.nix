{
  description = "lazywt — a TUI for managing git worktrees";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go_1_25
            pkgs.air
            pkgs.gopls
            pkgs.gotools  # goimports, etc.
          ];
        };

        packages.default = pkgs.buildGoModule {
          pname = "lw";
          version = "0.1.0";
          src = ./.;
          vendorHash = null;
        };
      });
}
