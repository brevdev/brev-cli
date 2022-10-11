{ pkgs ? import <nixpkgs> {} }:

with pkgs;
let
    pkgs = import (builtins.fetchGit {
         # Descriptive name to make the store path easier to identify                
         name = "my-old-revision";                                                 
         url = "https://github.com/NixOS/nixpkgs/";                       
         ref = "refs/heads/nixpkgs-unstable";                     
         rev = "ff8b619cfecb98bb94ae49ca7ceca937923a75fa";                                           
     }) {};                                                                           

     myPkg = pkgs.golangci-lint;
in
mkShell {
  nativeBuildInputs = [
    go_1_18
    gopls
    tmux
    gofumpt
    myPkg
    # golangci-lint #myPkg # instead of golang-lint-ci or whatever the package was called
    # nix-shell -p golangci-lint -I nixpkgs=https://github.com/NixOS/nixpkgs/archive/ff8b619cfecb98bb94ae49ca7ceca937923a75fa.tar.gz
    gosec
    delve
    go-tools
    gotests
    gomodifytags
  ];
}