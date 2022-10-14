# NIX STUFF:
wget https://nixos.org/nix/install -O nix-install
yes | sh nix-install --daemon
sudo sh -c "echo 'build-users-group = nixbld
keep-outputs = true
keep-derivations = true
experimental-features = nix-command flakes
trusted-users = root ubuntu
build-users-group = nixbld
' > /etc/nix/nix.conf"