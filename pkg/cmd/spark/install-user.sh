#!/usr/bin/env bash
set -euo pipefail

# Creates the brev service user with passwordless sudo and an SSH directory.

BREV_USER="${BREV_USER:-brevcloud}"
BREV_HOME="${BREV_HOME:-/home/${BREV_USER}}"
SUDOERS_FILE="/etc/sudoers.d/${BREV_USER}"

echo "Configuring user ${BREV_USER}..."

# Create the user if it does not already exist.
if ! id -u "${BREV_USER}" >/dev/null 2>&1; then
    sudo useradd -m -d "${BREV_HOME}" -s /bin/bash "${BREV_USER}"
fi

# Ensure the home directory exists with the right permissions.
sudo install -d -m 700 -o "${BREV_USER}" -g "${BREV_USER}" "${BREV_HOME}"

# Grant passwordless sudo to the brev user.
echo "${BREV_USER} ALL=(ALL) NOPASSWD:ALL" | sudo tee "${SUDOERS_FILE}" >/dev/null
sudo chmod 0440 "${SUDOERS_FILE}"
sudo visudo -c -f "${SUDOERS_FILE}"

# Prepare SSH directory and authorized_keys.
sudo install -d -m 700 -o "${BREV_USER}" -g "${BREV_USER}" "${BREV_HOME}/.ssh"
sudo touch "${BREV_HOME}/.ssh/authorized_keys"
sudo chmod 600 "${BREV_HOME}/.ssh/authorized_keys"
sudo chown -R "${BREV_USER}:${BREV_USER}" "${BREV_HOME}/.ssh"

# Final ownership consistency.
sudo chown -R "${BREV_USER}:${BREV_USER}" "${BREV_HOME}"

echo "User ${BREV_USER} is ready at ${BREV_HOME}"