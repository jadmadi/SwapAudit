#!/usr/bin/env bash
# Fixture tests for scripts/check-contacts.sh.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHECK="${ROOT}/scripts/check-contacts.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

make_fixture() {
    local dir="$1"
    mkdir -p "${dir}/docs/decisions" "${dir}/.github/ISSUE_TEMPLATE"
    cat > "${dir}/docs/decisions/0002-security-contact.md" <<'EOF'
# Decision
<!-- contact-sentry
canonical_email: contact@jadmadi.net
canonical_repo: jadmadi/SwapAudit
canonical_security_url: https://github.com/jadmadi/SwapAudit/security/advisories/new
-->
EOF
    printf '# Security\ncontact@jadmadi.net\nhttps://github.com/jadmadi/SwapAudit/security/advisories/new\n' > "${dir}/SECURITY.md"
    printf '# Maintainers\ncontact@jadmadi.net\n' > "${dir}/MAINTAINERS.md"
    printf '# Support\ncontact@jadmadi.net\n' > "${dir}/SUPPORT.md"
    printf '# Conduct\ncontact@jadmadi.net\n' > "${dir}/CODE_OF_CONDUCT.md"
    printf 'contact_links:\n  - url: https://github.com/jadmadi/SwapAudit/security/advisories/new\n' > "${dir}/.github/ISSUE_TEMPLATE/config.yml"
    printf '# PR\n' > "${dir}/.github/PULL_REQUEST_TEMPLATE.md"
}

PASS_DIR="${TMP}/pass"
FAIL_DIR="${TMP}/fail"
MISSING_DIR="${TMP}/missing"
make_fixture "${PASS_DIR}"
cp -R "${PASS_DIR}" "${FAIL_DIR}"
printf '# Maintainers\n' > "${FAIL_DIR}/MAINTAINERS.md"
mkdir -p "${MISSING_DIR}"

if bash "${CHECK}" "${PASS_DIR}" >/dev/null; then
    echo "PASS: consistent fixture"
else
    echo "FAIL: consistent fixture was rejected" >&2
    exit 1
fi

if bash "${CHECK}" "${FAIL_DIR}" >/dev/null 2>&1; then
    echo "FAIL: inconsistent fixture was accepted" >&2
    exit 1
else
    echo "PASS: inconsistent fixture rejected"
fi

if bash "${CHECK}" "${MISSING_DIR}" >/dev/null 2>&1; then
    echo "FAIL: missing decision fixture was accepted" >&2
    exit 1
else
    echo "PASS: missing decision fixture rejected"
fi

echo "Contact sentry fixtures passed."
