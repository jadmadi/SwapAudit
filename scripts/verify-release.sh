#!/usr/bin/env bash
# Verify a published SwapAudit release over the public internet.
# Usage: ./scripts/verify-release.sh [version]
# Env: SWAPAUDIT_REPO (default: jadmadi/SwapAudit), SKIP_GO_INSTALL=1, SKIP_SIGNATURE=1

set -euo pipefail

VERSION="${1:-v0.1.0}"
REPO="${SWAPAUDIT_REPO:-jadmadi/SwapAudit}"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"
API="https://api.github.com/repos/${REPO}/releases/tags/${VERSION}"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required" >&2
    exit 2
fi

EXPECTED=(
    swapaudit-linux-amd64
    swapaudit-linux-arm64
    swapaudit-linux-armv7
    swapaudit-linux-riscv64
)

curl -fsSL "${API}" -o "${TMP}/release.json"
for name in "${EXPECTED[@]}"; do
    code=$(curl -sL -o /dev/null -w '%{http_code}' "${BASE}/${name}")
    if [[ "${code}" != "200" ]]; then
        echo "FAIL: ${name} returned HTTP ${code}" >&2
        exit 1
    fi
done
echo "PASS: four architecture assets are publicly reachable"

for name in "${EXPECTED[@]}" checksums.txt checksums.txt.sig checksums.txt.pem; do
    curl -fsSL "${BASE}/${name}" -o "${TMP}/${name}"
done

(
    cd "${TMP}"
    sha256sum -c checksums.txt --ignore-missing
)
echo "PASS: checksums verified"

chmod +x "${TMP}/swapaudit-linux-amd64"
VERSION_OUTPUT=$("${TMP}/swapaudit-linux-amd64" --version)
VERSION_STRIPPED="${VERSION#v}"
if ! grep -Fq "${VERSION}" <<<"${VERSION_OUTPUT}" && ! grep -Fq "${VERSION_STRIPPED}" <<<"${VERSION_OUTPUT}"; then
    echo "FAIL: version output does not contain ${VERSION} or ${VERSION_STRIPPED}: ${VERSION_OUTPUT}" >&2
    exit 1
fi
echo "PASS: amd64 binary reports ${VERSION_STRIPPED}"

if [[ "${SKIP_SIGNATURE:-0}" != "1" ]]; then
    if [[ ! -f "${TMP}/checksums.txt.sig" || ! -f "${TMP}/checksums.txt.pem" ]]; then
        echo "FAIL: signature assets are missing" >&2
        exit 1
    fi
    if command -v cosign >/dev/null 2>&1; then
        cosign verify-blob \
            --certificate "${TMP}/checksums.txt.pem" \
            --signature "${TMP}/checksums.txt.sig" \
            --certificate-identity-regexp "https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/${VERSION}" \
            --certificate-oidc-issuer https://token.actions.githubusercontent.com \
            "${TMP}/checksums.txt"
        echo "PASS: signature verified"
    else
        echo "WARN: cosign not installed; signature assets exist but were not verified"
    fi
fi

if [[ "${SKIP_GO_INSTALL:-0}" != "1" ]]; then
    if command -v go >/dev/null 2>&1; then
        GOBIN="$(mktemp -d)"
        VERSION_PKG="github.com/jadmadi/swapaudit/internal/version"
        GOPROXY=https://proxy.golang.org go install \
            -ldflags="-X ${VERSION_PKG}.Version=${VERSION} -X ${VERSION_PKG}.Commit=release -X ${VERSION_PKG}.Date=unknown" \
            "github.com/jadmadi/swapaudit/cmd/swapaudit@${VERSION}"
        "${GOBIN}/swapaudit" --version | grep -F "${VERSION_STRIPPED}"
        echo "PASS: go install @${VERSION}"
    else
        echo "WARN: go unavailable; skipped go install"
    fi
fi

echo "Release verification passed for ${VERSION}."
