#!/usr/bin/env bash
set -eo pipefail

# Install the latest version of the Linux binary
DOWNLOAD_URL="$(curl -s https://api.github.com/repos/brevdev/brev-cli/releases/latest | grep "browser_download_url.*linux.*amd64" | cut -d '"' -f 4)"

# Create temporary directory and ensure cleanup
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# Download the latest release
wget --directory-prefix="${TMP_DIR}" "${DOWNLOAD_URL}"

# Find and extract the archive
ARCHIVE_FILE="$(find "${TMP_DIR}" -name "brev*.tar.gz" -type f)"
tar -xzf "${ARCHIVE_FILE}" -C "${TMP_DIR}"

# Install the binary to system location
sudo mv "${TMP_DIR}/brev" /usr/local/bin/brev
sudo chmod +x /usr/local/bin/brev

# Run post-installation setup
sudo brev postinstall
