# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Read-only `--json` mode emitting a versioned, canonically ordered machine-readable snapshot.
- `--no-command` privacy flag that omits command lines from JSON output.
- `docs/json-schema.md` contract with nullability rules, ordering guarantees, and consumer recipes.
- Interactive four-tier action picker with review panels and typed scope-token confirmation.
- `--interactive` / `--no-interactive` flags and `SWAPAUDIT_NO_INTERACTIVE` for picker control.
- Measured swap/tmpfs before-and-after line after each picker action.
- `sample` subcommand with `--once` and `--stream` NDJSON output for swap and tmpfs.
- OLS rate estimator with sample, span, R², monotonicity, and direction gating for a nullable ETA.
- Bounded sampling ring with `(pid, start_ticks)` identity and exited/started attribution.
- Optional `--record` NDJSON traces at mode `0600` with a replay helper.
- `docs/sampling.md` with lanes, estimator gates, attribution caveats, and recipes.
- LimitReader bounds (50MB binary, 1MB checksum) to protect self-updater against unbounded streams.
- UID-scoped fallback paths for temporary update-check state caches.
- Interactive reader edge-case and live temporary pruning unit test coverage.
- Deep nested directory modification time propagation and symlink exclusion tests for cache inspection.
- Malformed proc stat and scheduler line parsing unit tests.

### Changed
- Enforced fail-closed temporary file creation in build script.

## [0.1.0] - 2026-10-19

### Added
- Modular layered Go architecture with `cmd/swapaudit` and clean `internal/` packages.
- Accurate kernel memory auditing parsing `/proc/meminfo`, `/proc/[pid]/status`, and `/proc/[pid]/sched`.
- PID reuse protection via kernel start-ticks verification before SIGTERM dispatch.
- Tmpfs cache inspection in `/tmp` with symlink guards, UID verification, and mtime depth analysis.
- Responsive terminal table engine supporting automatic column dropping, ASCII mode, and NO_COLOR.
- Build tooling including GoReleaser v2 configuration, multi-mode build script, and Makefile.
- Continuous integration workflows for automated race detection, linting, and cross-compilation.
- Dual-licensing under Waqf Digital Public License (WaqfDPL-Mulzim 1.0) and GNU AGPLv3.
