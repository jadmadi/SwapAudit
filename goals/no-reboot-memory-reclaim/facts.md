# Facts: Clean Swap and RAM Without Reboot via Guarded Swap Drain and Cache Reclaim

## Problem
- `swapaudit` can send `SIGTERM` to selected processes and delete selected `/tmp` directories.
- It has no way to force anonymous pages held by live, wanted processes out of swap.
- The kernel only faults swapped pages back into RAM on access. The supported no-reboot
  mechanism on Linux is `swapoff <device>` followed by `swapon <device>`.
- `swapoff` fails or invokes the OOM killer when the used swap does not fit in available RAM.

## Live Host Evidence (2026-11-02)
- `/proc/swaps`: one partition `/dev/nvme0n1p3`, 17,892,336 KiB, 0 KiB used, priority -2.
- `free -m`: 31,994 MiB RAM, 24,823 MiB free, 25,881 MiB available, 17,472 MiB swap.
- `/proc/pressure/memory`: available (`some` and `full` lines).
- `/sys/fs/cgroup/memory.reclaim`: exists but is root-only.
- `/sbin/swapoff` and `/sbin/swapon` exist; `/sbin` is not on the unprivileged `PATH`.
- `sudo -n true` exits 1: no passwordless root. Live mutation cannot be executed in this
  session; the executor contract is proven with injected fakes and a live read-only dry-run.

## Safety Regression Evidence
- `git diff a078f96 HEAD -- internal/` shows `internal/cache/liveness.go` (-183),
  `internal/cleaner/cleaner.go` (556 -> 266 lines), and the orphan classifier removed.
- `docs/safety.md` still promises `--all-users`, `--clean-orphaned-caches`,
  `--explain-caches`, non-TTY exit 2, UID checks, and open-handle checks. None exist at HEAD.
- Live false positive: PID 328044 `ollama serve` has UID 999, PPID 1,
  cgroup `/system.slice/ollama.service`, and `tty_nr=0`. Current triage labels it
  `abandoned-agents` because `arg == "serve"` and `IsProcessOrphaned(1)` returns true.
- `--clean-orphaned` as root would signal a system service owned by another UID.

## Reclaim Math
- Let `U` be the sum of `Used` across all non-skipped swap devices.
- Let `A` be `MemAvailable` and `R` the reserve.
- Draining is feasible only when `A - U - R >= 0`.
- The default reserve is `max(512 MiB, MemTotal / 10)`.
- Peak RAM during a drain is the sum of all drained pages, because pages faulted back in
  stay resident while later devices are drained.
- zram is compressed swap inside RAM. Draining it decompresses pages into RAM and usually
  increases RAM pressure, so it is skipped unless `--include-zram` is explicit.

## Interface Contracts
- `/proc/swaps` fields: Filename, Type, Size (KiB), Used (KiB), Priority.
- `/proc/meminfo` fields used: MemTotal, MemFree, MemAvailable, Buffers, Cached,
  SReclaimable, Shmem, Dirty, Writeback, SwapTotal, SwapFree.
- `/proc/pressure/memory` lines: `some avg10=... avg60=... avg300=... total=...` and `full`.
- `swapoff <device>` disables one area; `swapon [-p priority] <device>` re-enables it.
- `/proc/sys/vm/drop_caches` accepts `3` to drop page cache, dentries, and inodes; a
  `sync()` first is required to write back dirty pages.
- Root is required for `swapoff`, `swapon`, `drop_caches`, and `memory.reclaim`.

## Non-Goals
- No automatic OOM killing.
- No daemon or systemd unit.
- No `memory.reclaim` or `compact_memory` in this iteration.
- No kernel tuning writes (swappiness, vfs_cache_pressure).
- No zram drain by default.

## Exit Code Contract
- `0`: success, dry-run, cancelled, already clean, or no swap configured.
- `2`: usage or precondition failure (not root, non-TTY without `--yes`, infeasible drain).
- `3`: a device was left disabled or a reclaim command failed.
- `1`: unexpected internal error.
