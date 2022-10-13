#!/usr/bin/env sh
# Install the latest version of the Linux binary

set -eo pipefail

# Get THE DOWNLOAD URL
DOWNLOAD_URL=$(curl -s https://brevapi.us-west-2-prod.control-plane.brev.dev)

# download the tar to a tmp directory

TMP_DIR=$(mktemp -d)
curl -sL "$DOWNLOAD_URL" -o "$TMP_DIR/brev.tar.gz"

# extract the tar to the bin directory
tar -xzf "$TMP_DIR/brev.tar.gz" -C "$TMP_DIR"

# move the binary to the bin directory
mv "$TMP_DIR/brev" /usr/local/bin/brev

# remove the tmp directory
rm -rf "$TMP_DIR"

# make the binary executable
chmod +x /usr/local/bin/brev

# run post install commands

brev postinstall
