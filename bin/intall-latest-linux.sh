#!/usr/bin/env sh
# This script installs the latest version of brev-cli

# get the dl url for the latest release
DL_URL=$(curl -s https://api.github.com/repos/brevdev/brev-cli/releases/latest ")
