# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `internal/report` accounting model with exact-byte attribution, explicit `unattributed`/`overlap` residuals, per-pool reclaim, category shares, and population counts.
- Allocated-block cache sizing (`Stat_t.Blocks * 512`); apparent size is retained for JSON and diagnostics.
- Cache liveness classification: `/proc` open-handle/cwd/mapping index, `in-use-caches`, `recent-caches`, and opt-in `orphaned-caches`.
- `--orphan-cache-age`, `--clean-orphaned-caches`, `--include-orphaned-caches`, `--allow-generic-caches`, and `--explain-caches`.
- Full process scan with zero-swap agent visibility, `AgentPopulation`, `--explain-agents`, and `--allow-live-parent`.
- Parent-liveness, orphan-evidence, idle-evidence, UID, and start-ticks rechecks immediately before `SIGTERM`; the signal is printed in dry-run and live output.
- One-line verdict, middle truncation with suffix preservation, short IDs, pattern grouping, `--expand`, `--collapse`, `--no-group`, and `--wide`/`--no-truncate`.
- Versioned `--json` audit and cleanup contract (`schema_version: 1`) with golden and determinism tests.
- Exit-code contract: `0` success, `1` runtime, `2` usage, `3` cleanup partial failure.
- Non-interactive cleanup guard: cleanup without `--dry-run`/`--yes` on a non-TTY stdin exits `2`.
- `docs/metrics.md`, `docs/cache-liveness.md`, `docs/output-layout.md`, and `docs/json-schema.md`.
- Label sentry for retired metric names.

### Changed
- `active-caches` is now `recent-caches`; it never claimed liveness, only recency.
- `Process Swap (RAM)` is now `Process swap (VmSwap)`; `/tmp Caches (tmpfs)` is now `/tmp allocated (tmpfs)`.
- The Actionable and Protected tables are disjoint; cleanable rows appear exactly once.
- `--all` disables shell aggregation, grouping, and the 10 MB filter; `--limit 0` means unlimited.
- Reclaimable bytes are shown per pool (`tmpfs` and `swap`); a combined figure carries `upper bound; pools may overlap`.
- `--clean-all` still excludes orphaned agents and now also excludes orphaned caches.
- Non-TTY cleanup behavior is explicit instead of silently declining every group.

### Fixed
- Header totals no longer disagree with category sums; every residual is rendered.
- Attributed bytes exceeding the system total produce `overlap`, never a negative residual.
- Prune footers no longer count skipped targets as hidden; hidden bytes and counts match.
- `abandoned-agents` and `orphaned-caches` rows render at zero instead of disappearing.
- Zero-swap orphan/dormant agents are no longer invisible to the audit and cleanup path.
- Cache sizes no longer overstate reclaimable bytes for sparse files.
- Shared-prefix cleanup targets keep their distinguishing suffix.

### Added (previous)
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
