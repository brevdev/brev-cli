#!/bin/sh
LATEST_TAG_FULL=$(curl -H "Accept: application/vnd.github.v3.text+json" "https://api.github.com/repos/brevdev/brev-cli/tags?per_page=1" |  grep -o "\"v.*\"" | sed 's/"//g')
LATEST_TAG_ABBREV=$(curl -H "Accept: application/vnd.github.v3.text+json" "https://api.github.com/repos/brevdev/brev-cli/tags?per_page=1" |  grep -o "\"v.*\"" | sed 's/v//' | sed 's/"//g')
mkdir -p /tmp/brev
curl -L https://github.com/brevdev/brev-cli/releases/download/${LATEST_TAG_FULL}/brev-cli_${LATEST_TAG_ABBREV}_$(uname | awk '{print tolower($0)}')_amd64.tar.gz > /tmp/brev/brev.tar.gz
tar -xzvf /tmp/brev/brev.tar.gz -C /tmp/brev
sudo cp /tmp/brev/brev /usr/bin
