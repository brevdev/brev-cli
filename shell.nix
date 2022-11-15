{ system ? builtins.currentSystem, pkgs ? import <nixpkgs> { inherit system; } }:

with pkgs;
let
    pkgs = import (builtins.fetchGit {
         # Descriptive name to make the store path easier to identify                
         name = "my-old-revision";                                                 
         url = "https://github.com/NixOS/nixpkgs/";                       
         ref = "refs/heads/nixpkgs-unstable";                     
         rev = "ff8b619cfecb98bb94ae49ca7ceca937923a75fa";                                           
    #  }) {};    
     }) {
      inherit system;
    };                                                                         
    flake-utils = {
      url = "github:numtide/flake-utils";
      inputs.nixpkgs.follows = "nixpkgs";
    };
     olderVersionOfGolangci-lint = pkgs.golangci-lint;
    #  system = builtins.currentSystem;
in
          mkShell {
            nativeBuildInputs = [
              go_1_19
              gopls
              tmux
              gofumpt
              olderVersionOfGolangci-lint
              gosec
              delve
              go-tools
              gotests
              gomodifytags
            ];
            # pkgs.system="x86_64-linux";
          }
        # )