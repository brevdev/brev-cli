#!/usr/bin/env bash
set -euo pipefail

# Removes the brev-agent systemd service.
# This script reverses everything done by install-service.sh.
# This script is idempotent and can be run multiple times safely.

SERVICE_NAME="brev-agent"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
ENV_FILE="/etc/default/${SERVICE_NAME}"

echo "Uninstalling ${SERVICE_NAME} systemd service..."

reload_systemd=false

# Stop the service if it's running
if sudo systemctl is-active --quiet "${SERVICE_NAME}" 2>/dev/null; then
    echo "Stopping service '${SERVICE_NAME}'..."
    sudo systemctl stop "${SERVICE_NAME}"
    echo "Service stopped"
else
    echo "Service '${SERVICE_NAME}' is not running"
fi

# Disable the service if it's enabled
if sudo systemctl is-enabled --quiet "${SERVICE_NAME}" 2>/dev/null; then
    echo "Disabling service '${SERVICE_NAME}'..."
    sudo systemctl disable "${SERVICE_NAME}"
    echo "Service disabled"
else
    echo "Service '${SERVICE_NAME}' is not enabled"
fi

# Remove the service file if it exists
if sudo test -f "${SERVICE_FILE}"; then
    echo "Removing service file '${SERVICE_FILE}'..."
    sudo rm -f "${SERVICE_FILE}"
    reload_systemd=true
    echo "Service file removed"
else
    echo "Service file '${SERVICE_FILE}' does not exist"
fi

# Remove the environment file if it exists
if sudo test -f "${ENV_FILE}"; then
    echo "Removing environment file '${ENV_FILE}'..."
    sudo rm -f "${ENV_FILE}"
    echo "Environment file removed"
else
    echo "Environment file '${ENV_FILE}' does not exist"
fi

# Reload systemd only if the service file was removed
if [ "${reload_systemd}" = true ]; then
    echo "Reloading systemd daemon..."
    sudo systemctl daemon-reload
    echo "Systemd daemon reloaded"
else
    echo "No systemd daemon reload needed"
fi

# Reset failed state if it exists
if sudo systemctl is-failed --quiet "${SERVICE_NAME}" 2>/dev/null; then
    echo "Resetting failed state for '${SERVICE_NAME}'..."
    sudo systemctl reset-failed "${SERVICE_NAME}" 2>/dev/null || true
fi

echo "Uninstall complete for ${SERVICE_NAME}"
