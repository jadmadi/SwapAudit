# Facts: AI CLI VPS Coverage

## 1. Context & Motivation
- Target: Keep continuous AI CLI tools (`opencode`, `commandcode` / `cmd`, `muse`, `agy`) running on Linux VPS environments without RAM / tmpfs exhaustion or server reboots.
- Observed leaks in practice:
  1. `/tmp/commandcode*` directories accumulating test runs, shellouts, and project caches.
  2. `/tmp/muse-*` and `/tmp/opencode*` scratch outputs in tmpfs RAM.
  3. Orphaned daemon / upgrade / runner child processes (e.g. `muse-bin`, `node` executing `command-code`, `opencode serve`) reparented to PID 1.
  4. Inactive swapped agents idling indefinitely past thresholds.

## 2. Code Boundaries & Interfaces
- Process Triage: `internal/triage/triage.go` classifies processes into `model.CatAbandonedAgent`, `model.CatDormantAgent`, `model.CatDesktopDaemon`, or `model.CatActiveProcess`.
- Cache Triage: `internal/cache/cache.go` defines `DefaultTmpPatterns` matching temporary directories in `/tmp` for `untouched-caches` and `abandoned-runs`.

## 3. Verified Signatures
- `commandcode` / `cmd`:
  - Binary/comm: `commandcode`, `cmd`
  - Arguments: `command-code`, `commandcode`
  - Scratch paths: `/tmp/commandcode*`, `/tmp/command-code*`
- `opencode`:
  - Binary/comm: `opencode`
  - Scratch paths: `/tmp/opencode*`
- `muse`:
  - Binary/comm: `muse`, `muse-bin*`
  - Scratch paths: `/tmp/muse-*`
- `agy`:
  - Binary/comm: `agy`, `antigravity`
  - Arguments: `--conversation`
