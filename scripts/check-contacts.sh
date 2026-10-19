#!/usr/bin/env bash
# Verify that security, support, and conduct contacts stay consistent.

set -euo pipefail

ROOT="${1:-.}"
if [[ ! -d "${ROOT}" ]]; then
    echo "usage: $0 [repo-root]" >&2
    exit 2
fi

DECISION="${ROOT}/docs/decisions/0002-security-contact.md"
if [[ ! -f "${DECISION}" ]]; then
    echo "FAIL: missing ${DECISION}" >&2
    exit 1
fi

canonical_email=$(sed -n 's/^canonical_email:[[:space:]]*//p' "${DECISION}" | head -1)
canonical_security_url=$(sed -n 's/^canonical_security_url:[[:space:]]*//p' "${DECISION}" | head -1)
canonical_repo=$(sed -n 's/^canonical_repo:[[:space:]]*//p' "${DECISION}" | head -1)

if [[ -z "${canonical_email}" || -z "${canonical_security_url}" || -z "${canonical_repo}" ]]; then
    echo "FAIL: contact-sentry header is incomplete in ${DECISION}" >&2
    exit 1
fi

FAIL=0

require_contains() {
    local file="$1" needle="$2" label="$3"
    if [[ ! -f "${file}" ]]; then
        echo "FAIL: ${label}: missing ${file}" >&2
        FAIL=1
        return
    fi
    if ! grep -Fq -- "${needle}" "${file}"; then
        echo "FAIL: ${label}: ${file} does not contain ${needle}" >&2
        FAIL=1
    fi
}

require_contains "${ROOT}/SECURITY.md" "${canonical_email}" "SECURITY.md email"
require_contains "${ROOT}/SECURITY.md" "${canonical_security_url}" "SECURITY.md advisory URL"
require_contains "${ROOT}/MAINTAINERS.md" "${canonical_email}" "MAINTAINERS.md email"
require_contains "${ROOT}/SUPPORT.md" "${canonical_email}" "SUPPORT.md email"
require_contains "${ROOT}/CODE_OF_CONDUCT.md" "${canonical_email}" "CODE_OF_CONDUCT.md email"
require_contains "${ROOT}/.github/ISSUE_TEMPLATE/config.yml" "${canonical_security_url}" "issue template advisory URL"

if [[ ! -f "${ROOT}/.github/PULL_REQUEST_TEMPLATE.md" ]]; then
    echo "FAIL: missing .github/PULL_REQUEST_TEMPLATE.md" >&2
    FAIL=1
fi

# The retired address may appear in the decision record's migration note,
# but not in the published support/security surface.
OLD_HITS=$(grep -RIn "jad@madi.me" "${ROOT}" \
    --exclude-dir=.git --exclude-dir=goals --exclude-dir=business-reviews \
    --exclude="0002-security-contact.md" --exclude="CHANGELOG.md" \
    --exclude="check-contacts.sh" 2>/dev/null || true)
if [[ -n "${OLD_HITS}" ]]; then
    echo "FAIL: retired security address still published:" >&2
    echo "${OLD_HITS}" | sed 's/^/  /' >&2
    FAIL=1
fi

if [[ "${FAIL}" -ne 0 ]]; then
    exit 1
fi

echo "Contact sentry clean: ${canonical_email}; ${canonical_security_url}"
