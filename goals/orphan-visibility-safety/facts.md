# Facts: Make Orphaned and Dormant Agent Visibility Complete and Fail-Closed

## Architectural Invariants & Constraints

- **Visibility is independent of swap.** An agent must be discovered, classified, and counted even when `VmSwap == 0`. Swap bytes are a reclaim estimate, not a discovery filter.
- **Ambiguous evidence never authorizes a signal.** If parent liveness, orphan evidence, idle evidence, UID, or start ticks are missing or changed, skip the target.
- **Re-check at the last responsible moment.** UID, parent, orphan evidence, idle, and start ticks are re-read immediately before `SIGTERM`, not trusted from the scan snapshot.
- **Orphan cleanup is opt-in.** `--clean-all` never includes `--include-orphaned`; the existing safety goal's decision is preserved.
- **No SIGKILL.** The tool sends `SIGTERM` only, prints it, and does not retry or escalate.
- **No silent non-TTY skip.** A cleanup request without `--dry-run`/`--yes` and without a terminal is a usage error (exit 2), not a silent success.
- **Zero rows are information.** `abandoned-agents | 0` is rendered; absence is not a valid empty state.
- **No real process kills in tests.** Use seams and non-existent PIDs.
- **No new dependency.** Standard library only.

## Observed Facts (Repository)

| Fact | Evidence | Consequence |
| --- | --- | --- |
| `CollectSwapItems` drops `VmSwap == 0` | `internal/app/app.go:63-65` | Zero-swap agents are invisible to display and cleanup |
| `ClassifyOrphan` evidence is discarded | `internal/app/app.go:75` | No reason/parent/tty/cgroup in the item |
| `abandoned-agents` is rendered only when count > 0 | `internal/app/display.go:368-369` | Zero is indistinguishable from broken |
| Empty state is only `nothing to reclaim` | `internal/app/display.go:67-69` | No agent/orphan status |
| Idle is `-1` when `/proc/[pid]/sched` is unreadable | `internal/proc/proc.go:331-336` | Silently not dormant; no `idle-unknown` count |
| `CategorizeProcess` requires `lastActive >= 0` for dormant | `internal/triage/triage.go:47-50` | Unknown idle → active, not cleanable |
| Kill path checks UID | `internal/cleaner/cleaner.go:234-250` | Already fixed; do not remove |
| Kill path re-checks start ticks | `internal/cleaner/cleaner.go:252-263` | Already fixed; do not remove |
| Kill path sends SIGTERM only | `internal/cleaner/cleaner.go:264-268` | No SIGKILL; output does not say so |
| Non-TTY `AskConfirmation` returns false on EOF | `internal/cleaner/cleaner.go:81-89` | Silent skip and exit 0 in pipelines |
| `--clean-all` excludes orphaned | `internal/app/app.go:119-137`, `:229-234` | Already fixed; do not re-fix |
| Dormant prompt warns about unsaved state | `internal/cleaner/cleaner.go:353` | Already fixed; keep the warning |
| `ResolveCleanupFlags` is pure and tested | `internal/app/app.go:119-137` | Extend rather than replace |

## Observed Facts (Live Host, 2026-10-19)

| Observation | Command | Value |
| --- | --- | --- |
| Dormant agents | `./swapaudit --no-color --ascii` | 5 (`agy` 582.3 MB, chrome-devtools 89.2/71.0 MB, agy 50.2/22.1 MB) |
| Orphaned agents | same | 0; `abandoned-agents` row absent |
| Agent processes | `ps -eo pid,ppid,comm,stat --sort=-rss | grep -E 'agy|devin|chrome-devtools'` | 12+ rows, most with live parents |
| `agy` zero-swap candidates | `for p in $(pgrep -x agy); do awk '/^VmSwap:/{print p,$2}' p=$p /proc/$p/status; done` | Some agents may have 0 kB; record exactly |
| `chrome-devtools` parents | `ps -o pid,ppid,comm -p $(pgrep chrome-devtools | tr '\n' ',')` | Likely live `agy` or shell parents; record |
| Orphan evidence for each agent | `cat /proc/$p/cgroup`; `awk '{print $7}' /proc/$p/stat` | Record reason strings |
| Process with unreadable sched | `test -r /proc/$p/sched` | Record; `hidepid` not verified |
| `SIGKILL` references | `grep -R "SIGKILL" internal` | None expected |

These are single-host observations. The goal must prove the zero-swap path with fixtures, not depend on this host having a zero-swap agent.

## External Facts

- `/proc/[pid]/status` exposes `VmSwap` (swap used) and `VmRSS` (resident set). A process can have zero of either.
- `/proc/[pid]/sched` may be unreadable under `hidepid` or container restrictions; `se.exec_start` is a scheduling timestamp, not a user-interaction timestamp.
- `/proc/[pid]/stat` field 4 is `ppid`; field 7 is `tty_nr`; field 22 is `starttime`.
- `/proc/[pid]/cgroup` on cgroup v2 exposes the unified path as `0::<path>`.
- `SIGTERM` is a request; a process may ignore it. The tool does not escalate to `SIGKILL`.
- A process can be reparented to PID 1 or a subreaper after its parent dies. The existing `ClassifyOrphan` requires `.scope`/`session-` and a non-zero tty to reduce false positives.
- `os.Stdin.Stat().Mode()&os.ModeCharDevice` is a practical TTY check for the non-interactive guard; a stricter `isatty` ioctl can be used if needed.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| Zero-swap agents are still useful to clean up (they consume PIDs/RAM, not swap) | The user asked for them to be visible | Show `0 B swapped`; keep cleanup opt-in |
| A live non-systemd parent makes a dormant kill risky | The parent may own the child or the session | Skip by default; `--allow-live-parent` explicit |
| Orphan evidence can change between scan and kill | Parent can die or appear; cgroup/tty can change | Re-run `ClassifyOrphan` at kill time; test seam |
| Idle evidence can become unreadable | `/proc` permissions can change; process can exit | Re-read; skip `idle-unknown` |
| A TTY check is enough to distinguish interactive from scripted cleanup | The CLI is a terminal tool | Use `ModeCharDevice`; test with a buffer/pipe |
| The current agent list is intentionally narrow | Governance/feature freeze | Do not add runtimes in this goal |

## Unknowns

- How many zero-swap agents exist on a typical developer host. **Unknown.** The population line measures it.
- Whether `se.exec_start` is a reliable "last activity" proxy for all agent runtimes. **Unknown.** The threshold and the evidence display make it inspectable; do not claim precision.
- Whether a reparented agent can retain a `.service` cgroup and a non-zero tty. **Unknown.** The existing heuristic fails closed on `.service` without `.scope`.
- Whether a process can disappear between the parent read and the signal. **Unknown.** Treat `ESRCH` as `process-gone`; skip.
- Whether `os.Stdin.Stat()` is reliable in all container/CI environments. **Acceptable risk.** `--dry-run`/`--yes` are the explicit escape hatches.

## Evidence Commands

```bash
grep -n "swapBytes <= 0\|ClassifyOrphan\|CategorizeProcess" internal/app/app.go
grep -n "counts\[c\] > 0\|nothing to reclaim\|abandoned-agents\|End orphaned" internal/app/display.go
grep -n "lastActive >= 0\|CatDormantAgent\|CatAbandonedAgent" internal/triage/triage.go
grep -n "func IsProcessOrphaned\|func ClassifyOrphan\|ReadProcessLastActive" internal/proc/proc.go
grep -n "AskConfirmation\|SIGTERM\|SIGKILL\|parent\|startTicks" internal/cleaner/cleaner.go
for p in $(pgrep -f 'agy|devin|chrome-devtools'); do echo "== $p"; grep -E '^(Name|VmSwap|VmRSS|Uid|PPid):' /proc/$p/status; awk '{print "tty="$7}' /proc/$p/stat; cat /proc/$p/cgroup; done
go test -race ./...
```

## File & Interface Contracts

- `internal/proc`: `ProcessSnapshot`, `ScanProcesses`, `ReadParentInfo`, `ParseVmRSS`; keep `ClassifyOrphan`, `ReadProcessLastActive`, `ReadStartTicks`.
- `internal/triage`: `IsAgent`, `ClassifyAgent`, `AgentPopulation`, `BuildAgentPopulation`; keep `AggregateShells`.
- `internal/model`: add `IdleKnown`, `OrphanReason`, `ParentPID`, `ParentComm`, `ParentAlive`, `TTYNr`, `Cgroup`, `Signal`.
- `internal/app`: `CollectAuditData` (items + population), `--allow-live-parent`, `--explain-agents`, non-TTY guard, warnings.
- `internal/app/display.go`: zero row, population line, orphan status line, evidence table, signal text.
- `internal/cleaner`: `readParentInfo`, `classifyOrphan`, `readIdle`, `killProcess` seams; parent/orphan/idle rechecks; new skip reasons; SIGTERM text.
- `docs/safety.md`: the V1–V6 contract.
- `README.md`, `CHANGELOG.md`: flags, visibility, non-TTY behavior.
- `go.mod`: unchanged.
