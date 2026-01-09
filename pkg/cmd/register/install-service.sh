#!/usr/bin/env bash
set -eo pipefail

STATE_DIR="${STATE_DIR:-/home/brevcloud/.brev-agent}"

# Create systemd service file
sudo tee /etc/systemd/system/brevd.service > /dev/null <<'EOF'
[Unit]
Description=Brev Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-/etc/default/brevd
ExecStart=/usr/local/bin/brevd spark agent
Restart=on-failure
RestartSec=10s
User=brevcloud
Group=brevcloud

[Install]
WantedBy=multi-user.target
EOF

# Create default environment file if it doesn't exist
if [ ! -f /etc/default/brevd ]; then
    sudo tee /etc/default/brevd > /dev/null <<EOF
# Env vars consumed by brevd. These will be populated during enrollment.
BREV_AGENT_BREV_CLOUD_NODE_ID=""
BREV_AGENT_BREV_CLOUD_URL=""
BREV_AGENT_REGISTRATION_TOKEN=""
BREV_AGENT_CLOUD_CRED_ID=""
BREV_AGENT_STATE_DIR="${STATE_DIR}"
EOF
fi

sudo chmod 600 /etc/default/brevd

# Reload systemd
sudo systemctl daemon-reload

echo "Successfully installed brevd systemd service"