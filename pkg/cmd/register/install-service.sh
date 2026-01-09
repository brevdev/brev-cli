#!/usr/bin/env bash
set -euo pipefail

# Installs the brev-agent systemd service.
# This script is idempotent and can be run multiple times safely.

STATE_DIR="${STATE_DIR:-/home/brevcloud/.brev-agent}"
SERVICE_FILE="/etc/systemd/system/brev-agent.service"
ENV_FILE="/etc/default/brev-agent"

echo "Installing brev-agent systemd service..."

# Define the desired service file content
read -r -d '' SERVICE_CONTENT <<'EOF' || true
[Unit]
Description=Brev Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-/etc/default/brev-agent
ExecStart=/usr/local/bin/brev agent
Restart=on-failure
RestartSec=10s
User=brevcloud
Group=brevcloud

[Install]
WantedBy=multi-user.target
EOF

# Check if service file exists and has correct content
reload_systemd=false
if sudo test -f "${SERVICE_FILE}"; then
    existing_content=$(sudo cat "${SERVICE_FILE}" 2>/dev/null || echo "")
    if [ "${existing_content}" = "${SERVICE_CONTENT}" ]; then
        echo "Service file '${SERVICE_FILE}' already exists with correct content"
    else
        echo "Updating service file '${SERVICE_FILE}'..."
        echo "${SERVICE_CONTENT}" | sudo tee "${SERVICE_FILE}" > /dev/null
        reload_systemd=true
    fi
else
    echo "Creating service file '${SERVICE_FILE}'..."
    echo "${SERVICE_CONTENT}" | sudo tee "${SERVICE_FILE}" > /dev/null
    reload_systemd=true
fi

# Create default environment file if it doesn't exist
if sudo test -f "${ENV_FILE}"; then
    echo "Environment file '${ENV_FILE}' already exists, preserving existing configuration"
else
    echo "Creating environment file '${ENV_FILE}'..."
    sudo tee "${ENV_FILE}" > /dev/null <<EOF
# Env vars consumed by brev-agent. These will be populated during enrollment.
BREV_AGENT_BREV_CLOUD_NODE_ID=""
BREV_AGENT_BREV_CLOUD_URL=""
BREV_AGENT_REGISTRATION_TOKEN=""
BREV_AGENT_CLOUD_CRED_ID=""
BREV_AGENT_STATE_DIR="${STATE_DIR}"
EOF
fi

# Ensure correct permissions on environment file
echo "Ensuring correct permissions on '${ENV_FILE}'..."
sudo chmod 600 "${ENV_FILE}"

# Reload systemd only if the service file was created or changed
if [ "${reload_systemd}" = true ]; then
    echo "Reloading systemd daemon..."
    sudo systemctl daemon-reload
else
    echo "No systemd daemon reload needed"
fi

# Enable the service to start on boot
if sudo systemctl is-enabled --quiet brev-agent 2>/dev/null; then
    echo "Service 'brev-agent' is already enabled"
else
    echo "Enabling service 'brev-agent' to start on boot..."
    sudo systemctl enable brev-agent
    echo "Service enabled"
fi

# Start the service if it's not already running
if sudo systemctl is-active --quiet brev-agent 2>/dev/null; then
    echo "Service 'brev-agent' is already running"
else
    echo "Starting service 'brev-agent'..."
    sudo systemctl start brev-agent
    echo "Service started"
fi

echo "Brev-agent systemd service is ready and running"