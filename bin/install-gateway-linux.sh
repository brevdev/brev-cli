#!/bin/bash

set -e

echo "Start installation..."

wget --show-progress -qO ./gateway.tar.gz "https://data.services.jetbrains.com/products/download?code=GW&platform=linux&type=eap,rc,release,beta"

GATEWAY_TEMP_DIR=$(mktemp -d)

tar -C "$GATEWAY_TEMP_DIR" -xf gateway.tar.gz
rm ./toolbox.tar.gz

"$GATEWAY_TEMP_DIR"/*/bin/gateway.sh

rm -r "$GATEWAY_TEMP_DIR"

echo "JetBrains Gateway was successfully installed"