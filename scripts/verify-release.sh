#!/usr/bin/env bash
# Verify a published SwapAudit release: asset names, checksums, signature, binary.
# Usage: ./scripts/verify-release.sh [version]
# Env: SWAPAUDIT_REPO (default: jadmadi/SwapAudit), SKIP_GO_INSTALL=1, SKIP_SIGNATURE=1

set -euo pipefail

VERSION="${1:-v0.1.0}"
REPO="${SWAPAUDIT_REPO:-jadmadi/SwapAudit}"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

if ! command -v gh >/dev/null 2>&1; then
    echo "gh is required" >&2
    exit 2
fi

EXPECTED=(
    swapaudit-linux-amd64
    swapaudit-linux-arm64
    swapaudit-linux-armv7
    swapaudit-linux-riscv64
)

ASSETS=$(gh release view "${VERSION}" --repo "${REPO}" --json assets --jq '.assets[].name' | sort)
for name in "${EXPECTED[@]}"; do
    if ! grep -qx "${name}" <<<"${ASSETS}"; then
        echo "FAIL: release is missing ${name}" >&2
        echo "${ASSETS}" >&2
        exit 1
    fi
done
echo "PASS: four architecture assets present"

DOWNLOAD_PATTERNS=(
    --pattern 'swapaudit-linux-*'
    --pattern 'checksums.txt'
    --pattern 'checksums.txt.sig'
    --pattern 'checksums.txt.pem'
)
gh release download "${VERSION}" --repo "${REPO}" --dir "${TMP}" "${DOWNLOAD_PATTERNS[@]}"

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
    if command -v go >/dev/null 2>&1 && command -v gh >/dev/null 2>&1; then
        export GH_TOKEN="${GH_TOKEN:-$(gh auth token)}"
        export GOPRIVATE="github.com/jadmadi/*"
        export GONOSUMDB="github.com/jadmadi/*"
        export GOPROXY=direct
        export GIT_CONFIG_COUNT=1
        export GIT_CONFIG_KEY_0=credential.helper
        export GIT_CONFIG_VALUE_0='!gh auth git-credential'
        GOBIN="$(mktemp -d)"
        VERSION_PKG="github.com/jadmadi/swapaudit/internal/version"
        go install -ldflags="-X ${VERSION_PKG}.Version=${VERSION} -X ${VERSION_PKG}.Commit=release -X ${VERSION_PKG}.Date=unknown" \
            "github.com/jadmadi/swapaudit/cmd/swapaudit@${VERSION}"
        "${GOBIN}/swapaudit" --version | grep -F "${VERSION_STRIPPED}"
        echo "PASS: go install @${VERSION}"
    else
        echo "WARN: go or gh unavailable; skipped go install"
    fi
fi

echo "Release verification passed for ${VERSION}."
