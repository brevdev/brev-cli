#!/usr/bin/env bash
set -eo pipefail

# Fetch release metadata from GitHub API
API_RESPONSE="$(curl -sf https://api.github.com/repos/brevdev/brev-cli/releases/latest 2>&1)" || {
    echo "Error: Failed to fetch release info from GitHub API." >&2
    echo "This may be caused by rate limiting. Try again later or set a GITHUB_TOKEN." >&2
    exit 1
}

# Extract the download URL for linux/amd64
DOWNLOAD_URL="$(echo "${API_RESPONSE}" | grep "browser_download_url.*linux.*amd64" | cut -d '"' -f 4)"
if [ -z "${DOWNLOAD_URL}" ]; then
    echo "Error: Could not find release for linux amd64" >&2
    exit 1
fi

# Create temporary directory and ensure cleanup
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# Download the latest release
curl -L "${DOWNLOAD_URL}" -o "${TMP_DIR}/$(basename "${DOWNLOAD_URL}")"

# Find and extract the archive
ARCHIVE_FILE="$(find "${TMP_DIR}" -name "brev*.tar.gz" -type f)"
tar -xzf "${ARCHIVE_FILE}" -C "${TMP_DIR}"

# Install the binary to system location
sudo mv "${TMP_DIR}/brev" /usr/local/bin/brev
sudo chmod +x /usr/local/bin/brev

