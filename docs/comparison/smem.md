# swapaudit vs. smem

Both are userspace memory reporters, but they measure different things. **smem**
computes USS and PSS from `/proc/<pid>/smaps` so shared pages are divided among
the processes that map them. **swapaudit** attributes swap and tmpfs usage to
specific processes and directories, classifies what it finds, and can reclaim
selected targets.

smem answers *"how much physical memory does this process really cost?"*.
swapaudit answers *"who is holding swap and tmpfs, and what is safe to remove?"*.

## At a glance

| Dimension | swapaudit | smem |
| :-------- | :-------- | :--- |
| Role | Swap and tmpfs auditor with cleanup | Physical memory accounting reporter |
| Model | One-shot CLI | One-shot CLI |
| Primary metric | `VmSwap`, idle time, parent PID, `/tmp` size and age | USS, PSS, RSS, and swap per process |
| Shared-memory handling | Not computed; reports per-process `VmSwap` | PSS divides shared pages among sharers; USS is private only |
| Swap handling | Per-process `VmSwap` is the main signal | Separate `Swap` column; USS/PSS exclude swapped memory |
| Scope | Processes, categories, and `/tmp` directories | Processes, users, mappings, or systemwide |
| Classification | Dormant agents, orphans, stale caches, active apps, desktop daemons | None; filters by process, user, or mapping |
| Action | SIGTERM selected agents, remove selected directories | None; read-only |
| Dry run | `--dry-run` | Not applicable |
| Confirmation | Interactive `[y/N]`, bypass with `-y` | Not applicable |
| Privileges | Normal user | Normal user sees accessible processes; root sees all |
| Requirements | Linux with `/proc` | Python; `Pss` in `/proc/<pid>/smaps` (Linux 2.6.27+) |
| Footprint | One-shot, sub-second, no idle cost | Parses `smaps` for every process; can be slow on large systems |
| Output | Terminal report with categories and reclaim totals | Table, totals, bar charts, pie charts |
| Distribution | Static binary, `go install`, self-upgrade | Distribution package; `smemcap` for offline captures |
| License | WaqfDPL-1.0 or AGPL-3.0 | GPL-2.0-or-later |
| Best fit | Finding and reclaiming swap and tmpfs occupants | Understanding shared memory and true physical memory cost |

## The core difference: PSS accounting vs. swap attribution

smem's key contribution is **proportional set size**. A process's memory is split
into three views:

- **USS (Unique Set Size):** private pages that belong only to this process.
- **RSS (Resident Set Size):** all resident pages, including shared pages. Summing
  RSS across processes double-counts every shared library and shared mapping.
- **PSS (Proportional Set Size):** USS plus each shared page divided by the number
  of processes mapping it. Summing PSS across processes gives a much better
  estimate of the physical memory actually in use.

smem reads `/proc/<pid>/smaps` and reports the default columns `PID`, `USER`,
`COMMAND`, `SWAP`, `USS`, `PSS`, and `RSS`, sorted by PSS. The `SWAP` column is
described as "amount of swap space consumed (ignoring sharing)". USS and PSS
include physical memory only; they explicitly exclude memory that has been
swapped out.

swapaudit does not compute USS or PSS, and it does not rank processes by resident
memory. Its process rows are about swap occupancy and age: `VmSwap` from
`/proc/<pid>/status`, idle time from `/proc/<pid>/sched`, the parent PID, and the
command line. Its directory rows are about `/tmp` size and mtime. That makes the
two tools complementary rather than overlapping:

```
        smem                                  swapaudit
  +-------------------+                  +-------------------+
  | /proc/<pid>/smaps |                  | /proc/<pid>/status|
  | USS / PSS / RSS   |                  | VmSwap            |
  | shared pages      |                  | idle time         |
  | divided fairly    |                  | parent PID        |
  +---------+---------+                  +---------+---------+
            |                                      |
   "who uses physical RAM?"              "who is in swap, and
            |                             what is safe to clean?"
            v                                      v
      read-only report                     classified cleanup
```

smem can also report by user (`-u`), by mapping (`-m`), or systemwide (`-w`), and
it can filter processes, users, or mappings with regular expressions. swapaudit
reports by triage category instead. smem's `smemcap` utility can capture the
needed `/proc` data on one machine and analyze it later with `smem --source`,
which is useful for embedded systems and offline forensics. swapaudit has no
equivalent capture mode.

## Where smem wins

- **Correct shared-memory accounting.** PSS avoids the double-counting that makes
  RSS totals misleading on systems with many browsers, editors, and language
  servers sharing libraries.
- **Multiple aggregation levels.** It can report per process, per user, per
  mapping, or systemwide, with `-K` and `-R` options to account for the kernel
  image and firmware memory in the systemwide view.
- **Flexible filtering and output.** Process, user, and mapping regex filters;
  column selection with `-c`; sorting with `-s`; totals with `-t`; unit suffixes
  with `-k`; percentages with `-p`; numeric UIDs with `-n`.
- **Graphical output.** Bar and pie charts are available when matplotlib is
  installed, which is useful for presentations and quick comparisons.
- **Offline capture.** `smemcap` plus `smem --source` lets you analyze a running
  system's memory without Python on the target.
- **A swap column.** It reports per-process swap alongside USS/PSS/RSS, so you get
  a physical-memory and swap view in one table.
- **Read-only by design.** There is no risk of accidental deletion or process
  termination, which makes it suitable for shared and production systems.

## Where swapaudit wins

- **Swap and tmpfs are the focus.** It reports `VmSwap` per process and scans
  `/tmp` for stale build caches and abandoned test runs. smem's USS/PSS exclude
  swapped memory and it does not inspect `/tmp` contents.
- **Idle and orphan classification.** It knows whether an agent has been idle for
  hours and whether its parent shell died. smem has no notion of age or
  parentage; it reports whatever processes exist now.
- **It can act.** SIGTERM to selected agents and removal of selected directories,
  with `--dry-run`, per-group confirmation, and `-y` for automation. smem is
  strictly read-only.
- **Safety rails.** Path confinement to `/tmp` and `/var/tmp`, ownership checks on
  files, symlink rejection, PID start-ticks verification, and protection for
  desktop daemons and active apps.
- **Developer-aware categories.** Dormant AI agents, MCP servers, orphaned
  sessions, shell floods, untouched caches, and abandoned runs are first-class
  concepts. smem only knows process, user, and mapping.
- **No Python or `smaps` parsing cost.** swapaudit is a static Go binary that
  reads a small set of `/proc` files and `/tmp` metadata. smem parses
  `/proc/<pid>/smaps` for every visible process, which can be expensive on a
  busy workstation.
- **Self-upgrade.** `swapaudit upgrade` verifies a SHA256 checksum and replaces
  the binary atomically.

## Where each falls short

### smem

- It is read-only. It will not tell you what is safe to remove, and it cannot
  clean anything.
- USS and PSS exclude swapped memory. The `Swap` column is separate and is
  described as ignoring sharing, so it is not a unique-owner swap measure.
- It has no idle-time, parent-death, or stale-file analysis. It cannot distinguish
  a dormant agent from an active one if both are currently resident.
- It does not inspect `/tmp`, tmpfs contents, or build caches.
- It requires Python and readable `/proc/<pid>/smaps`. A normal user sees only
  processes whose `/proc` data is accessible; `hidepid` and similar mount options
  can hide data.
- Parsing `smaps` for every process can be slow and can add measurable overhead on
  a large or busy system.
- It is a point-in-time reporter, not a continuous monitor or an OOM guardrail.

### swapaudit

- It does not compute USS or PSS, and it does not account for shared pages. Its
  process rows show swap bytes, not physical memory cost.
- It has no per-user, per-mapping, or systemwide physical-memory report.
- It has no charts, CSV output, or offline `/proc` capture.
- It is manual: no continuous monitoring, no pressure trigger, and no automatic
  action unless you schedule it.
- It only acts on its known categories and only sends SIGTERM. A wedged process
  can ignore it.
- It is Linux-only and `/proc`-dependent.

## Using them together

smem tells you where physical memory is going; swapaudit tells you where swap is
going and what can be reclaimed.

1. **Get the physical-memory picture.**

   ```bash
   smem -w
   smem -u -k -t
   smem -m -k -t
   ```

2. **Zoom in on a suspect.**

   ```bash
   smem -P 'chrome|firefox|code' -c 'pid user command swap uss pss rss' -k -s pss
   ```

3. **Attribute the swap and find stale files.**

   ```bash
   swapaudit
   swapaudit --sort age
   ```

4. **Reclaim only what you trust.**

   ```bash
   swapaudit --clean-all --dry-run
   swapaudit --clean-untouched --clean-abandoned -y
   ```

A useful split of responsibilities:

- If you need to know whether a process is expensive because of shared libraries,
  use smem's PSS.
- If you need to know whether a process is expensive because it was swapped out
  hours ago, use swapaudit's `VmSwap` and idle time.
- If you need to free memory without killing anything, use swapaudit's `/tmp`
  cleanup.

## Which one should you use?

| Situation | Recommendation |
| :-------- | :------------- |
| You need to understand shared memory and PSS | smem |
| You need a per-user or per-mapping memory breakdown | smem |
| You need to know which process holds swap | swapaudit |
| You need to find dormant agents or orphaned sessions | swapaudit |
| You need to remove stale `/tmp` build output | swapaudit |
| You want a read-only report for a production host | smem |
| You want to act on what the report finds | swapaudit |
| You want one tool and no cleanup risk | smem |
| You want one tool and no Python dependency | swapaudit |

## References

- `smem(8)` man page: <https://manpages.debian.org/trixie/smem/smem.8.en.html>
- smem source: <https://sources.debian.org/src/smem/1.5-3/smem/>
- smem upstream: <http://www.selenic.com/smem>
- swapaudit: [README](../../README.md), [`internal/proc`](../../internal/proc/proc.go),
  [`internal/triage`](../../internal/triage/triage.go),
  [`internal/cleaner`](../../internal/cleaner/cleaner.go),
  [`internal/cache`](../../internal/cache/cache.go)

smem behavior verified against the Debian 1.5-3 man page and source; PSS/USS
definitions and the separate swap column are taken from that implementation.

## Related comparisons

- [All comparisons](README.md)
