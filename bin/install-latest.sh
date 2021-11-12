mkdir -p /tmp/brev
curl -L https://github.com/brevdev/brev-cli/releases/download/v0.2.1/brev-cli_0.2.1_$(uname | awk '{print tolower($0)}')_amd64.tar.gz > /tmp/brev/brev.tar.gz
tar -xzvf /tmp/brev/brev.tar.gz -C /tmp/brev
sudo cp /tmp/brev/brev /usr/local/bin
