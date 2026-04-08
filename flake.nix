{
  description = "NVIDIA Brev CLI";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
      treefmt-nix,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = self.shortRev or self.dirtyShortRev or "dev";
        brev-cli = pkgs.callPackage ./default.nix { inherit version; };
        treefmtEval = treefmt-nix.lib.evalModule pkgs {
          projectRootFile = "flake.nix";
          programs.nixfmt.enable = true;
        };
      in
      {
        formatter = treefmtEval.config.build.wrapper;

        packages = {
          default = brev-cli;
          inherit brev-cli;
        };

        apps.default = flake-utils.lib.mkApp {
          drv = brev-cli;
        };

        devShells.default = import ./shell.nix { inherit pkgs system; };
        devShell = self.devShells.${system}.default;
      }
    );
}
