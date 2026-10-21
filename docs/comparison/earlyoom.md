# swapaudit vs. earlyoom

They are not competitors. **earlyoom** is an always-on safety net that prevents a
hard OOM lockup by killing a process. **swapaudit** is an on-demand auditor that
explains where swap and tmpfs memory went, then reclaims specific occupants.

One answers *"is the system about to die?"*. The other answers *"who is holding
the memory, and what is safe to reclaim?"*.

## At a glance

| Dimension | swapaudit | earlyoom |
| :-------- | :-------- | :------- |
| Role | On-demand auditor and surgical cleanup tool | Always-on early OOM daemon |
| Operating model | One-shot CLI, no service | Long-running daemon, usually systemd |
| Trigger | You invoke it | Available memory and free swap drop below thresholds |
| Signal source | Per-PID `VmSwap`, idle time, parent PID, `/tmp` size and mtime | Aggregate `MemAvailable` and free swap |
| Default policy | 12h stale `/tmp` artifacts, 2h idle agents | 10% memory + 10% swap free, SIGTERM; 5%/5%, SIGKILL |
| Target selection | Classified dormant/orphaned agents and stale `/tmp` entries | Highest `oom_score` (normally the largest memory consumer) |
| Action | SIGTERM selected agents, remove selected directories | SIGTERM, then SIGKILL, until pressure recovers |
| Dry run | `--dry-run` | `--dryrun` |
| Confirmation | Interactive `[y/N]`, bypass with `-y` | None; daemon acts autonomously |
| Privileges | Normal user | Root or `CAP_KILL`; the systemd unit uses `DynamicUser` + `CAP_KILL` |
| Footprint | One-shot, sub-second, no idle cost | ~2 MiB RSS, `mlockall()`, adaptive polling up to 10 Hz |
| Observability | Rich terminal report with categories and reclaim totals | Periodic log lines and one log entry per kill |
| Configuration | CLI flags, `SWAPAUDIT_NO_UPDATE_CHECK` | CLI flags, `/etc/default/earlyoom` |
| Distribution | Static binary, `go install`, self-upgrade | Distro packages (Debian, Ubuntu, Fedora, Arch), systemd/init |
| Implementation | Go standard library, zero dependencies | C, libc only, no dependencies |
| License | WaqfDPL-1.0 or AGPL-3.0 | MIT |
| Best fit | Developer workstations, build boxes, swap forensics | Unattended servers, desktops needing an OOM guardrail |

## The core difference: pressure vs. attribution

earlyoom watches two aggregate counters. It checks available memory and free swap
up to ten times per second, backing off when memory is plentiful. If **both** fall
below 10%, it signals the process with the highest kernel `oom_score`; below 5% it
uses SIGKILL. It repeats until pressure recovers. If the machine has no swap, the
swap condition is ignored.

That model is intentionally blind to *why* memory is scarce. earlyoom does not
know which process swapped out hours ago, whether that process is idle, or whether
`/tmp` is full of abandoned build directories. It only knows that the machine is
close to the edge and that something must die.

swapaudit inverts the question. It walks `/proc` and attributes swap per PID using
`VmSwap`, reads idle time from `/proc/<pid>/sched`, checks whether the parent shell
is gone, and separates known agent processes from desktop daemons and active apps.
It also walks `/tmp`, measures directory sizes, and treats the newest recursive
mtime as the true last-activity time. Only then does it classify occupants into
actionable groups.

```
                 earlyoom                              swapaudit
        +--------------------------+          +--------------------------+
        | aggregate pressure       |          | per-PID attribution      |
        | MemAvailable + free swap |          | VmSwap + idle + parent   |
        +------------+-------------+          +------------+-------------+
                     |                                     |
             pick highest oom_score              classify + confirm
                     |                                     |
                     v                        +------------+------------+
              kill one process                |                         |
                     |                  prune stale /tmp        SIGTERM idle
                     v                  build caches            agent sessions
             recover headroom
```

## Where earlyoom wins

- **It acts when nobody is watching.** No invocation, no cron entry, no dashboard.
  A runaway allocation at 3 a.m. still gets handled.
- **It is small and proven.** About 2 MiB resident, `mlockall()` so it stays
  responsive under pressure, adaptive polling, packaged by major distributions,
  and MIT licensed.
- **Policy controls.** `--prefer`, `--avoid`, and `--ignore` regexes on process
  `comm`; `--sort-by-rss`; `-g` to kill a process group; `--ignore-root-user`.
- **Operational hooks.** Journald logging, optional D-Bus notifications, and
  pre-kill/post-kill scripts through `-P` and `-N`.
- **Staged escalation.** SIGTERM first, SIGKILL only if pressure keeps falling.
- **tmpfs-aware accounting.** Since v1.8 it measures memory against `user mem
  total` (`MemAvailable + AnonPages`), so a filling tmpfs lowers the denominator
  and can trigger action instead of being ignored as unreclaimable cache.

## Where swapaudit wins

- **Attribution.** It names the PID holding the swap, how long it has been idle,
  and whether its parent shell died. `free`, `top`, `vmstat`, and earlyoom cannot
  answer that.
- **tmpfs cleanup.** It measures `/tmp` directories and removes stale build caches
  and abandoned test runs directly. earlyoom can only respond to tmpfs pressure by
  killing a process; it cannot free the tmpfs pages themselves.
- **Developer-aware classification.** Dormant AI coding agents, MCP servers,
  orphaned sessions, shell floods, build caches, and test workspaces are explicit
  categories. Desktop daemons and active apps are protected.
- **Safety rails.** Path confinement to `/tmp` and `/var/tmp`, ownership checks on
  files, symlink rejection, PID start-ticks re-verification immediately before
  SIGTERM (PID-reuse protection), interactive confirmation, `--dry-run`, and `-y`
  for automation.
- **No daemon, no privileges, no idle cost.** A normal user can audit and clean
  their own footprint. Nothing is installed as a service.
- **Actionable output.** Sorted by size or age, with reclaim totals and per-group
  prompts. Built for a human doing triage, not just for a log.
- **Self-upgrade.** `swapaudit upgrade` streams the release binary, verifies its
  SHA256 checksum, and replaces the executable atomically.

## Where each falls short

### earlyoom

- It cannot explain *why* swap is high or *which* process is dormant. Its unit of
  analysis is the whole machine.
- Victim selection is "highest `oom_score`", which often means the largest active
  application — an IDE, browser, or database — rather than the stale agent that
  caused the pressure.
- It frees memory by killing processes, not by removing files. Build caches and
  abandoned workspaces are invisible to it.
- A machine with a large swap area can sit at high swap usage indefinitely without
  crossing the "both below threshold" condition.
- Policy is global and regex-based; there is no notion of per-user or per-project
  cleanup.

### swapaudit

- It is not a daemon. If you never run it, it never helps, and it will not save you
  from a sudden allocation spike.
- It only acts on its known categories (agents and stale `/tmp` artifacts). It is
  not a general-purpose reclaimer and will not kill an arbitrary memory hog.
- It has no continuous pressure monitoring, no PSI integration, and no `oom_score`
  awareness.
- It sends SIGTERM but has no SIGKILL escalation. A wedged process can ignore it.
- It is Linux-only and `/proc`-dependent, as is earlyoom.
- The `--clean-*` flags are destructive by design and need care when automated.

## Using them together

The tools compose cleanly because they operate at different layers.

1. **earlyoom as the guardrail.** Run it as a service with thresholds tuned to the
   machine, and protect processes you never want killed:

   ```bash
   earlyoom -m 10 -s 10 --avoid '^(Xorg|gnome-shell|plasmashell|systemd)$'
   ```

   Regexes match `/proc/<pid>/comm`, which is truncated to 15 bytes.

2. **swapaudit as the periodic hygienist.** Inspect first, then automate only the
   cleanup groups you trust:

   ```bash
   swapaudit --clean-all --dry-run
   swapaudit --clean-untouched --clean-abandoned -y
   ```

   Keep `--clean-dormant` and `--clean-orphaned` manual, or use a generous idle
   threshold, if an agent session might still be needed.

3. **Order of operations.** If the machine is already sluggish, run swapaudit first
   to reclaim stale tmpfs data and dormant sessions without killing active work. If
   memory is falling off a cliff, earlyoom is the only one of the two that will act
   in time.

## Which one should you use?

| Situation | Recommendation |
| :-------- | :------------- |
| Unattended server, CI runner, appliance | earlyoom (or systemd-oomd) |
| Developer workstation with heavy swap and many agents | swapaudit |
| Laptop that occasionally hard-locks under memory pressure | earlyoom as the safety net, swapaudit for periodic cleanup |
| You need to explain where swap went | swapaudit |
| You need a kill to happen automatically at 3 a.m. | earlyoom |
| You want to remove gigabytes of stale `/tmp` build output | swapaudit |
| You want one tool and no daemon | swapaudit, accepting that cleanup is manual |
| You want one tool and no manual intervention | earlyoom, accepting that it kills rather than cleans |

## References

- swapaudit: [README](../../README.md), [`internal/proc`](../../internal/proc/proc.go),
  [`internal/triage`](../../internal/triage/triage.go),
  [`internal/cleaner`](../../internal/cleaner/cleaner.go),
  [`internal/cache`](../../internal/cache/cache.go)
- earlyoom: [README](https://github.com/rfjakob/earlyoom),
  [man page](https://github.com/rfjakob/earlyoom/blob/master/MANPAGE.md),
  [systemd unit](https://github.com/rfjakob/earlyoom/blob/master/earlyoom.service.in)

earlyoom behavior verified against v1.9.0 sources (2025-09-16).

## Related comparisons

- [All comparisons](README.md)
- [swapaudit vs. systemd-oomd](systemd-oomd.md)
