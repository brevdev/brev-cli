# {
#     inputs = {
#         nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
#     };
#     outputs = { self, nixpkgs }: 
#     let
#     pkgs = nixpkgs.legacyPackages.x86_64-linux;
#     in
#     {
#     # foo = "bar";
#     packages.x86_64-linux.hello = pkgs.hello;
#     packages.x86_64-linux.hello2 = pkgs.cowsay;
#     devShell.x86_64-linux = pkgs.mkShell {
#         buildInputs = [ self.packages.x86_64-linux.hello self.packages.x86_64-linux.hello2 ];
#     };
    
#     };
# }

{
    inputs = {
        nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
        flake-utils.url = "github:numtide/flake-utils";
    };
    outputs = { self, nixpkgs, flake-utils }: 
        flake-utils.lib.eachDefaultSystem (system:
            let
            pkgs = nixpkgs.legacyPackages.${system};
            in
            {
            # foo = "bar";
            packages.hello = pkgs.hello;

            devShell = pkgs.mkShell { buildInputs = [ pkgs.hello pkgs.cowsay ]; };
        });
}
