#!/usr/bin/env bash
set -eo pipefail

# Install the latest version of the Linux binary
DOWNLOAD_URL="$(curl -s https://api.github.com/repos/brevdev/brev-cli/releases/latest | grep "browser_download_url.*linux.*amd64" | cut -d '"' -f 4)"

# Create temporary directory and ensure cleanup
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

# Download the latest release
curl -L "${DOWNLOAD_URL}" -o "${TMP_DIR}/$(basename "${DOWNLOAD_URL}")"

# Find and extract the archive
ARCHIVE_FILE="$(find "${TMP_DIR}" -name "brev*.tar.gz" -type f)"
tar -xzf "${ARCHIVE_FILE}" -C "${TMP_DIR}"

# Install the binary to user-local location (no sudo required)
INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "${INSTALL_DIR}"
mv "${TMP_DIR}/brev" "${INSTALL_DIR}/brev"
chmod +x "${INSTALL_DIR}/brev"

echo "Successfully installed brev CLI to ${INSTALL_DIR}/brev"

# Warn if the install directory is not in PATH
case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
        echo ""
        echo "WARNING: ${INSTALL_DIR} is not in your PATH."
        echo "Add it by running:"
        echo ""
        echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
        echo ""
        ;;
esac

