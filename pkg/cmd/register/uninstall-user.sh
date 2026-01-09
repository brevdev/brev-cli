#!/usr/bin/env bash
set -euo pipefail

# Removes the brev service user and all associated configuration.
# This script reverses everything done by install-user.sh.

BREV_USER="${BREV_USER:-brevcloud}"
BREV_HOME="${BREV_HOME:-/home/${BREV_USER}}"
SUDOERS_FILE="/etc/sudoers.d/${BREV_USER}"

echo "Uninstalling user ${BREV_USER}..."

# Remove the sudoers file if it exists
if sudo test -f "${SUDOERS_FILE}"; then
    echo "Removing sudoers file '${SUDOERS_FILE}'..."
    sudo rm -f "${SUDOERS_FILE}"
    echo "Sudoers file removed"
else
    echo "Sudoers file '${SUDOERS_FILE}' does not exist, skipping..."
fi

# Check if the user exists
if id -u "${BREV_USER}" >/dev/null 2>&1; then
    echo "User '${BREV_USER}' exists, proceeding with removal..."
    
    # Kill any processes owned by the user
    if pgrep -u "${BREV_USER}" >/dev/null 2>&1; then
        echo "Killing processes owned by '${BREV_USER}'..."
        sudo pkill -u "${BREV_USER}" || true
        sleep 2
        # Force kill if still running
        if pgrep -u "${BREV_USER}" >/dev/null 2>&1; then
            echo "Force killing remaining processes..."
            sudo pkill -9 -u "${BREV_USER}" || true
            sleep 1
        fi
    fi
    
    # Remove the user and home directory
    echo "Removing user '${BREV_USER}' and home directory..."
    sudo userdel -r "${BREV_USER}" 2>/dev/null || {
        # If userdel -r fails (e.g., home directory doesn't exist or is already removed)
        # try without -r flag
        sudo userdel "${BREV_USER}" 2>/dev/null || true
    }
    
    # Manually remove home directory if it still exists
    if sudo test -d "${BREV_HOME}"; then
        echo "Manually removing home directory '${BREV_HOME}'..."
        sudo rm -rf "${BREV_HOME}"
    fi
    
    echo "User and home directory removed"
else
    echo "User '${BREV_USER}' does not exist, skipping user removal..."
fi

# Clean up any remaining group if it exists and is empty
if getent group "${BREV_USER}" >/dev/null 2>&1; then
    echo "Removing group '${BREV_USER}'..."
    sudo groupdel "${BREV_USER}" 2>/dev/null || {
        echo "Note: Group '${BREV_USER}' could not be removed (may still be in use)"
    }
fi

echo "Uninstall complete for user ${BREV_USER}"
