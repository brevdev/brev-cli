#!/usr/bin/env bash
set -eo pipefail

# Install the latest version of the Linux binary
API_RESPONSE="$(curl -s https://api.github.com/repos/brevdev/brev-cli/releases/latest)"

# Check for GitHub API rate limit error
if echo "${API_RESPONSE}" | grep -q "API rate limit exceeded"; then
    echo "Error: GitHub API rate limit exceeded." >&2
    echo "" >&2
    echo "This often happens when many requests come from the same IP address." >&2
    echo "If you are using a VPN, try turning it off and running this script again." >&2
    echo "" >&2
    echo "For more details, see: https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting" >&2
    exit 1
fi

DOWNLOAD_URL="$(echo "${API_RESPONSE}" | grep "browser_download_url.*linux.*amd64" | cut -d '"' -f 4)"

if [ -z "${DOWNLOAD_URL}" ]; then
    echo "Error: Could not find release for linux amd64" >&2
    echo "GitHub API response:" >&2
    echo "${API_RESPONSE}" >&2
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

