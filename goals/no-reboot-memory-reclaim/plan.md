# Plan: Clean Swap and RAM Without Reboot via Guarded Swap Drain and Cache Reclaim

## Execution Steps

- [x] **Phase 1 — Kernel readers and model**
  - Add swap-device parsing for `/proc/swaps` with per-device used bytes and priority.
  - Add page-cache counters (`Buffers`, `Cached`, `SReclaimable`, `Shmem`, `Dirty`,
    `Writeback`, `MemFree`) to `model.SystemSwap` and the snapshot document.
  - Add `/proc/pressure/memory` parsing.
  - Add `ReadProcUID`, `ReadTTYNr`, `ReadProcessCgroup`, and `ClassifyOrphan`.
  - Add table-driven tests for every parser using fixture readers.

- [x] **Phase 2 — Pure reclaim planner**
  - Add `internal/reclaim` with `BuildPlan`, `DefaultReserve`, per-device decisions,
    zram detection, headroom math, and stable block reasons.
  - Test feasible, infeasible, already-clean, no-swap, zram-only, mixed zram,
    per-device-used, and reserve cases.
  - Test that `MemAvailable - used - reserve == 0` is feasible and `-1` is not.

- [x] **Phase 3 — Guarded executor**
  - Add a `System` seam (`SwapOff`, `SwapOn`, `DropCaches`, `Sync`) with a real
    implementation that resolves `/sbin/swapoff` and `/sbin/swapon` outside `PATH`.
  - Drain each device as `swapoff -> verify off -> swapon -> verify on`.
  - Re-check headroom before every device and abort if it disappeared.
  - Restore every disabled device on error, cancellation, or signal.
  - Never execute in `--dry-run`; require root and `--yes` or a TTY.
  - Return exit `3` if a device remains disabled.
  - Test the full call order, recovery on `swapon` failure, preflight abort,
    zram skip, already-clean skip, and dry-run purity with a fake system.

- [x] **Phase 4 — CLI and report integration**
  - Add `swapaudit reclaim [--drain-swap] [--drop-caches] [--dry-run] [--yes]
    [--json] [--reserve] [--include-zram] [--timeout]`.
  - Print a plan and a measured before/after receipt.
  - Add a reclaim-readiness block to the main report: feasibility, reserve,
    headroom, swap devices, page-cache estimate, PSI, and the exact next command.
  - Add reclaim commands to the quick-command section and usage text.
  - Validate incompatible flags before touching `/proc` or `/sys`.

- [x] **Phase 5 — Safety regressions**
  - Enforce target UID == real UID before every `SIGTERM`; skip and report otherwise.
  - Replace `IsProcessOrphaned` with fail-closed `ClassifyOrphan` evidence
    (parent liveness, `.service` cgroup, controlling terminal, `.scope`/`session-`).
  - Stop classifying every `serve`, `--service`, `--stdio`, or `mcp` argv as an agent.
  - Reject non-TTY cleanup without `--dry-run` or `--yes` with exit `2` before scanning.
  - Remove the dead no-op process filter in `CollectSwapItems`.
  - Add regression tests for `ollama serve`, systemd services, and the UID guard.

- [x] **Phase 6 — Verification and evidence**
  - `gofmt -l .`, `go vet ./...`, `go test -race ./...`, `./scripts/check.sh`.
  - Live read-only `swapaudit reclaim --dry-run` receipt.
  - Live audit proving `ollama serve` is no longer cleanable.
  - Write `evidence.md` with a standalone `Verdict: PASS` line.
  - Commit with `(goals/no-reboot-memory-reclaim/goal.md)` and certify.

## Acceptance Assertions
- AC-01: A machine with 4 GiB used swap and 2 GiB available RAM is infeasible and
  performs no `swapoff`.
- AC-02: A machine with 4 GiB used swap, 8 GiB available, and a 2 GiB reserve is
  feasible.
- AC-03: Every drained device is re-enabled before the next device starts.
- AC-04: A `swapon` failure returns exit 3 and prints the exact recovery command.
- AC-05: `--dry-run` calls neither `swapoff` nor `swapon` nor `drop_caches`.
- AC-06: A non-root mutation is refused with exit 2 before any command.
- AC-07: A non-TTY mutation without `--yes` is refused with exit 2.
- AC-08: zram-only swap is not drained unless `--include-zram` is set.
- AC-09: Zero used swap performs no churn and exits 0.
- AC-10: `ollama serve` (UID 999, `.service`, tty 0) is not an orphaned agent.
- AC-11: A process owned by another UID is skipped before `SIGTERM`.
- AC-12: The snapshot remains valid JSON with the new cache fields.

## Edge Cases
- No swap configured.
- `swapoff` returns an error but the device is still active.
- `swapoff` succeeds and `swapon` fails once, then succeeds on retry.
- A device disappears between plan and execution.
- `/proc/swaps` is unreadable.
- A swap priority other than the default `-2`.
- Interrupt arrives between `swapoff` and `swapon`.
- `drop_caches` write is denied despite euid 0 (container).
