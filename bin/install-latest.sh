#!/usr/bin/env sh
mkdir -p /tmp/brev
wget https://github.com/brevdev/brev-cli/releases/download/v0.1.1/brev-cli_0.1.1_linux_amd64.tar.gz -O /tmp/brev/brev.tar.gz
tar -xzvf /tmp/brev/brev.tar.gz -C /tmp/brev
sudo cp /tmp/brev/brev /usr/local/bin
mkdir -p /tmp/brev
wget https://github.com/brevdev/brev-cli/releases/download/v0.1.1/brev-cli_0.1.1_linux_amd64.tar.gz -O /tmp/brev/brev.tar.gz
tar -xzvf /tmp/brev/brev.tar.gz -C /tmp/brev
sudo cp /tmp/brev/brev /usr/local/bin
