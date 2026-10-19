# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `--all-users` opt-in for signaling processes owned by another UID.
- `--include-orphaned` opt-in for including orphaned-agent cleanup with `--clean-all`.
- `--check-updates` and `SWAPAUDIT_CHECK_UPDATES=1` opt-in for one explicit release check.
- `PRIVACY.md` and `docs/decisions/0001-update-check-policy.md`.
- `SUPPORT.md`, issue templates, PR template, and `docs/decisions/0002-security-contact.md`.
- Contact-consistency sentry with pass/fail fixtures.
- UID, tty, and cgroup evidence parsing plus a fail-closed orphan classifier.
- One-line cleanup summary with counts and skip reasons.
- `docs/safety.md` operator safety contract and manual verification protocol.
- LimitReader bounds (50MB binary, 1MB checksum) to protect self-updater against unbounded streams.
- UID-scoped fallback paths for temporary update-check state caches.
- Interactive reader edge-case and live temporary pruning unit test coverage.
- Deep nested directory modification time propagation and symlink exclusion tests for cache inspection.
- Malformed proc stat and scheduler line parsing unit tests.

### Changed
- `--clean-all` no longer includes orphaned cleanup; orphan cleanup is always explicit.
- Dormant-cleanup prompt now warns that `SIGTERM` may lose unsaved in-memory state instead of claiming conversations are preserved.
- Default audit is network-silent; update checks are opt-in.
- Removed the 150 ms background update-check race and the automatic 24-hour cadence.
- Removed `--no-update-check` and `SWAPAUDIT_NO_UPDATE_CHECK`; the default is silent.
- Enforced fail-closed temporary file creation in build script.

### Security
- Consolidated security, support, and conduct contacts on `contact@jadmadi.net` with GitHub private advisories as the primary security channel.
- Removed the conflicting `jad@madi.me` security contact from the published policy.

### Fixed
- False-positive orphan classification for systemd-managed agents.
- Cross-user process signaling when running as root without `--all-users`.
- Unverified conversation-preservation claim in the destructive cleanup prompt.
- Update state now preserves the last known version and URL after a failed explicit check.

## [0.1.0] - 2026-10-19

### Release
- First tagged release in the private repository.
- Four Linux assets: `swapaudit-linux-amd64`, `swapaudit-linux-arm64`, `swapaudit-linux-armv7`, `swapaudit-linux-riscv64`.
- SHA256 checksums (`checksums.txt`) and cosign keyless signatures (`checksums.txt.sig`, `checksums.txt.pem`).
- ARMv7 asset name fixed to match the updater contract.
- Uncompressed binaries with a reproducible amd64 build.

### Added
- Modular layered Go architecture with `cmd/swapaudit` and clean `internal/` packages.
- Accurate kernel memory auditing parsing `/proc/meminfo`, `/proc/[pid]/status`, and `/proc/[pid]/sched`.
- PID reuse protection via kernel start-ticks verification before SIGTERM dispatch.
- Tmpfs cache inspection in `/tmp` with symlink guards, UID verification, and mtime depth analysis.
- Responsive terminal table engine supporting automatic column dropping, ASCII mode, and NO_COLOR.
- Build tooling including GoReleaser v2 configuration, multi-mode build script, and Makefile.
- Continuous integration workflows for automated race detection, linting, and cross-compilation.
- Dual-licensing under Waqf Digital Public License (WaqfDPL-Mulzim 1.0) and GNU AGPLv3.

### Known limitations
- Linux only; uses `/proc` and Linux cgroups.
- RAM used relies on `MemAvailable` (Linux 3.14+).
- Scans `/tmp`, not `/var/tmp`.
- Agent detection covers `agy`, `devin`, `chrome-devtools`, and `antigravity`.
- No JSON output, package-manager package, GUI, or daemon.
- The repository is private; unauthenticated install and self-upgrade are not available until publication.
