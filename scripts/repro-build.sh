#!/usr/bin/env bash
# Build linux/amd64 twice with fixed metadata and compare the hashes.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

OUT="$(mktemp -d)"
trap 'rm -rf "${OUT}"' EXIT

VERSION_PKG="github.com/jadmadi/swapaudit/internal/version"
DATE="2026-10-19T00:00:00Z"
LDFLAGS="-s -w -X ${VERSION_PKG}.Version=v0.1.0 -X ${VERSION_PKG}.Commit=repro -X ${VERSION_PKG}.Date=${DATE}"

build() {
    local out="$1"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -trimpath -ldflags="${LDFLAGS}" -o "${out}" ./cmd/swapaudit
}

build "${OUT}/first"
build "${OUT}/second"

FIRST=$(sha256sum "${OUT}/first" | awk '{print $1}')
SECOND=$(sha256sum "${OUT}/second" | awk '{print $1}')

if [[ "${FIRST}" != "${SECOND}" ]]; then
    echo "not reproducible: ${FIRST} != ${SECOND}" >&2
    exit 1
fi

echo "reproducible: yes (${FIRST})"
