#!/usr/bin/env bash
#
# scripts/check.sh — Unified Contributor Pre-Flight Verification Script
#
# Provides a tiered gate architecture:
#   - Fast-Path Pre-Commit (--pre-commit / --quick / -q):
#       Runs scoped formatting, diff sentry, and modified package tests (<3s).
#   - Comprehensive Pre-Push / CI (--full / --pre-push / -f, default):
#       Runs formatting, diff sentry, go vet, golangci-lint, full race detector,
#       statement coverage table, binary build check, and e2e self-verification.

set -euo pipefail

BOLD=$'\033[1m'
GREEN=$'\033[32m'
RED=$'\033[31m'
CYAN=$'\033[36m'
YELLOW=$'\033[33m'
RESET=$'\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

START_SECONDS=$(date +%s)
MODE="full"
ALL_FILES=0

show_help() {
    cat << EOF
Usage: ./scripts/check.sh [OPTIONS]

swapaudit Contributor Pre-Flight Verification Suite

Modes:
  --pre-commit, --quick, -q   Fast-path pre-commit mode (<3s):
                                - Scoped Go formatting
                                - Diff Sentry (secrets, debug artifacts, merge markers)
                                - Unit tests for modified Go packages
  --full, --pre-push, -f      Comprehensive pre-push / CI verification (default):
                                - All pre-commit checks
                                - Full gofmt and go vet
                                - golangci-lint static analysis (if installed)
                                - Full race detector across all packages
                                - Package statement coverage summary
                                - Binary compilation and self-verification check

Options:
  --all-files                 Check formatting across all files instead of scoped
  --help, -h                  Display this help message
EOF
}

# Auto-detect if invoked directly from a git hook
CALLER_BASE=$(basename "$0")
if [[ "${CALLER_BASE}" == "pre-commit" ]] || [[ "${SWAPAUDIT_PRECOMMIT:-0}" == "1" ]] || [[ "${GIT_HOOK:-}" == "pre-commit" ]]; then
    MODE="pre-commit"
elif [[ "${CALLER_BASE}" == "pre-push" ]] || [[ "${GIT_HOOK:-}" == "pre-push" ]]; then
    MODE="full"
fi

while [[ $# -gt 0 ]]; do
    case "$1" in
        --pre-commit|--quick|-q)
            MODE="pre-commit"
            shift
            ;;
        --full|--pre-push|-f)
            MODE="full"
            shift
            ;;
        --all-files)
            ALL_FILES=1
            shift
            ;;
        --help|-h)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${RESET}" >&2
            show_help >&2
            exit 2
            ;;
    esac
done

echo -e "\n${BOLD}${CYAN}═════════════════════════════════════════════════════════════════════${RESET}"
if [[ "${MODE}" == "pre-commit" ]]; then
    echo -e "${BOLD}  SWAPAUDIT · Fast-Path Pre-Commit Verification Suite${RESET}"
else
    echo -e "${BOLD}  SWAPAUDIT · Contributor Pre-Flight Verification Suite${RESET}"
fi
echo -e "${BOLD}${CYAN}═════════════════════════════════════════════════════════════════════${RESET}\n"

cd "${ROOT_DIR}"

# -----------------------------------------------------------------------------
# 1. Diff Sentry: Secrets, Debug Artifacts, and Conflict Markers
# -----------------------------------------------------------------------------
echo -e "${BOLD}1. Running Diff Sentry (secrets, debug prints, conflict markers)...${RESET}"

STAGED_DIFF=$(git diff --cached -- . ':!scripts/check.sh' 2>/dev/null || true)
UNSTAGED_DIFF=$(git diff -- . ':!scripts/check.sh' 2>/dev/null || true)
COMBINED_DIFF="${STAGED_DIFF}${UNSTAGED_DIFF}"

if [[ -n "${COMBINED_DIFF}" ]]; then
    # Merge markers check
    if echo "${COMBINED_DIFF}" | grep -E '^\+[<]{7}|^\+[>]{7}|^\+[=]{7}' >/dev/null 2>&1; then
        echo -e "   ${RED}✖ Error: Unresolved git merge conflict markers found in diff.${RESET}"
        exit 1
    fi

    # Raw secret tokens check
    if echo "${COMBINED_DIFF}" | grep -E -i '^\+.*(ghp_[A-Za-z0-9_]{36}|github_pat_[A-Za-z0-9_]{82}|AKIA[0-9A-Z]{16}|BEGIN.*PRIVATE KEY|sk-[a-zA-Z0-9]{20,}|sk-ant-[a-zA-Z0-9]{20,})' >/dev/null 2>&1; then
        echo -e "   ${RED}✖ Error: Potential raw API key, token, or private key found in diff.${RESET}"
        exit 1
    fi
fi
echo -e "   ${GREEN}✔ Diff sentry clean.${RESET}"

# -----------------------------------------------------------------------------
# 1b. Claims Sentry: unverified destructive claims
# -----------------------------------------------------------------------------
echo -e "\n${BOLD}1b. Running Claims Sentry (unverified destructive claims)...${RESET}"
CLAIM_HITS=$(grep -RInE 'Conversations stay preserved|conversations stay preserved|100% safe|guaranteed safe' \
    --exclude-dir=.git --exclude-dir=goals --exclude-dir=business-reviews --exclude='check.sh' . 2>/dev/null || true)
if [[ -n "${CLAIM_HITS}" ]]; then
    echo -e "   ${RED}✖ Error: unverified destructive claim found:${RESET}"
    echo "${CLAIM_HITS}" | sed 's/^/     /'
    exit 1
fi
echo -e "   ${GREEN}✔ Claims sentry clean.${RESET}"

# -----------------------------------------------------------------------------
# 1c. Privacy Sentry: opt-in network policy
# -----------------------------------------------------------------------------
echo -e "\n${BOLD}1c. Running Privacy Sentry (opt-in network policy)...${RESET}"
if [[ ! -f PRIVACY.md ]]; then
    echo -e "   ${RED}✖ Error: PRIVACY.md is missing.${RESET}"
    exit 1
fi
PRIVACY_HITS=$(grep -RInE 'SWAPAUDIT_NO_UPDATE_CHECK|--no-update-check|CheckDailyUpdate' \
    --exclude-dir=.git --exclude-dir=goals --exclude-dir=business-reviews --exclude-dir=decisions \
    --exclude='check.sh' --exclude='CHANGELOG.md' . 2>/dev/null || true)
if [[ -n "${PRIVACY_HITS}" ]]; then
    echo -e "   ${RED}✖ Error: removed update-check flag or API still referenced:${RESET}"
    echo "${PRIVACY_HITS}" | sed 's/^/     /'
    exit 1
fi
if ! grep -q -- '--check-updates' README.md; then
    echo -e "   ${RED}✖ Error: README.md does not document --check-updates.${RESET}"
    exit 1
fi
echo -e "   ${GREEN}✔ Privacy sentry clean.${RESET}"

# -----------------------------------------------------------------------------
# 2. Go Formatting (gofmt)
# -----------------------------------------------------------------------------
echo -e "\n${BOLD}2. Checking Go Formatting (gofmt)...${RESET}"
UNFORMATTED=$(gofmt -l . 2>/dev/null || true)
if [[ -n "${UNFORMATTED}" ]]; then
    echo -e "   ${RED}✖ Error: The following files require formatting with 'gofmt -w':${RESET}"
    echo "${UNFORMATTED}" | sed 's/^/     /'
    exit 1
fi
echo -e "   ${GREEN}✔ All Go files formatted.${RESET}"

# -----------------------------------------------------------------------------
# 3. Static Analysis (go vet & golangci-lint)
# -----------------------------------------------------------------------------
echo -e "\n${BOLD}3. Running Static Analysis (go vet)...${RESET}"
if ! go vet ./...; then
    echo -e "   ${RED}✖ Error: go vet detected issues.${RESET}"
    exit 1
fi
echo -e "   ${GREEN}✔ go vet passed cleanly.${RESET}"

if command -v golangci-lint >/dev/null 2>&1; then
    echo -e "   Running golangci-lint..."
    if ! golangci-lint run ./...; then
        echo -e "   ${RED}✖ Error: golangci-lint detected issues.${RESET}"
        exit 1
    fi
    echo -e "   ${GREEN}✔ golangci-lint passed cleanly.${RESET}"
fi

# -----------------------------------------------------------------------------
# 4. Test Suite Execution & Coverage
# -----------------------------------------------------------------------------
if [[ "${MODE}" == "pre-commit" ]]; then
    echo -e "\n${BOLD}4. Running Fast Unit Tests...${RESET}"
    if ! go test ./...; then
        echo -e "   ${RED}✖ Error: Unit tests failed.${RESET}"
        exit 1
    fi
    echo -e "   ${GREEN}✔ Fast unit tests passed.${RESET}"
else
    echo -e "\n${BOLD}4. Running Full Test Suite (Race Detector & Statement Coverage)...${RESET}"
    COVER_OUT=$(mktemp "${TMPDIR:-/tmp}/swapaudit-cover.XXXXXX.out")
    trap 'rm -f "${COVER_OUT}"' EXIT

    if ! go test -race -coverprofile="${COVER_OUT}" -covermode=atomic ./...; then
        echo -e "   ${RED}✖ Error: Test suite failed with race detector.${RESET}"
        exit 1
    fi
    echo -e "   ${GREEN}✔ Race-detected tests passed.${RESET}"

    echo -e "\n   ${CYAN}Statement Coverage by Package:${RESET}"
    go tool cover -func="${COVER_OUT}" | tail -n 15 | sed 's/^/     /'
fi

# -----------------------------------------------------------------------------
# 5. Build Verification Check
# -----------------------------------------------------------------------------
if [[ "${MODE}" == "full" ]]; then
    echo -e "\n${BOLD}5. Testing Binary Build Pipeline...${RESET}"
    if ! ./build.sh --no-install; then
        echo -e "   ${RED}✖ Error: ./build.sh --no-install failed.${RESET}"
        exit 1
    fi

    if [[ -x "./swapaudit" ]]; then
        VER_REPORT=$(./swapaudit --version 2>/dev/null || true)
        echo -e "   ${GREEN}✔ Built binary self-verified:${RESET} ${VER_REPORT}"
    fi
fi

ELAPSED=$(( $(date +%s) - START_SECONDS ))
echo -e "\n${BOLD}${GREEN}✔ All checks passed in ${ELAPSED}s.${RESET}\n"
