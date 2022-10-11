{ pkgs ? import <nixpkgs> {} }:

with pkgs;
let
    # x = import (builtins.fetchTarball {
    #     url = "https://github.com/NixOS/nixpkgs/archive/ff8b619cfecb98bb94ae49ca7ceca937923a75fa.tar.gz";
    #     sha256 = "0h7wqi8xnxs7dimc1xd0cmzvni5d526v6ch385iapsws7lqmwpva";
    # }) {};

    # myPkg = x.golangci-lint;
in
mkShell {
  nativeBuildInputs = [
    go_1_18
    gopls
    tmux
    gofumpt
    # golangci-lint #myPkg # instead of golang-lint-ci or whatever the package was called
    gosec
    delve
    go-tools
    gotests
    gomodifytags
  ];
}