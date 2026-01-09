#!/usr/bin/env bash
set -euo pipefail

# Creates the brev service user with passwordless sudo and an SSH directory.
# This script is idempotent and can be run multiple times safely.

BREV_USER="${BREV_USER:-brevcloud}"
BREV_HOME="${BREV_HOME:-/home/${BREV_USER}}"
SUDOERS_FILE="/etc/sudoers.d/${BREV_USER}"

echo "Configuring user ${BREV_USER}..."

# Create or update the BREV_USER
if ! id -u "${BREV_USER}" >/dev/null 2>&1; then
    # If the BREV_USER does not exist, create it and set the home directory and shell to /bin/bash
    echo "Creating user '${BREV_USER}' with home directory '${BREV_HOME}' and shell '/bin/bash'..."
    sudo useradd -m -d "${BREV_HOME}" -s /bin/bash "${BREV_USER}"
else
    # If the BREV_USER exists, ensure the shell is set to /bin/bash if it is not already
    echo "User '${BREV_USER}' already exists"
    current_shell=$(getent passwd "${BREV_USER}" | cut -d: -f7)
    if [ "${current_shell}" != "/bin/bash" ]; then
        echo "Updating user '${BREV_USER}'s shell to '/bin/bash'..."
        sudo usermod -s /bin/bash "${BREV_USER}"
    else
        echo "User '${BREV_USER}'s shell is already '/bin/bash'"
    fi
fi

# Ensure the home directory exists with the right permissions.
if [ ! -d "${BREV_HOME}" ]; then
    echo "Creating home directory ${BREV_HOME}..."
    sudo install -d -m 700 -o "${BREV_USER}" -g "${BREV_USER}" "${BREV_HOME}"
else
    # Directory exists, ensure correct permissions and ownership
    echo "Home directory '${BREV_HOME}' already exists, ensuring correct permissions and ownership..."
    sudo chown "${BREV_USER}:${BREV_USER}" "${BREV_HOME}"
    sudo chmod 700 "${BREV_HOME}"
fi

# Grant passwordless sudo to the brev user (idempotent).
sudoers_content="${BREV_USER} ALL=(ALL) NOPASSWD:ALL"
if sudo test -f "${SUDOERS_FILE}"; then
    existing_content=$(sudo cat "${SUDOERS_FILE}" 2>/dev/null || echo "")
    if [ "${existing_content}" = "${sudoers_content}" ]; then
        echo "Sudoers file already configured correctly"
    else
        echo "Updating sudoers file..."
        echo "${sudoers_content}" | sudo tee "${SUDOERS_FILE}" >/dev/null
        sudo chmod 0440 "${SUDOERS_FILE}"
        sudo visudo -c -f "${SUDOERS_FILE}"
    fi
else
    echo "Creating sudoers file..."
    echo "${sudoers_content}" | sudo tee "${SUDOERS_FILE}" >/dev/null
    sudo chmod 0440 "${SUDOERS_FILE}"
    sudo visudo -c -f "${SUDOERS_FILE}"
fi

# Prepare SSH directory and authorized_keys.
ssh_dir="${BREV_HOME}/.ssh"
authorized_keys="${ssh_dir}/authorized_keys"

if ! sudo test -d "${ssh_dir}"; then
    echo "Creating SSH directory..."
    sudo install -d -m 700 -o "${BREV_USER}" -g "${BREV_USER}" "${ssh_dir}"
else
    # Directory exists, ensure correct permissions and ownership
    echo "SSH directory '${ssh_dir}' already exists, ensuring correct permissions and ownership..."
    sudo chown "${BREV_USER}:${BREV_USER}" "${ssh_dir}"
    sudo chmod 700 "${ssh_dir}"
fi

if ! sudo test -f "${authorized_keys}"; then
    echo "Creating authorized_keys file..."
    sudo touch "${authorized_keys}"
else
    echo "Authorized keys file '${authorized_keys}' already exists"
fi

# Ensure correct permissions and ownership for authorized_keys
echo "Ensuring correct permissions and ownership for authorized_keys..."
sudo chown "${BREV_USER}:${BREV_USER}" "${authorized_keys}"
sudo chmod 600 "${authorized_keys}"

# Final ownership consistency check (avoid unnecessary recursive chown).
# Only fix ownership if something is wrong to avoid touching all files unnecessarily.
if [ "$(stat -c '%U' "${BREV_HOME}" 2>/dev/null || stat -f '%Su' "${BREV_HOME}" 2>/dev/null)" != "${BREV_USER}" ]; then
    echo "Fixing home directory ownership..."
    sudo chown -R "${BREV_USER}:${BREV_USER}" "${BREV_HOME}"
fi

echo "User ${BREV_USER} is ready at ${BREV_HOME}"