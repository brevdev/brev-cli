#!/usr/bin/env bash
set -euo pipefail

# Keep all installation work inside main so a truncated `curl | bash` download
# cannot execute a partial installer.

curl_brev_cli() {
    curl \
        --fail \
        --silent \
        --show-error \
        --location \
        --proto '=https' \
        --proto-redir '=https' \
        --connect-timeout 10 \
        "$@"
}

get_release_arch() {
    case "$(uname -m)" in
        x86_64|amd64) printf '%s\n' "amd64" ;;
        aarch64|arm64) printf '%s\n' "arm64" ;;
        *) return 1 ;;
    esac
}

get_latest_release_tag() {
    # Use GitHub's web redirect instead of its REST API so installations from
    # shared egress IPs are not subject to the unauthenticated API quota.
    local release_url release_tag
    release_url="$(curl_brev_cli --max-time 30 --output /dev/null --write-out '%{url_effective}' \
        "https://github.com/brevdev/brev-cli/releases/latest")" || return 1
    release_tag="${release_url##*/}"
    [[ "$release_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || return 1
    printf '%s\n' "$release_tag"
}

main() {
    local os release_arch release_tag release_version archive_name archive_path install_dir

    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$os" in
        darwin|linux) ;;
        *)
            printf 'Error: Unsupported operating system: %s\n' "$os" >&2
            return 1
            ;;
    esac

    if ! release_arch="$(get_release_arch)"; then
        printf 'Error: Unsupported architecture: %s\n' "$(uname -m)" >&2
        return 1
    fi

    TMP_DIR="$(mktemp -d)"
    trap 'rm -rf -- "$TMP_DIR"' EXIT

    if ! release_tag="$(get_latest_release_tag)"; then
        printf 'Error: Could not determine the latest Brev CLI release.\n' >&2
        return 1
    fi

    release_version="${release_tag#v}"
    archive_name="brev-cli_${release_version}_${os}_${release_arch}.tar.gz"
    archive_path="${TMP_DIR}/${archive_name}"

    # The archive has no total time limit; slow but progressing downloads may
    # take as long as needed.
    curl_brev_cli \
        --output "$archive_path" \
        "https://github.com/brevdev/brev-cli/releases/download/${release_tag}/${archive_name}"
    tar -xzf "$archive_path" -C "$TMP_DIR" -- brev

    if [[ ! -f "${TMP_DIR}/brev" || -L "${TMP_DIR}/brev" ]]; then
        printf 'Error: Release archive did not contain a regular brev executable.\n' >&2
        return 1
    fi

    install_dir="${BREV_INSTALL_DIR:-${HOME}/.local/bin}"
    mkdir -p "$install_dir"
    mv "${TMP_DIR}/brev" "${install_dir}/brev"
    chmod 0755 "${install_dir}/brev"

    printf 'Successfully installed Brev CLI to %s/brev\n' "$install_dir"

    case ":${PATH}:" in
        *":${install_dir}:"*) ;;
        *)
            printf '\nWarning: %s is not in your PATH.\n' "$install_dir" >&2
            printf 'Add it to your shell profile, then restart your shell:\n' >&2
            printf '    export PATH="%s:%s"\n' "$install_dir" "\$PATH" >&2
            ;;
    esac
}

TMP_DIR=""
main "$@"
