#!/usr/bin/env bash
# Captures deterministic CLI outputs for windows-WSL vs linux parity comparison.
# Usage: BREV_BIN=/path/to/brev scripts/parity-capture.sh outdir
# Requires: compiled brev binary with no credentials (unauthenticated outputs only).
set -euo pipefail

BREV_BIN="${BREV_BIN:?set BREV_BIN to the compiled brev binary}"
OUTDIR="${1:?usage: parity-capture.sh outdir}"
# Fresh HOME so local credentials/org state never leak into captures.
CLEAN_HOME="$(mktemp -d)"
export HOME="$CLEAN_HOME"
mkdir -p "$OUTDIR"

# Each entry: name<TAB>shell snippet. Snippets run with $BREV set to the binary.
# Keep outputs deterministic and auth-free: --help/--version/table headers,
# error messages, exit codes. Table *data* requires auth and is out of scope.
declare -a CASES=(
  "version	\$BREV --version"
  "help	\$BREV --help"
  "ls_help	\$BREV ls --help"
  "org_ls_help	\$BREV org ls --help"
  "search_help	\$BREV search --help"
  "shell_help	\$BREV shell --help"
  "exec_help	\$BREV exec --help"
  "ports_help	\$BREV ports --help"
  "start_help	\$BREV start --help"
  "stop_help	\$BREV stop --help"
  "unknown_cmd	\$BREV definitely-not-a-command"
  "ls_no_auth	echo n | \$BREV ls"
  "org_ls_no_auth	echo n | \$BREV org ls"
)

for entry in "${CASES[@]}"; do
  name="${entry%%$'\t'*}"
  snippet="${entry#*$'\t'}"
  # shellcheck disable=SC2016  # snippet must not expand at parse time
  out="$(BREV="$BREV_BIN" bash -c "$snippet" 2>&1 || true)"
  code="$(BREV="$BREV_BIN" bash -c "$snippet" >/dev/null 2>&1; echo $?)"
  # Normalize environment noise:
  # - windows/WSL paths and versions that legitimately differ across hosts
  # - timings/durations if any leak into output
  # - remote "New Version" upgrade banner (network + release timing dependent)
  norm="$(printf '%s' "$out" \
    | sed -E 's/[0-9]+\.[0-9]+[smµ]+(s)?[0-9]*/<DURATION>/g' \
    | sed -E 's#/mnt/c/[^\r\n ]*#<WINPATH>#g' \
    | sed -E 's#[A-Za-z]:\\\\[^\r\n ]*#<WINPATH>#g' \
    | sed -E 's#(/usr/local|/home|/Users)/[^\r\n ]*go[^\r\n ]*#<GOPATH>#g' \
    | sed -E '/A new version of brev has been released/,+5d' \
    | sed -E '/^[0-9]{4}\/[0-9]{2}\/[0-9]{2}.*RESTY/d' \
    | sed -E 's/^EOF$//')"
  printf '%s\n--- exit=%s\n' "$norm" "$code" > "$OUTDIR/$name.txt"
done

echo "captured $(ls "$OUTDIR" | wc -l | tr -d ' ') cases into $OUTDIR"
