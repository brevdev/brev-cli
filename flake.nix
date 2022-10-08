{
  description = "A Nix-flake-based Go 1.17 development environment";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:NixOS/nixpkgs";
  };

  outputs =
    { self
    , flake-utils
    , nixpkgs
    }:

    flake-utils.lib.eachDefaultSystem (system:
    let
      goVersion = 18;
      overlays = [ (self: super: { go = super."go_1_${toString goVersion}"; }) ];
      pkgs = import nixpkgs { inherit overlays system; };
      # frameworks = pkgs.darwin.apple_sdk.frameworks;

    in
    {
      devShell = pkgs.mkShellNoCC {
        buildInputs = with pkgs; [
          go
          gotools
        gopls
        go-outline
        gocode
        gopkgs
        gocode-gomod
        godef
        golint
        # pkgs.rustc
        # pkgs.cargo
        # frameworks.Security
        # frameworks.CoreFoundation
        # frameworks.CoreServices
        ];

        shellHook = ''
          ${pkgs.go}/bin/go version
          
        '';
      };
    });
}

# export PS1="[$name] \[$txtgrn\]\u@\h\[$txtwht\]:\[$bldpur\]\w \[$txtcyn\]\$git_branch\[$txtred\]\$git_dirty \[$bldylw\]\$aws_env\[$txtrst\]\$ "
# export NIX_LDFLAGS="-F${frameworks.CoreFoundation}/Library/Frameworks -framework CoreFoundation $NIX_LDFLAGS";