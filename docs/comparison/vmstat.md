# swapaudit vs. vmstat

**vmstat** is a system-wide sampler. It reports processes, memory, paging, block
I/O, interrupts, context switches, and CPU activity, and it can show whether the
system is swapping right now. **swapaudit** is a point-in-time auditor that
attributes swap and tmpfs usage to individual processes and directories, then
can reclaim selected targets.

vmstat tells you that the system is paging. swapaudit tells you who is paging and
what is safe to clean.

## At a glance

| Dimension | swapaudit | vmstat |
| :-------- | :-------- | :----- |
| Role | Swap and tmpfs auditor with cleanup | System-wide virtual memory statistics sampler |
| Model | One-shot CLI | One-shot report or interval sampling |
| Primary signal | Per-PID `VmSwap`, idle time, parent PID, `/tmp` size and age | `swpd`, `free`, `buff`, `cache`, `si`, `so`, `bi`, `bo`, CPU |
| Scope | Processes and `/tmp` directories | Whole system |
| Time dimension | Snapshot | First report since boot, then interval samples |
| Per-process visibility | Yes | No |
| Classification | Dormant agents, orphans, stale caches, active apps, desktop daemons | None |
| Action | SIGTERM selected agents, remove selected directories | None; read-only |
| Dry run | `--dry-run` | Not applicable |
| Confirmation | Interactive `[y/N]`, bypass with `-y` | Not applicable |
| Privileges | Normal user | Normal user; `-m` may need `/proc/slabinfo` access |
| Requirements | Linux with `/proc` | Linux with readable `/proc` (procps-ng) |
| Footprint | One-shot, sub-second, no idle cost | Very small; suitable for continuous sampling |
| Output | Terminal report with categories and reclaim totals | Fixed columns; `-s`, `-d`, `-m`, `-p`, `-a` modes |
| Distribution | Static binary, `go install`, self-upgrade | Part of procps-ng, installed almost everywhere |
| License | WaqfDPL-1.0 or AGPL-3.0 | LGPL-2.1-or-later (procps-ng `vmstat`) |
| Best fit | Identifying swap occupants and cleaning them | Confirming active swapping and correlating it with CPU and I/O |

## The core difference: rates vs. attribution

vmstat reads kernel counters and prints a compact table. In VM mode, the fields
are:

- **Procs:** `r` runnable processes, `b` blocked processes.
- **Memory:** `swpd` used swap, `free` idle memory, `buff` buffers, `cache` page
  cache.
- **Swap:** `si` memory swapped in per second, `so` memory swapped out per second.
- **IO:** `bi` KiB received from block devices per second, `bo` KiB sent to block
  devices per second.
- **System:** `in` interrupts per second, `cs` context switches per second.
- **CPU:** `us`, `sy`, `id`, `wa`, `st`, and `gu`.

The first report contains averages since boot; later reports cover the sampling
interval. The process and memory columns are instantaneous in both cases. This is
why a first line can show nonzero `si`/`so` even when nothing is swapping now.
Use `vmstat 1 5` for current behavior, or `vmstat -y 1 5` to omit the first
report entirely.

swapaudit does not measure rates. It takes a snapshot of the current state:
`VmSwap` per process, idle time per process, parent PID, and the size and newest
mtime of `/tmp` directories. It can then classify the occupants and act on the
safe ones. vmstat cannot do any of that, because it never looks at per-process
memory or `/tmp` contents.

```
        vmstat                                  swapaudit
  +-------------------+                  +-------------------+
  | /proc/meminfo     |                  | /proc/<pid>/status|
  | /proc/stat        |                  | VmSwap            |
  | /proc/vmstat      |                  | idle time         |
  | system-wide rates |                  | parent PID        |
  +---------+---------+                  +---------+---------+
            |                                      |
   "is the system paging now?"          "which PID or directory
            |                            is responsible, and what
            v                            is safe to reclaim?"
     time-series report                       v
                                       classified cleanup
```

One caveat applies to both layers: `vmstat` reports swap-layer activity and does
not identify the backing device. If swap is backed by zram, those counters no
longer represent disk I/O. It also cannot distinguish a page that was swapped out
minutes ago from one that was swapped out days ago; only swapaudit's age and idle
signals can do that.

## Where vmstat wins

- **It is everywhere.** vmstat is part of procps-ng and is installed on
  practically every Linux system. No extra package, service, or configuration is
  required.
- **It shows whether swap is active or merely allocated.** `swpd` tells you how
  much swap is used; `si` and `so` tell you whether the kernel is actively moving
  pages. That distinction matters when deciding whether a cleanup is urgent.
- **It provides context.** It correlates swapping with runnable and blocked
  processes, block I/O, interrupts, context switches, and CPU wait time. A high
  `wa` with nonzero `si`/`so` is a very different situation from a large `swpd`
  with zero swap traffic.
- **It is a time series.** `vmstat 1`, `vmstat -y 1 5`, and `vmstat -t 1` let you
  watch a trend instead of reading a single snapshot. This is useful for
  confirming whether a cleanup actually reduced paging.
- **It has multiple modes.** `-s` for event counters and memory totals, `-a` for
  active/inactive memory, `-d` for disk statistics, `-D` for a disk summary,
  `-m` for slabinfo, `-p` for a partition, `-w` for wide output, `-S` for units.
- **It is cheap.** The sampling overhead is negligible, so it is safe to run for
  long periods or in a monitoring loop.
- **It is scriptable.** Fixed columns and simple output make it easy to parse or
  log, even if the field semantics are terse.

## Where swapaudit wins

- **Per-process attribution.** It names the PID holding the swap. vmstat has no
  per-process mode at all; it can only tell you that swapping is happening.
- **Idle and orphan classification.** It knows how long a process has been idle
  and whether its parent shell died. vmstat has no concept of process age or
  parentage.
- **tmpfs and file cleanup.** It measures `/tmp` directories and removes stale
  build caches and abandoned test runs. vmstat does not inspect files or
  filesystem contents.
- **It can act.** SIGTERM to selected agents and removal of selected directories,
  with `--dry-run`, per-group confirmation, and `-y` for automation. vmstat is
  strictly read-only.
- **Safety rails.** Path confinement to `/tmp` and `/var/tmp`, ownership checks on
  files, symlink rejection, PID start-ticks verification, and protection for
  desktop daemons and active apps.
- **Developer-aware categories.** Dormant AI agents, MCP servers, orphaned
  sessions, shell floods, untouched caches, and abandoned runs are first-class
  concepts. vmstat only knows system-wide counters.
- **Snapshot detail without sampling.** One invocation gives you the full
  occupant list, sorted by size or age, without waiting for an interval.
- **Self-upgrade.** `swapaudit upgrade` verifies a SHA256 checksum and replaces
  the binary atomically.

## Where each falls short

### vmstat

- It has no per-process or per-file detail. It cannot tell you which PID, cgroup,
  or directory is responsible for swap or page cache usage.
- `swpd` is used swap, not total or free swap. Use `vmstat -s`, `free`, or
  `/proc/meminfo` for totals.
- The first report's rate fields are averages since boot. Without `-y` or a delay
  argument, they can make historical swapping look like current activity.
- It does not identify the swap backing device. zram, disk swap, and other
  devices all feed the same swap counters.
- It does not distinguish stale swap pages from actively used ones.
- It does not inspect `/tmp`, tmpfs contents, or per-process memory.
- It takes no action and offers no classification, dry-run, or safety model.
- Some modes depend on `/proc` permissions; `-m` may not be available to standard
  users, and `hidepid` or `subset=pid` mount options can hide data.
- The column names are terse, and exact semantics can vary with kernel version.

### swapaudit

- It is a snapshot. It does not show rates, trends, CPU wait, I/O throughput, or
  context switches, so it cannot tell you whether the system is actively thrashing
  right now.
- It is manual: no continuous monitoring, no pressure trigger, and no automatic
  action unless you schedule it.
- It only acts on its known categories and only sends SIGTERM. A wedged process
  can ignore it.
- It does not read PSI, cgroup, or zram statistics, so it cannot correlate swap
  occupancy with memory pressure or compression ratios.
- It is Linux-only and `/proc`-dependent.

## Using them together

vmstat is the natural first command when a machine feels slow; swapaudit is the
natural second command when vmstat shows swap activity.

1. **Check whether the system is actually swapping.**

   ```bash
   vmstat -y -w 1 5
   ```

   Ignore the first report with `-y`. Look at `si`, `so`, `b`, and `wa`. A large
   `swpd` with zero `si`/`so` means swap is allocated but not actively churning.

2. **Get the swap totals and counters.**

   ```bash
   LC_ALL=C vmstat -s | grep -i swap
   ```

3. **Attribute the activity.**

   ```bash
   swapaudit
   swapaudit --sort age
   ```

4. **Reclaim safely, then verify.**

   ```bash
   swapaudit --clean-all --dry-run
   swapaudit --clean-untouched --clean-abandoned -y
   vmstat -y -w 1 5
   ```

   If `si` and `so` fall toward zero after the cleanup, the pressure was likely
   driven by swapped-out processes or stale tmpfs data. If `si`/`so` stay high,
   the machine has an active memory consumer that swapaudit's categories may not
   cover; investigate with `ps`, `top`, or `smem`.

A useful division of labor:

- vmstat tells you **whether** paging is happening and how it correlates with CPU
  and I/O.
- swapaudit tells you **who** is in swap and **what** can be removed.
- smem tells you **how much physical memory** each process really costs.
- earlyoom or systemd-oomd gives you an automatic **guardrail** when pressure
  crosses a threshold.

## Which one should you use?

| Situation | Recommendation |
| :-------- | :------------- |
| You need to know whether the system is swapping now | vmstat |
| You need to correlate swapping with CPU wait and I/O | vmstat |
| You need to know which process holds swap | swapaudit |
| You need to find dormant agents or orphaned sessions | swapaudit |
| You need to remove stale `/tmp` build output | swapaudit |
| You want a zero-install, read-only diagnostic | vmstat |
| You want to act on what the diagnostic finds | swapaudit |
| You want continuous monitoring | vmstat (or a monitoring system) |
| You want a single snapshot with categories | swapaudit |

## References

- `vmstat(8)` man page: <https://man7.org/linux/man-pages/man8/vmstat.8.html>
- procps-ng: <https://gitlab.com/procps-ng/procps>
- swapaudit: [README](../../README.md), [`internal/proc`](../../internal/proc/proc.go),
  [`internal/triage`](../../internal/triage/triage.go),
  [`internal/cleaner`](../../internal/cleaner/cleaner.go),
  [`internal/cache`](../../internal/cache/cache.go)

vmstat behavior verified against the procps-ng 4.0.4 man page and local output;
first-report and field semantics are taken from the manual.

## Related comparisons

- [All comparisons](README.md)
