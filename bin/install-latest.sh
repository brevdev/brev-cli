#!/usr/bin/env bash
set -eo pipefail

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

# Install the binary to system location
sudo mv "${TMP_DIR}/brev" /usr/local/bin/brev
sudo chmod +x /usr/local/bin/brev

echo "Successfully installed brev CLI to /usr/local/bin/brev"
