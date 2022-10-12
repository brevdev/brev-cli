#!/bin/bash

### docker ###
# https://docs.docker.com/engine/install/ubuntu/
sudo apt-get install -y \
    ca-certificates \
    curl \
    gnupg \
    lsb-release
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --yes --dearmor -o /etc/apt/keyrings/docker.gpg
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
# https://docs.docker.com/engine/install/linux-postinstall/
sudo systemctl enable docker.service
sudo systemctl enable containerd.service
sudo usermod -aG docker $USER

sudo apt-get install -y make build-essential zip

# ff only
git config --global pull.ff only

# git diff-blame
wget https://raw.githubusercontent.com/dmnd/git-diff-blame/master/git-diff-blame
chmod +x git-diff-blame
sudo mv git-diff-blame /usr/local/bin

# node node
Node v14.x + npm
(echo ""; echo "##### Node v14.x + npm #####"; echo "";)
sudo apt install ca-certificates
curl -fsSL https://deb.nodesource.com/setup_14.x | sudo -E bash -
sudo apt-get install -y nodejs
(echo ""; echo "##### Node v14.x + npm #####"; echo "";)
sudo apt install ca-certificates
curl -fsSL https://deb.nodesource.com/setup_14.x | sudo -E bash -
sudo apt-get install -y nodejs

# npm-no-sudo
# install npm packages globally without sudo | modified from https://stackoverflow.com/questions/18088372/how-to-npm-install-global-not-as-root
mkdir "${HOME}/.npm-packages"
printf "prefix=${HOME}/.npm-packages" >> $HOME/.npmrc
cat <<EOF | tee -a ~/.bashrc | tee -a ~/.zshrc
NPM_PACKAGES="\${HOME}/.npm-packages"
NODE_PATH="\${NPM_PACKAGES}/lib/node_modules:\${NODE_PATH}"
PATH="\${NPM_PACKAGES}/bin:\${PATH}"
# command
# Unset manpath so we can inherit from /etc/manpath via the `manpath`
unset MANPATH # delete if you already modified MANPATH elsewhere in your config
MANPATH="\${NPM_PACKAGES}/share/man:\$(manpath)"
EOF


# installing gatsby
node npm-no-sudo
npm install -g gatsby-cli

# golang
# installing Golang v1.18
(echo ""; echo "##### Golang v18x #####"; echo "";)
wget https://golang.org/dl/go1.18.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.18.linux-amd64.tar.gz
echo "" | sudo tee -a ~/.bashrc
echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.bashrc
echo "" | sudo tee -a ~/.zshrc
echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.zshrc
echo "" | sudo tee -a ~/.bashrc
echo "export PATH=\$PATH:\$HOME/go/bin" | sudo tee -a ~/.bashrc
echo "" | sudo tee -a ~/.zshrc
echo "export PATH=\$PATH:\$HOME/go/bin" | sudo tee -a ~/.zshrc
rm go1.18.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# install golang extension tools
export GOPATH=$HOME/go
go install -v golang.org/x/tools/gopls@latest
go install -v github.com/go-delve/delve/cmd/dlv@latest
go install -v honnef.co/go/tools/cmd/staticcheck@latest
go install -v github.com/cweill/gotests/gotests@latest
go install -v github.com/fatih/gomodifytags@latest
go install -v github.com/josharian/impl@latest
go install -v github.com/haya14busa/goplay/cmd/goplay@latest
go install -v github.com/ramya-rao-a/go-outline@latest

# bumpversion
sudo apt update
sudo apt install -y python3-pip
pip install --upgrade bump2version
echo "" | sudo tee -a ~/.bashrc
echo "export PATH=\$PATH:\$HOME/.local/bin" | sudo tee -a ~/.bashrc
echo "" | sudo tee -a ~/.zshrc
echo "export PATH=\$PATH:\$HOME/.local/bin" | sudo tee -a ~/.zshrc

newgrp docker 

# NIX STUFF (this can be installed without any of the above and it should still work):
wget https://nixos.org/nix/install -O nix-install
yes | sh nix-install --daemon
sudo sh -c "echo 'build-users-group = nixbld
keep-outputs = true
keep-derivations = true
experimental-features = nix-command flakes
trusted-users = root ubuntu
build-users-group = nixbld
' > /etc/nix/nix.conf"