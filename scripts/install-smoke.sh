#!/usr/bin/env bash
# Install and smoke-test the released amd64 binary.
# Usage: ./scripts/install-smoke.sh [version]
# Env: SWAPAUDIT_BINARY=/path/to/binary to test a local build instead of downloading.

set -euo pipefail

VERSION="${1:-v0.1.0}"
REPO="${SWAPAUDIT_REPO:-jadmadi/SwapAudit}"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

BIN="${TMP}/swapaudit-linux-amd64"

if [[ -n "${SWAPAUDIT_BINARY:-}" ]]; then
    if [[ ! -x "${SWAPAUDIT_BINARY}" ]]; then
        echo "SWAPAUDIT_BINARY is not executable: ${SWAPAUDIT_BINARY}" >&2
        exit 2
    fi
    cp "${SWAPAUDIT_BINARY}" "${BIN}"
    chmod +x "${BIN}"
    echo "Using local binary: ${SWAPAUDIT_BINARY}"
else
    if ! command -v gh >/dev/null 2>&1; then
        echo "gh is required to download a private release" >&2
        exit 2
    fi
    if ! gh auth status >/dev/null 2>&1; then
        echo "gh is not authenticated; run 'gh auth login' or set SWAPAUDIT_BINARY" >&2
        exit 2
    fi
    gh release download "${VERSION}" --repo "${REPO}" --dir "${TMP}" \
        --pattern 'swapaudit-linux-amd64' --pattern 'checksums.txt'
    chmod +x "${BIN}"
    (cd "${TMP}" && sha256sum -c checksums.txt --ignore-missing)
fi

"${BIN}" --version
"${BIN}" --dry-run --no-color --ascii --limit 1
echo "Install smoke passed for ${VERSION}."
