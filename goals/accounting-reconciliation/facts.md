# Facts: Make Swap and tmpfs Accounting Reconcile with Every Header Total

## Architectural Invariants & Constraints

- **One sum, one owner.** The report builder is the only place that sums `SwapBytes`, `AllocatedBytes`, or category totals. Display functions read report fields; they never recompute a total with an ad-hoc loop.
- **No negative byte values.** If attributed bytes exceed a system total (shared swap, scan/statfs race), report `Overlap` and clamp `Unattributed` to zero.
- **Pools are not additive.** `ReclaimTmpfs` and `ReclaimSwap` may overlap; no primary `Total` field, and any combined figure must carry `upper bound; pools may overlap`.
- **Kernel totals are authoritative.** `SwapUsed` and `TmpfsUsed` come from `/proc/meminfo` and `statfs`; the scanner can only reduce the attributed portion, never redefine the total.
- **Exact bytes, rounded display.** Invariants are tested in `int64` bytes; the UI may format to one decimal.
- **No silent exclusions.** Every scan skip reason is counted and surfaced as `unattributed` with a reason breakdown.
- **No new dependency.** `go.mod` stays dependency-free; CGO stays disabled.
- **No behavior change outside reporting.** This goal does not change which processes are killed, which caches are cleaned, or which files are scanned.

## Observed Facts (Repository)

| Fact | Evidence | Consequence |
| --- | --- | --- |
| `SwapUsed` is `SwapTotal - SwapFree` | `internal/proc/proc.go:54` | Header total is authoritative; process sum is only an attribution |
| `MemUsed` is `MemTotal - MemAvail` | `internal/proc/proc.go:55` | It is not process RSS/PSS and must be labelled |
| `/tmp` used is `statfs` blocks minus free blocks | `internal/proc/proc.go:68-73` | Scanner can under-attribute due to UID/name/daemon/dotfile filters |
| Processes are included only when `VmSwap > 0` | `internal/app/app.go:63-65` | The process pool is "swapped processes", not all processes |
| Swap sums are per-category | `internal/app/display.go:316-328` | A residual row is required |
| `Process Swap (RAM)` is a literal label | `internal/app/display.go:366` | The metric name is misleading and must change |
| `Cleanable` adds `totalReclaimTmp + totalReclaimSwap` | `internal/app/display.go:84` | The headline mixes pools |
| Share denominators are attributed totals | `internal/app/display.go:374-376`, `:415-417` | Shares sum to 100% of the scan, not of the system; label both |
| Actionable limit is hardcoded to 12 | `internal/app/display.go:117` | `--limit` does not affect the actionable table |
| Main-table hidden count mixes size filtering and limit | `internal/app/display.go:251-258`, `:302-304` | The footer cannot be compared with raw category entities |
| `--all` still hides small protected items | `internal/app/display.go:251-258` | Help says "all"; behavior is not all |
| Cache size uses `info.Size()` | `internal/cache/cache.go:124` | Sparse files overstate reclaimable bytes |
| Scan excludes other UIDs, daemon prefixes, dotfiles, non-matching names | `internal/cache/cache.go:44`, `:51-70`, `:58`, `:91-114` | Residuals are expected and must be explained |
| Prune footer uses `len(targets)-shown` | `internal/cleaner/cleaner.go:187-189` | Skipped targets inflate the hidden count without adding bytes |
| Dry-run already splits reclaim | `internal/cleaner/cleaner.go:358-370` | The audit header is inconsistent with the cleanup summary |

## Observed Facts (Live Host, 2026-10-19)

| Observation | Command | Value |
| --- | --- | --- |
| Swap total | `grep '^SwapTotal:' /proc/meminfo` | `17892336 kB` ≈ 17.1 GiB |
| Swap free | `grep '^SwapFree:' /proc/meminfo` | `10015744 kB` |
| Swap used | derived | `7876592 kB` ≈ 7.5 GiB |
| `Σ VmSwap` | `awk '/^VmSwap:/{s+=$2} END{print s}' /proc/[0-9]*/status` | `2999900 kB` ≈ 2.86 GiB |
| Swap residual | derived | ≈ 4.65 GiB |
| `/tmp` total | `df -B1 /tmp` | `16774586368` bytes |
| `/tmp` used | `df -B1 /tmp` | `5596966912` bytes ≈ 5.2 GiB |
| Scanned cache categories | `./swapaudit --no-color --ascii` | `782.2 MB + 373.8 MB + 3.0 GB ≈ 4.15 GiB` |
| Tmpfs residual | derived | ≈ 1.05 GiB |
| Actionable population | `./swapaudit --no-color --ascii` | 150 cleanable; 12 shown; 138 hidden; 23.3 MB hidden |
| Main-table population | `./swapaudit --no-color --ascii` | 804 raw category entities; 12 shown; 717 hidden by the table's mixed filters |
| `--all` hidden count | `./swapaudit --no-color --ascii --all --limit 0` | 594 hidden; `--all` is not all |
| Shell aggregation | `./swapaudit --no-color --ascii` | `bash (66 shells)`, one row; summary counts 66 |
| Process swap categories | `./swapaudit --no-color --ascii` | `dormant-agents 814.7 MB`, `desktop-daemons 263.3 MB`, `active-apps 1.8 GB` |

These are single-host observations, not universal guarantees. They are sufficient to prove the structural gap; the tests must use fixtures for exact arithmetic.

## External Facts

- `/proc/meminfo` `SwapTotal` and `SwapFree` are in kB (1024-byte units) on Linux.
- `/proc/[pid]/status` `VmSwap` is the amount of swap used by that process; it is not a system-wide total and can include shared swapped pages.
- `statfs` `Blocks * Bsize` is the filesystem size; `Bfree` is free blocks; `Bavail` is free blocks available to unprivileged users. tmpfs normally has no reserved blocks, but report both if they differ.
- `syscall.Stat_t.Blocks` is in 512-byte units on Linux, regardless of the filesystem block size.
- tmpfs pages can be swapped out. Deleting a tmpfs file can free both tmpfs space and swap; therefore tmpfs and swap reclaim pools overlap.
- Directory inodes consume blocks; a pure file-size sum cannot exactly equal `statfs` used bytes.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| The sum of visible `VmSwap` is the best available process attribution | It is the only per-process swap metric in `/proc/status` without PSS/smaps | Compare with independent `/proc` sum; document shared-page caveat |
| `Bfree` is the right header free value for tmpfs | Avoids a misleading "free" that excludes reserved blocks | Compare `Bfree` and `Bavail` on tmpfs; if different, report both |
| Allocated blocks are the right reclaim estimate | `statfs` counts blocks, not apparent size | Sparse-file fixture |
| Scan skips are stable during one audit | The report is a snapshot | Compute residuals from one scan; show overlap/race if totals move |
| One decimal is enough for the UI | The CLI is human-readable | Exact-byte tests; formatted golden tests allow rounding |

## Unknowns

- Whether `VmSwap` includes shared swap entries in all kernel versions. **Unknown.** Label the sum as an upper bound and show `Overlap` when it exceeds the system total.
- How much of the swap residual is kernel, shmem/tmpfs, or inaccessible processes on a given host. **Unknown.** Show one residual and the reasons the scanner can prove; do not invent a split.
- Whether hard links inside scanned caches double-count allocated blocks. **Unknown.** Document as `hardlink-overlap`; a future inode-aware scan can fix it.
- Whether `statfs` and the recursive walk can be made atomic. **Unknown.** Use `Overlap` for races; do not fail the audit.

## Evidence Commands

```bash
grep -n "SwapUsed\|MemUsed\|TmpfsUsed" internal/proc/proc.go
grep -n "totalReclaimTmp\|totalReclaimSwap\|Process Swap (RAM)\|more hidden\|hiddenBytes" internal/app/display.go
grep -n "info.Size()\|newestMtime\|lastActive > maxAge\|currentUid" internal/cache/cache.go
grep -n "len(targets)-shown\|hiddenBytes" internal/cleaner/cleaner.go
grep -E '^(SwapTotal|SwapFree|MemTotal|MemAvailable):' /proc/meminfo
df -B1 /tmp
awk '/^VmSwap:/{s+=$2} END{printf "%d kB\n", s}' /proc/[0-9]*/status
go test -race ./...
```

## File & Interface Contracts

- `internal/report/accounting.go` (new): `Pool`, `ReasonBytes`, `Attribution`, `Reclaim`, `Population`, `CategoryTotal`, `Thresholds`, `AuditReport`, `Build`, `Reconciles`, `BiggestWin`, `CleanableCount`.
- `internal/model/model.go`: add `AllocatedBytes`, `ApparentBytes`, `Kind`, `ID`, `Evidence`, `OpenHandles`; keep `SizeBytes` as the reclaim/display size.
- `internal/cache/cache.go`: add `ScanStats`/`SkipReason` and allocated-size accounting; preserve the existing selection rules.
- `internal/app/app.go`: build one `AuditReport` and pass it to display; return `ScanStats` from the collector.
- `internal/app/display.go`: read report fields; add reconciliation, residual rows, split reclaim, population lines; remove `Process Swap (RAM)`.
- `internal/cleaner/cleaner.go`: fix hidden-target counting; keep the existing split reclaim summary.
- `docs/metrics.md` (new): the A1 metric dictionary and caveats.
- `scripts/check.sh`: optional label sentry.
- `go.mod`: unchanged.
