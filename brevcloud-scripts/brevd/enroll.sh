#!/usr/bin/env bash
set -euo pipefail

LOG_DIR=/tmp/brevd
LOG_FILE="${LOG_DIR}/enroll.log"
ENV_FILE="/etc/default/brevd"
SERVICE="brevd"
MOCK=%t

log() {
	local msg="$1"
	mkdir -p "${LOG_DIR}"
	logger -t brev-enroll "${msg}" 2>/dev/null || true
	printf '%s\n' "${msg}" >>"${LOG_FILE}"
}

run_cmd() {
	local cmd="$1"
	bash -c "${cmd}" >>"${LOG_FILE}" 2>&1
}

ensure_dir() {
	local dir="$1"
	local cmds=(
		"sudo -n mkdir -p \"${dir}\""
		"sudo mkdir -p \"${dir}\""
		"mkdir -p \"${dir}\""
	)
	for c in "${cmds[@]}"; do
		if run_cmd "${c}"; then
			return 0
		fi
	done
	log "failed to create dir ${dir}"
	exit 1
}

write_file() {
	local content="$1"
	local dest="$2"
	local tmp="${dest}.tmp"

	printf '%s\n' "${content}" >"${tmp}"

	local cmds=(
		"sudo -n tee \"${dest}\" >/dev/null"
		"sudo tee \"${dest}\" >/dev/null"
		"tee \"${dest}\" >/dev/null"
	)
	for c in "${cmds[@]}"; do
		if bash -c "${c}" <"${tmp}" >>"${LOG_FILE}" 2>&1; then
			rm -f "${tmp}"
			return 0
		fi
	done

	log "failed to write file to ${dest}"
	rm -f "${tmp}"
	exit 1
}

set_file_mode() {
	local mode="$1"
	local path="$2"
	local cmds=(
		"sudo -n chmod ${mode} \"${path}\""
		"sudo chmod ${mode} \"${path}\""
		"chmod ${mode} \"${path}\""
	)
	for c in "${cmds[@]}"; do
		if run_cmd "${c}"; then
			return 0
		fi
	done
	log "failed to chmod ${mode} ${path}"
	exit 1
}

extract_env_value() {
	# Extracts an env value from a string containing KEY=VALUE lines.
	# Leaves quoting intact only long enough to strip it; always succeeds.
	local key="$1"
	local content="$2"
	while IFS= read -r line; do
		# Skip comments and blank lines.
		if [[ -z "${line}" || "${line}" =~ ^[[:space:]]*# ]]; then
			continue
		fi
		# Trim leading spaces.
		line="${line#"${line%%[![:space:]]*}"}"
		if [[ "${line}" == "${key}="* ]]; then
			local val="${line#${key}=}"
			# Strip matching quotes.
			if [[ "${val}" =~ ^\".*\"$ || "${val}" =~ ^\'.*\'$ ]]; then
				val="${val:1:${#val}-2}"
			fi
			printf '%s' "${val}"
			break
		fi
	done <<<"${content}"
	return 0
}

restart_service() {
	local svc="$1"
	local cmds=(
		"sudo -n systemctl restart \"${svc}\""
		"sudo systemctl restart \"${svc}\""
		"systemctl restart \"${svc}\""
	)
	for c in "${cmds[@]}"; do
		if run_cmd "${c}"; then
			return 0
		fi
	done
	log "failed to restart service ${svc}"
	exit 1
}

# Populate from caller (brev CLI) via string substitution.
ENV_CONTENT=$(cat <<'EOF'
%s
EOF
)

log "enroll start"
log "probe: $(uname -a)"
log "user: $(whoami)"
log "host: $(hostname)"
log "env file target: ${ENV_FILE}"

ensure_dir "$(dirname "${ENV_FILE}")"
write_file "${ENV_CONTENT}" "${ENV_FILE}"
set_file_mode 600 "${ENV_FILE}"

state_dir="$(extract_env_value "BREV_AGENT_STATE_DIR" "${ENV_CONTENT}")"
device_token_path="$(extract_env_value "BREV_AGENT_DEVICE_TOKEN_PATH" "${ENV_CONTENT}")"

if [[ -n "${state_dir}" ]]; then
	log "ensuring state dir ${state_dir}"
	ensure_dir "${state_dir}"
fi

if [[ -n "${device_token_path}" ]]; then
	log "ensuring device token parent dir for ${device_token_path}"
	ensure_dir "$(dirname "${device_token_path}")"
elif [[ -n "${state_dir}" ]]; then
	# Default device token lives under the state dir.
	ensure_dir "${state_dir}"
fi

if [[ "${MOCK}" == "true" ]]; then
	log "mock mode: skipping agent restart"
	exit 0
fi

restart_service "${SERVICE}"
log "enroll success"

