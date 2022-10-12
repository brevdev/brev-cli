{
  description = "golang development environment ";
  
inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    

  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem
      (system:
        let pkgs = nixpkgs.legacyPackages.${system}; 
        pkgs.currentSystem = system;
        pkgs = import (builtins.fetchGit {
         # Descriptive name to make the store path easier to identify                
         name = "my-old-revision";                                                 
         url = "https://github.com/NixOS/nixpkgs/";                       
         ref = "refs/heads/nixpkgs-unstable";                     
         rev = "ff8b619cfecb98bb94ae49ca7ceca937923a75fa";                                           
          }) {};    
          myPkg = pkgs.golangci-lint;
        in
        {
          # devShell = import ./shell.nix { inherit pkgs; };
          devShell = pkgs.mkShell { buildInputs = [ 
              pkgs.go_1_18
              pkgs.gopls
              pkgs.tmux
              pkgs.gofumpt
              myPkg
              pkgs.gosec
              pkgs.delve
              pkgs.go-tools
              pkgs.gotests
              pkgs.gomodifytags
              ]; currentSystem = system; };
        }
      );
}
