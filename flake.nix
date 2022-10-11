{
  description = "golang development environment (neovim + nvim_lsp + treesitter)";
inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    # system = {
    #   type = "nixExpr";
    #   value = "x86_64-linux";
    # };
    # system.type = "nixExpr";
    # system.value = "x86_64-linux"; 
  };
  # inputs.flake-utils.url = "github:numtide/flake-utils";
  # inputs.system = { type = "nixExpr"; value = "x86_64-linux"; }

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem
      (system:
        let pkgs = nixpkgs.legacyPackages.${system}; in
        {
          devShell = import ./shell.nix { inherit pkgs; };
        }
      );
}
