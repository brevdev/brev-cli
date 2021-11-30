#!/bin/bash

set -e

echo "Start installation..."

wget --show-progress -qO ./gateway.dmg "https://data.services.jetbrains.com/products/download?code=GW&platform=mac&type=eap,rc,release,beta"

hdiutil attach gateway.dmg
cp -R "/Volumes/JetBrains Gateway/JetBrains Gateway.app" /Applications
hdiutil unmount "/Volumes/JetBrains Gateway"
rm ./gateway.dmg

echo "JetBrains Gateway successfully installed"
