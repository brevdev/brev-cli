#!/bin/bash
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
install npm packages globally without sudo | modified from https://stackoverflow.com/questions/18088372/how-to-npm-install-global-not-as-root
node
mkdir "${HOME}/.npm-packages"
printf "prefix=${HOME}/.npm-packages" >> $HOME/.npmrc
cat <<EOF | tee -a ~/.bashrc | tee -a ~/.zshrc
NPM_PACKAGES="\${HOME}/.npm-packages"
NODE_PATH="\${NPM_PACKAGES}/lib/node_modules:\${NODE_PATH}"
PATH="\${NPM_PACKAGES}/bin:\${PATH}"
# command
Unset manpath so we can inherit from /etc/manpath via the `manpath`
unset MANPATH # delete if you already modified MANPATH elsewhere in your config
MANPATH="\${NPM_PACKAGES}/share/man:\$(manpath)"
EOF


# installing gatsby
node npm-no-sudo
npm install -g gatsby-cli

# golang
installing Golang v1.18
(echo ""; echo "##### Golang v18x #####"; echo "";)
wget https://golang.org/dl/go1.18.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.18.linux-amd64.tar.gz
echo "" | sudo tee -a ~/.bashrc
echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.bashrc
echo "" | sudo tee -a ~/.zshrc
echo "export PATH=\$PATH:/usr/local/go/bin" | sudo tee -a ~/.zshrc
rm go1.18.linux-amd64.tar.gz


# bumpversion
sudo apt update
sudo apt install -y python3-pip
pip install --upgrade bump2version
echo "" | sudo tee -a ~/.bashrc
echo "export PATH=\$PATH:\$HOME/.local/bin" | sudo tee -a ~/.bashrc
echo "" | sudo tee -a ~/.zshrc
echo "export PATH=\$PATH:\$HOME/.local/bin" | sudo tee -a ~/.zshrc