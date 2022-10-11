{ pkgs ? import <nixpkgs> {} }:

with pkgs;
let
    pkgs = import (builtins.fetchTarball {
        url = "https://github.com/NixOS/nixpkgs/archive/ff8b619cfecb98bb94ae49ca7ceca937923a75fa.tar.gz";
    }) {};

    myPkg = pkgs.golangci-lint;
in
mkShell {
  nativeBuildInputs = [
    go_1_18
    gopls
    tmux
    gofumpt
    # golangci-lint #myPkg # instead of golang-lint-ci or whatever the package was called
    # nix-shell -p golangci-lint -I nixpkgs=https://github.com/NixOS/nixpkgs/archive/ff8b619cfecb98bb94ae49ca7ceca937923a75fa.tar.gz
    gosec
    delve
    go-tools
    gotests
    gomodifytags
  ];
}