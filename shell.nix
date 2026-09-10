{ pkgs }:

pkgs.mkShell {
  packages = with pkgs; [
    go
    gopls
    tmux
    gofumpt
    golangci-lint
    gosec
    delve
    gotools
    gotests
    gomodifytags
  ];
}
