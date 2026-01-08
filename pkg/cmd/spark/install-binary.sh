#!/usr/bin/env bash
set -eo pipefail

# Installs the latest brev-cli binary as "brevd" from GitHub releases

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

# Get the appropriate download URL for this platform
DOWNLOAD_URL="$(curl -s https://api.github.com/repos/brevdev/brev-cli/releases/latest | grep "browser_download_url.*${OS}.*${ARCH}" | cut -d '"' -f 4)"

# Verify we found a suitable release
if [ -z "${DOWNLOAD_URL}" ]; then
    echo "Error: Could not find release for ${OS} ${ARCH}" >&2
    exit 1
fi

# Create temporary directory and ensure cleanup
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# Download and extract the release
curl -sL "${DOWNLOAD_URL}" -o "${TMP_DIR}/brev.tar.gz"
tar -xzf "${TMP_DIR}/brev.tar.gz" -C "${TMP_DIR}"

# Install the binary as "brevd" to /usr/local/bin/brevd
sudo mv "${TMP_DIR}/brev" /usr/local/bin/brevd
sudo chmod +x /usr/local/bin/brevd

echo "Successfully installed brevd to /usr/local/bin/brevd"
