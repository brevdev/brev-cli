#!/usr/bin/env bash

set -eo pipefail

# Install the latest version of the Linux binary

# Get THE DOWNLOAD URL
DOWNLOAD_URL=$(curl -s https://brevapi.us-west-2-prod.control-plane.brev.dev/api/autostop/cli-download-url)

# download the tar to a tmp directory

TMP_DIR=$(mktemp -d)

wget --directory-prefix=$TMP_DIR $DOWNLOAD_URL

# extract the tar to the bin directory
tar -xzf $TMP_DIR/brev* -C $TMP_DIR # glob is a hack to get the filename

# move the binary to the bin directory
mv $TMP_DIR/brev /usr/local/bin/brev

# remove the tmp directory
rm -rf $TMP_DIR

# make the binary executable
chmod +x /usr/local/bin/brev

# run post install commands, write now creates a file in etc
# to store email so needs root

sudo brev postinstall
