# Goal: Native AI CLI Coverage for Headless VPS Deployments

## Objective
Enable SwapAudit to autonomously recognize, audit, and safely prune orphaned processes and RAM-backed tmpfs caches from the four primary AI coding CLIs (`opencode`, `commandcode` / `cmd`, `muse`, and `agy`), preventing RAM/swap exhaustion on headless Linux servers without requiring system reboots.

## Deliverables
1. **Process Classifier Expansion (`internal/triage/triage.go`)**:
   - Detect `commandcode`, `cmd`, and `command-code` node runners.
   - Detect `muse`, `muse-bin*`, `opencode`, and `agy` agent sessions.
   - Classify into orphaned (`CatAbandonedAgent`) when PPID=1, and dormant (`CatDormantAgent`) when idle past threshold.
2. **Default Cache Patterns Expansion (`internal/cache/cache.go`)**:
   - Add `"commandcode*"`, `"command-code*"`, `"opencode*"`, and `"muse-*"` to `DefaultTmpPatterns`.
3. **Automated Unit & Table-Driven Tests (`internal/triage/triage_test.go`)**:
   - Verify orphaned and dormant detection for all four tool patterns.
   - Verify negative cases (active sessions, normal system daemons).
4. **Live Verification**:
   - Verify that `swapaudit --clean-all --dry-run` successfully recognizes live `commandcode` and `muse` scratch directories and orphaned processes.
