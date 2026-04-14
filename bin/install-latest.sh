#!/usr/bin/env bash
set -eo pipefail

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

# Fetch release metadata from GitHub API
API_RESPONSE="$(curl -sf ${GITHUB_TOKEN:+-H "Authorization: token ${GITHUB_TOKEN}"} https://api.github.com/repos/brevdev/brev-cli/releases/latest)" || {
    echo "Error: Failed to fetch release info from GitHub API." >&2
    echo "This is often caused by rate limiting when many requests come from the same IP." >&2
    echo "If you are using a VPN, try turning it off and running this script again." >&2
    echo "You can also set GITHUB_TOKEN to avoid rate limits." >&2
    echo "For more details, see: https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting" >&2
    exit 1
}

# Extract the download URL for this platform
DOWNLOAD_URL="$(echo "${API_RESPONSE}" | grep "browser_download_url.*${OS}.*${ARCH}" | cut -d '"' -f 4 || true)"
if [ -z "${DOWNLOAD_URL}" ]; then
    echo "Error: Could not find release for ${OS} ${ARCH}" >&2
    echo "GitHub API response (truncated): ${API_RESPONSE:0:200}" >&2
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
