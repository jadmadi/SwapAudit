# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-11-02

### Added
- `swapaudit reclaim` with a guarded `swapoff`/`swapon` drain that empties swap
  without rebooting and a `--drop-caches` page-cache reclaim.
- Pure reclaim planner with `MemAvailable - used swap - reserve` feasibility,
  per-device decisions, zram detection, and a default reserve of
  `max(512 MiB, 10% of RAM)`.
- Per-device restore: every swap area is re-enabled before the next is disabled;
  a failed restore exits `3` with the exact `sudo swapon` recovery command.
- Partial drains: when all swap does not fit the RAM budget, the planner selects
  the largest safe subset, reports the bytes left in swap, and executes only the
  selected devices.
- `reclaim --dry-run`, `--yes`, `--json`, `--include-zram`, `--reserve`, and
  `--timeout` with a measured before/after receipt.
- No-Reboot Reclaim Readiness block in the main report with feasibility,
  headroom, swap devices, page-cache estimate, and PSI averages.
- `/proc/swaps` per-device parser and `/proc/pressure/memory` parser.
- Page-cache counters (`Buffers`, `Cached`, `SReclaimable`, `Shmem`, `Dirty`,
  `Writeback`, `MemFree`) in the JSON snapshot.
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
- `watch` subcommand with sustain, hysteresis, cooldown, hourly cap, and escalation.
- Probed notification chain: notify-send, gdbus, dbus-send, kdialog, OSC 777, stderr.
- `--test-notify` transport probe and `--json` event NDJSON with no transport sends.
- `docs/watch.md` with the state diagram, payloads, fatigue tuning, and systemd-run recipe.
- LimitReader bounds (50MB binary, 1MB checksum) to protect self-updater against unbounded streams.
- UID-scoped fallback paths for temporary update-check state caches.
- Interactive reader edge-case and live temporary pruning unit test coverage.
- Deep nested directory modification time propagation and symlink exclusion tests for cache inspection.
- Malformed proc stat and scheduler line parsing unit tests.

### Changed
- Enforced fail-closed temporary file creation in build script.
- `--clean-all` now covers untouched caches, abandoned runs, and dormant agents
  only. Orphaned processes remain opt-in through `--clean-orphaned`.
- `/tmp` reclaim accounting now uses allocated blocks (`Stat_t.Blocks * 512`)
  for files and directories instead of apparent size.

### Fixed
- Process termination now re-reads the target's real UID and skips other-user
  processes, matching the documented ownership contract.
- Orphan detection now fails closed on `.service` cgroups and processes without
  a controlling terminal, so `ollama serve` and other system services are no
  longer classified as abandoned agents.
- `serve`, `--service`, `--stdio`, and generic `mcp` argv no longer mark every
  process as an AI agent.
- Cleanup without `--dry-run` or `--yes` now exits `2` before scanning when
  stdin is not a terminal.
- Update checks no longer advertise the base release tag for git-describe builds that are ahead of it.
- Watch shutdown now emits the stopped event with the correct reason through the sampler stop callback.

### Changed
- Refreshed goal certification records and audit receipts after repository
  maintenance.

### Removed
- Runtime detection and default tmp patterns for an unvetted third-party AI CLI.

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
