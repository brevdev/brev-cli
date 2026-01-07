#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

BIN_SOURCE_DEFAULT="${ROOT_DIR}/brevd"
BIN_TARGET_DEFAULT="/usr/local/bin/brevd"
ENV_FILE_DEFAULT="/etc/default/brevd"
UNIT_SOURCE="${SCRIPT_DIR}/systemd/brevd.service"
UNIT_TARGET_DEFAULT="/etc/systemd/system/brevd.service"
AUTO_ENABLE=true

usage() {
	cat <<EOF
Usage: sudo install.sh [options]

Copies the brevd binary, installs the systemd unit, writes a default env file,
and reloads/starts the service.

Options:
  --binary PATH        Path to existing brevd binary (default: ${BIN_SOURCE_DEFAULT})
  --bin-target PATH    Destination binary path (default: ${BIN_TARGET_DEFAULT})
  --env-file PATH      Environment file path (default: ${ENV_FILE_DEFAULT})
  --unit-path PATH     Destination systemd unit path (default: ${UNIT_TARGET_DEFAULT})
  --skip-enable        Install files but do not enable/start the service
  -h, --help           Show this message
EOF
}

require_root() {
	if [[ "${EUID}" -ne 0 ]]; then
		echo "This script must be run as root (try sudo)." >&2
		exit 1
	fi
}

ensure_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "Missing required command: $1" >&2
		exit 1
	fi
}

BIN_SOURCE="${BIN_SOURCE_DEFAULT}"
BIN_TARGET="${BIN_TARGET_DEFAULT}"
ENV_FILE="${ENV_FILE_DEFAULT}"
UNIT_TARGET="${UNIT_TARGET_DEFAULT}"

while [[ $# -gt 0 ]]; do
	case "$1" in
		--binary)
			BIN_SOURCE="$2"
			shift 2
			;;
		--bin-target)
			BIN_TARGET="$2"
			shift 2
			;;
		--env-file)
			ENV_FILE="$2"
			shift 2
			;;
		--unit-path)
			UNIT_TARGET="$2"
			shift 2
			;;
		--skip-enable)
			AUTO_ENABLE=false
			shift 1
			;;
		-h|--help)
			usage
			exit 0
			;;
		*)
			echo "Unknown option: $1" >&2
			usage
			exit 1
			;;
	esac
done

main() {
	require_root
	ensure_cmd install
	ensure_cmd systemctl

	if [[ ! -f "${BIN_SOURCE}" ]]; then
		echo "brevd binary not found at ${BIN_SOURCE}" >&2
		exit 1
	fi

	echo "Installing brevd binary to ${BIN_TARGET}"
	install -m 0755 "${BIN_SOURCE}" "${BIN_TARGET}"

	if [[ ! -f "${UNIT_SOURCE}" ]]; then
		echo "unit file missing at ${UNIT_SOURCE}" >&2
		exit 1
	fi

	echo "Installing systemd unit to ${UNIT_TARGET}"
	install -m 0644 "${UNIT_SOURCE}" "${UNIT_TARGET}"

	if [[ ! -f "${ENV_FILE}" ]]; then
		echo "Creating default env file at ${ENV_FILE}"
		cat <<"EOF" > "${ENV_FILE}"
# Env vars consumed by brevd. Replace placeholder values before starting.
BREV_AGENT_BREVCLOUD_URL="https://controlplane.example.com/agent/v1"
BREV_AGENT_REGISTRATION_TOKEN="replace-me"
BREV_AGENT_CLOUD_CRED_ID="replace-me"
# Optional overrides:
# BREV_AGENT_DISPLAY_NAME="my-node"
# BREV_AGENT_CLOUD_NAME="edge-cluster-a"
# BREV_AGENT_STATE_DIR="/var/lib/brevd"
# BREV_AGENT_DEVICE_TOKEN_PATH="/var/lib/brevd/device_token"
# BREV_AGENT_HEARTBEAT_INTERVAL="30s"
# BREV_AGENT_ENABLE_TUNNEL="true"
# BREV_AGENT_TUNNEL_SSH_PORT="22"
EOF
	else
		echo "Env file already exists at ${ENV_FILE}; leaving as-is"
	fi
	chmod 600 "${ENV_FILE}"

	echo "Reloading systemd units"
	systemctl daemon-reload

	if [[ "${AUTO_ENABLE}" == true ]]; then
		echo "Enabling and starting brevd"
		systemctl enable --now "$(basename "${UNIT_TARGET}")"
	else
		echo "Skipping enable/start per --skip-enable"
	fi

	echo "Done."
}

main "$@"

