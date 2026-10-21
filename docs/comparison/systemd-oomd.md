# swapaudit vs. systemd-oomd

They solve different halves of the same problem. **systemd-oomd** is a system
service that watches pressure stall information (PSI) and swap usage, then
SIGKILLs a cgroup before the kernel OOM killer fires. **swapaudit** is an
on-demand auditor that attributes swap and tmpfs usage to individual processes
and directories, then reclaims only what you select.

One asks *"which cgroup is stalling?"*. The other asks *"who is holding the
memory, and what is safe to reclaim?"*.

## At a glance

| Dimension | swapaudit | systemd-oomd |
| :-------- | :-------- | :----------- |
| Role | On-demand auditor and surgical cleanup tool | Always-on userspace OOM killer |
| Operating model | One-shot CLI, no service | systemd service, polls PSI continuously |
| Trigger | You invoke it | Memory pressure or swap usage crosses configured limits |
| Signal source | Per-PID `VmSwap`, idle time, parent PID, `/tmp` size and mtime | Cgroup PSI `full avg10`, system-wide memory and swap usage |
| Granularity | Individual PID or directory | Cgroup (every process under it) |
| Default policy | 12h stale `/tmp` artifacts, 2h idle agents | 60% pressure for 30s; 90% memory and swap used |
| Target selection | Classified dormant/orphaned agents and stale `/tmp` entries | Descendant cgroup with the most reclaim activity, or highest swap usage |
| Action | SIGTERM selected agents, remove selected directories | SIGKILL every process in the selected cgroup |
| Graceful shutdown | Yes, SIGTERM | No, SIGKILL only |
| Dry run | `--dry-run` | `--dry-run` (systemd v253+) |
| Confirmation | Interactive `[y/N]`, bypass with `-y` | None; daemon acts autonomously |
| Requirements | Linux with `/proc` | systemd, cgroups v2, memory accounting, PSI (Linux 4.20+) |
| Privileges | Normal user | `systemd-oom` user with `CAP_KILL` and `CAP_DAC_OVERRIDE` |
| Footprint | One-shot, sub-second, no idle cost | Small daemon, `MemoryMin`/`MemoryLow` 64M, `OOMScoreAdjust=-900` |
| Observability | Rich terminal report with categories and reclaim totals | Journal logs and `oomctl dump` |
| Configuration | CLI flags, `SWAPAUDIT_NO_UPDATE_CHECK` | Unit properties, `oomd.conf`, drop-ins |
| Distribution | Static binary, `go install`, self-upgrade | Built into systemd 247+; default on Fedora 34+ and Ubuntu 22.04 Desktop |
| Implementation | Go standard library, zero dependencies | C, part of systemd |
| License | WaqfDPL-1.0 or AGPL-3.0 | LGPL-2.1-or-later |
| Best fit | Developer workstations, build boxes, swap and tmpfs triage | systemd-managed desktops and servers needing an OOM guardrail |

## The core difference: cgroup pressure vs. per-process attribution

systemd-oomd does not look at free memory as a percentage. It polls PSI, which
reports the fraction of time tasks were delayed waiting for memory. A cgroup's
memory pressure is the `full avg10` value: the share of a 10-second window in
which **all** tasks in that cgroup were stalled.

Units opt in with `ManagedOOMMemoryPressure=kill`. When the monitored cgroup's
pressure exceeds the configured limit for the configured duration, systemd-oomd
picks an eligible descendant cgroup — starting with the one doing the most
reclaim activity — and sends SIGKILL to every process in it. The default limit
is 60% for 30 seconds; Fedora's user-session policy lowers the limit to 50%.

Swap-based action is separate. With `ManagedOOMSwap=kill` and the default
`SwapUsedLimit=90%`, systemd-oomd acts when **both** system memory usage and swap
usage exceed 90%. It then targets eligible descendant cgroups whose swap usage is
greater than 5% of total swap, highest swap usage first.

That is real attribution, but at cgroup granularity. systemd-oomd never asks
whether a process is idle, orphaned, or holding pages that were swapped out days
ago. It asks whether a cgroup is stalling right now, and it kills the whole
cgroup rather than a single process.

swapaudit inverts the question. It walks `/proc` and attributes swap per PID
using `VmSwap`, reads idle time from `/proc/<pid>/sched`, checks whether the
parent shell is gone, and separates known agent processes from desktop daemons
and active apps. It also walks `/tmp`, measures directory sizes, and treats the
newest recursive mtime as the true last-activity time. Only then does it classify
occupants into actionable groups.

```
               systemd-oomd                          swapaudit
      +-----------------------------+      +-----------------------------+
      | cgroup PSI + swap usage     |      | per-PID VmSwap + idle       |
      | + systemd unit policy       |      | + /tmp size and mtime       |
      +--------------+--------------+      +--------------+--------------+
                     |                                    |
          rank cgroups by reclaim                classify + confirm
          activity or swap usage                            |
                     |                         +----------+----------+
                     v                         |                     |
            SIGKILL whole cgroup        prune stale /tmp      SIGTERM idle
                                        build caches          agent sessions
```

## Where systemd-oomd wins

- **PSI is a better pressure signal than free-memory percentages.** It measures
  actual stalls, accounts for page cache and reclaim, and respects cgroup memory
  protections such as `memory.low`. A system can have little "free" memory and
  still be healthy; PSI distinguishes that from real thrashing.
- **Cgroup granularity matches systemd units.** It can target one application
  scope or service instead of guessing at a single process. Rogue workloads can
  be isolated with `systemd-run --user --scope`.
- **It is already there.** systemd-oomd ships with systemd 247+ and is enabled by
  default on Fedora 34+ and Ubuntu 22.04 Desktop. No extra daemon to install.
- **Per-unit policy.** `ManagedOOMMemoryPressure=`, `ManagedOOMMemoryPressureLimit=`,
  `ManagedOOMMemoryPressureDurationSec=`, and `ManagedOOMSwap=` let each unit opt
  in with its own thresholds. `ManagedOOMPreference=avoid|omit` protects critical
  units (with the caveats around xattrs and cgroup ownership).
- **Swap-pressure ranking.** Unlike a pure pressure monitor, it can rank cgroups
  by swap usage and kill the biggest cgroup-level swap consumer when the system
  is nearly out of both memory and swap.
- **It survives pressure.** The service has reserved memory (`MemoryMin` and
  `MemoryLow` of 64M) and `OOMScoreAdjust=-900`, so it remains runnable while the
  rest of the system is starving.
- **Guaranteed action.** SIGKILL cannot be ignored by a wedged process.
- **Inspectable state.** `oomctl dump` shows the cgroups and system context that
  systemd-oomd is tracking, and `--dry-run` (v253+) logs what it would kill.

## Where swapaudit wins

- **Per-process attribution.** It names the PID holding the swap, how long it has
  been idle, and whether its parent shell died. `oomctl dump` shows cgroup-level
  pressure and swap use, not which process inside the cgroup owns the pages.
- **tmpfs cleanup.** It measures `/tmp` directories and removes stale build caches
  and abandoned test runs directly. systemd-oomd can react to pressure caused by a
  full tmpfs, but it cannot delete the files; killing a process does not
  necessarily free tmpfs pages whose files still exist.
- **Idle and orphan awareness.** systemd-oomd has no concept of "this agent has
  been idle for three hours" or "this session's parent shell is gone". swapaudit
  uses both as first-class classification inputs.
- **No systemd or cgroups-v2 requirement.** It works on any Linux with `/proc`,
  including systems running another init, containers with restricted cgroups, and
  processes that are not managed by systemd.
- **Works when everything shares a cgroup.** systemd-oomd can only kill an
  eligible descendant cgroup. If a whole session or service lives in one cgroup,
  the blast radius is the whole group. swapaudit targets individual PIDs.
- **Graceful action.** SIGTERM gives an agent a chance to flush state and exit
  cleanly. systemd-oomd always uses SIGKILL.
- **Confirmation and dry-run before destruction.** Files and processes are listed,
  totals are shown, and nothing happens without `-y` or an explicit `y`.
- **Human-readable triage.** The report is the product: sorted by size or age,
  with categories, reclaim totals, and per-group prompts.
- **No service and no privileges.** A normal user can audit and clean their own
  footprint without enabling or tuning a system daemon.
- **Self-upgrade.** `swapaudit upgrade` verifies a SHA256 checksum and replaces
  the binary atomically.

## Where each falls short

### systemd-oomd

- It kills a whole cgroup. On a desktop where an editor, terminal, browser, and
  build all share one session scope, the selected descendant can take the whole
  session down. The man page explicitly warns about this for `user.slice` and
  `user-$UID.slice`.
- It uses SIGKILL only. There is no graceful shutdown and no chance to save work.
- It has no per-process swap, idle-time, parent, or file-age attribution. It
  cannot tell a dormant agent from an active build inside the same cgroup.
- It does not clean files. It frees memory by killing processes; stale tmpfs
  contents remain until something removes them.
- It requires systemd, a full cgroups-v2 hierarchy, memory accounting, and PSI.
  It does nothing on non-systemd systems or cgroups-v1 hosts.
- Stock systemd defaults `ManagedOOM*` to `auto`, so a system without distro
  policy monitors nothing. The default-on behavior comes from distribution
  drop-ins, not from the daemon itself.
- Only descendant cgroups that are leaves or have `memory.oom.group=1` are
  eligible. Some workloads are effectively unkillable by it.
- Thresholds are system-wide policy. A transient burst can trigger a kill that a
  human would have preferred to wait out.
- The swap action requires both memory and swap usage to be above 90%. A machine
  with high swap usage but plenty of free memory will not trigger it.
- It is a killer, not a diagnostic tool. It does not explain where swap went.

### swapaudit

- It is not a daemon. If you never run it, it never helps, and it will not save
  you from a sudden allocation spike or a PSI stall.
- It only acts on its known categories (agents and stale `/tmp` artifacts). It is
  not a general-purpose reclaimer and will not kill an arbitrary memory hog.
- It has no PSI, cgroup, or `oom_score` awareness. It cannot see memory pressure
  or resource-control protections.
- It sends SIGTERM but has no SIGKILL escalation. A wedged process can ignore it.
- It is Linux-only and `/proc`-dependent.
- The `--clean-*` flags are destructive by design and need care when automated.

## Using them together

They compose cleanly because they operate at different layers.

1. **systemd-oomd as the guardrail.** Check what it is monitoring, then tune the
   policy for your workload:

   ```bash
   oomctl dump
   ```

   ```ini
   # /etc/systemd/system/user@.service.d/10-oomd.conf
   [Service]
   ManagedOOMMemoryPressure=kill
   ManagedOOMMemoryPressureLimit=50%
   ```

   ```ini
   # /etc/systemd/system/-.slice.d/10-oomd-swap.conf
   [Slice]
   ManagedOOMSwap=kill
   ```

   Put large builds in their own transient scope so a kill cannot take the whole
   session with it:

   ```bash
   systemd-run --user --scope --property=MemoryMax=8G make -j32
   ```

2. **swapaudit as the periodic hygienist.** Inspect first, then automate only the
   cleanup groups you trust:

   ```bash
   swapaudit --clean-all --dry-run
   swapaudit --clean-untouched --clean-abandoned -y
   ```

   Keep `--clean-dormant` and `--clean-orphaned` manual, or use a generous idle
   threshold, if an agent session might still be needed.

3. **Order of operations.** If the machine is already sluggish but not yet
   stalling, run swapaudit first to reclaim stale tmpfs data and dormant sessions
   without killing active work. If PSI is climbing and the system is stalling,
   systemd-oomd is the only one of the two that will act in time.

## systemd-oomd vs. earlyoom

Both are automatic killers, but they observe different things:

- **earlyoom** watches aggregate free memory and free swap, then kills the single
  process with the highest kernel `oom_score`.
- **systemd-oomd** watches PSI per cgroup and swap usage, then kills a whole
  cgroup, ranked by reclaim activity or swap usage.

earlyoom works without systemd or cgroups v2 and can run on almost any Linux.
systemd-oomd integrates with systemd unit policy and is already enabled on modern
Fedora and Ubuntu desktops. See the [earlyoom comparison](earlyoom.md) for the
full breakdown.

## Which one should you use?

| Situation | Recommendation |
| :-------- | :------------- |
| Modern Fedora or Ubuntu Desktop | systemd-oomd (already enabled); tune the user-session limit |
| Server managed by systemd with cgroups v2 | systemd-oomd, with per-unit pressure policy |
| Non-systemd host, container, or cgroups-v1 system | swapaudit, or earlyoom for automatic kills |
| Developer workstation with heavy swap and many agents | swapaudit for attribution and cleanup; systemd-oomd as the guardrail |
| You need to explain where swap went | swapaudit |
| You need a kill to happen automatically at 3 a.m. | systemd-oomd or earlyoom |
| You want to remove gigabytes of stale `/tmp` build output | swapaudit |
| You need a graceful SIGTERM before termination | swapaudit |
| You want one tool and no daemon | swapaudit, accepting that cleanup is manual |

## References

- systemd-oomd: [service man page](https://www.freedesktop.org/software/systemd/man/latest/systemd-oomd.service.html),
  [oomd.conf](https://www.freedesktop.org/software/systemd/man/latest/oomd.conf.html),
  [resource-control directives](https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html#ManagedOOMMemoryPressure=),
  [oomctl](https://www.freedesktop.org/software/systemd/man/latest/oomctl.html)
- Distribution enablement: [Fedora 34 change](https://fedoraproject.org/wiki/Changes/EnableSystemdOomd),
  [Ubuntu 22.04 release notes](https://discourse.ubuntu.com/t/jammy-jellyfish-release-notes/24668)
- systemd implementation details: [`units/systemd-oomd.service.in`](https://github.com/systemd/systemd/blob/main/units/systemd-oomd.service.in),
  [`src/oom/oomd.conf`](https://github.com/systemd/systemd/blob/main/src/oom/oomd.conf)
- swapaudit: [README](../../README.md), [`internal/proc`](../../internal/proc/proc.go),
  [`internal/triage`](../../internal/triage/triage.go),
  [`internal/cleaner`](../../internal/cleaner/cleaner.go),
  [`internal/cache`](../../internal/cache/cache.go)

systemd-oomd behavior verified against systemd 262 documentation (2026).

## Related comparisons

- [All comparisons](README.md)
- [swapaudit vs. earlyoom](earlyoom.md)
