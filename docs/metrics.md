# Metric dictionary

Every byte on the screen is one of three things: a kernel total, a category sum, or an explicit `unattributed`/`overlap` residual. Display code reads report fields; it never recomputes a total.

## Canonical byte definitions

| Name | Definition | Source | Additive? |
| --- | --- | --- | --- |
| `SwapTotal` | `SwapTotal` | `/proc/meminfo` | no |
| `SwapFree` | `SwapFree` | `/proc/meminfo` | no |
| `SwapUsed` | `SwapTotal - SwapFree` | derived | no |
| `ProcessSwapAttributed` | `Σ VmSwap` over scanned processes with `VmSwap > 0` | `/proc/[pid]/status` | within the scan |
| `SwapUnattributed` | `max(0, SwapUsed - ProcessSwapAttributed)` | derived | no |
| `SwapOverlap` | `max(0, ProcessSwapAttributed - SwapUsed)` | derived | no |
| `TmpfsTotal` | `Blocks * Bsize` for `/tmp` | `statfs` | no |
| `TmpfsFree` | `Bfree * Bsize`; `Bavail` is reported separately | `statfs` | no |
| `TmpfsUsed` | `TmpfsTotal - TmpfsFree` | derived | no |
| `TmpfsAttributed` | `Σ AllocatedBytes` over scanned top-level entries | recursive walk | within the scan |
| `TmpfsUnattributed` | `max(0, TmpfsUsed - TmpfsAttributed)` | derived | no |
| `TmpfsOverlap` | `max(0, TmpfsAttributed - TmpfsUsed)` | derived | no |
| `ReclaimTmpfs` | `Σ cleanable cache AllocatedBytes` | scan | upper bound |
| `ReclaimSwap` | `Σ cleanable process VmSwap` | scan | upper bound |
| `MemTotal` | `MemTotal` | `/proc/meminfo` | no |
| `MemAvailable` | `MemAvailable` | `/proc/meminfo` | no |
| `MemUsed` | `MemTotal - MemAvailable` | derived | no |
| `AllocatedBytes` | `Stat_t.Blocks * 512` for files and directories | `Lstat` / walk | yes |
| `ApparentBytes` | `info.Size()` sum for files | walk | yes, display/JSON only |

`RAM used` means `total - available`. It is not the sum of process RSS, PSS, or `VmSwap`, and it is never compared with those sums.

## Invariants

1. `ProcessSwapAttributed + SwapUnattributed == SwapUsed` exactly in `int64` bytes when neither is clamped.
2. `TmpfsAttributed + TmpfsUnattributed == TmpfsUsed` exactly when neither is clamped.
3. If attributed bytes exceed the system total, the report sets `Overlap` and clamps `Unattributed` to zero. No rendered value is negative.
4. `ReclaimTmpfs` and `ReclaimSwap` are never added into a primary total. A combined figure is allowed only with the literal `upper bound; pools may overlap`.
5. Every rendered byte comes from a kernel total, a category sum, or a residual row. `internal/report` owns every sum.
6. Formatting may round to one decimal; the invariant is checked in exact bytes.
7. Both residual rows are always rendered, including `0 B`.
8. Shares use two explicit denominators: `share_of_attributed = category / attributed` and `share_of_system = category / system_total`. The table header says which one is shown. `share_of_system` can exceed `1.0` when the pool reports `overlap`; it is never negative.

## Scan skip reasons

The `/tmp` scan counts every exclusion:

| Reason | Meaning |
| --- | --- |
| `other-uid` | Top-level entry owned by another UID |
| `daemon-prefix` | Known daemon prefix (`tmux-`, `codex-daemon-`, `plasma-`, …) |
| `dotfile` | Name starts with `.` |
| `systemd-or-pulse` | `systemd-*` or `pulse-*` |
| `symlink` | Top-level symlink |
| `special-file` | Socket, named pipe, or device |
| `non-matching` | Does not match a cache/test pattern or keyword |
| `inaccessible` | `ReadDir`, `Lstat`, or walk error |
| `metadata-or-other` | Residual not attributable to a per-entry reason |
| `kernel-or-shmem` | Swap residual not attributable to any single process |

Process scan reasons: `zero-swap`, `inaccessible`, `vanished`, and `own-process` (the auditor itself). Only nonzero-byte reasons are rendered in the human summary; zero-byte reasons still appear in JSON counts.

## Caveats

- **Shared swap:** `VmSwap` can include shared swapped pages; the sum is an attribution, not a physical-memory claim.
- **Kernel and shmem:** the swap residual includes kernel pages, swapped tmpfs/shmem, and processes the scanner cannot read.
- **tmpfs pages can be swapped:** deleting a tmpfs file can free both tmpfs blocks and swap. The two reclaim pools may overlap.
- **Sparse files:** allocated blocks are used for reclaim estimates; apparent size is retained only for JSON and diagnostics.
- **Directory metadata:** directory inodes consume blocks; residual `metadata-or-other` is expected.
- **Hard links:** each path is counted once. A hard-linked tree can make the attributed sum exceed the block total; this appears as `overlap`.
- **Scan/statfs race:** files created or removed during the walk can produce a small residual or overlap. The audit reports it instead of failing.
- **Other UIDs and filters:** the scanner intentionally excludes other users, daemon prefixes, dotfiles, and non-matching names. Those bytes remain visible in the residual.

## Commands

```bash
grep -E '^(SwapTotal|SwapFree|MemTotal|MemAvailable):' /proc/meminfo
df -B1 /tmp
awk '/^VmSwap:/{s+=$2} END{printf "%d kB\n", s}' /proc/[0-9]*/status
./swapaudit --no-color --ascii
./swapaudit --json | jq '.attribution'
```
