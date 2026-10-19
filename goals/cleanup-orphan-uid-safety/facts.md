# Facts: Harden Orphan Detection, Process UID Safety, and Destructive Cleanup Claims

## Architectural Invariants & Constraints

- Process termination must fail closed: ambiguous evidence must never authorize a signal.
- A process owned by another UID must never be signaled unless `--all-users` is explicitly set.
- `--clean-all` must never imply orphan cleanup.
- `--dry-run` must exercise the same eligibility checks as a live run, but must not call `syscall.Kill` or `os.RemoveAll`.
- File cleanup semantics must not change: `/tmp`/`/var/tmp` confinement, symlink rejection, and UID checks remain.
- Zero external dependencies; tests must pass with `go test -race ./...`.
- No test may signal PID 1, the test process, or another user's live process.

## Observed Facts (Repository)

| Fact | Evidence | Consequence |
| --- | --- | --- |
| Orphan detection treats a `systemd`/`init` parent as orphaned | `internal/proc/proc.go:203-212` | A legitimate systemd-managed agent is marked `CatAbandonedAgent` and becomes cleanable |
| Process kills do not check target UID | `internal/cleaner/cleaner.go:148-161` | Root can signal another user's process; non-root gets `EPERM` but the tool attempts the signal |
| File pruning does check UID | `internal/cache/cache.go:83`; `internal/cleaner/cleaner.go:88` | The file path is safer than the process path; the asymmetry is the bug |
| `--clean-all` includes orphaned cleanup | `internal/app/app.go:227-233` | A broad cleanup command implicitly authorizes the riskiest action |
| The dormant prompt claims conversation preservation | `internal/cleaner/cleaner.go:246` | False reassurance before `SIGTERM`; no code verifies persistence |
| Agent detection covers `agy`, `devin`, `chrome-devtools`, `antigravity` | `internal/triage/triage.go:31-36` | Orphan cleanup only affects this small set; widening detection increases the risk this goal is fixing |
| The file cleanup path already has strong adversarial tests | `internal/cleaner/cleaner_test.go` | Reuse the same test style for the process path |
| Cleanup flags are booleans threaded through `ExecuteCleanup` | `internal/app/app.go:125-138`, `:227-241`; `internal/cleaner/cleaner.go:200-275` | Adding `AllUsers`/`IncludeOrphaned` without a struct creates parameter sprawl |
| The tool exits `0` for a successful audit even when nothing is cleanable | `internal/app/app_test.go` | Skipped targets should not become a failure |
| No persistent audit log exists | `grep -R "audit" internal cmd` | The summary must be console-only; a log file is a non-goal |

## Observed Facts (Live Host, 2026-10-19)

These facts justify the cgroup/tty heuristic. They are observations from this host, not universal guarantees.

| Observation | Command | Value |
| --- | --- | --- |
| Current shell process UID | `grep '^Uid:' /proc/self/status` | `Uid: 1000 1000 1000 1000` |
| Current shell process tty | `awk '{print $7}' /proc/self/stat` | `0` (the command ran through a pipe, not a tty) |
| Current shell cgroup | `cat /proc/self/cgroup` | `0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-org.kde.konsole-6463.scope` |
| PID 1 command | `cat /proc/1/comm` | `systemd` |
| A live `agy` process UID | `grep '^Uid:' /proc/1271524/status` | `Uid: 1000 1000 1000 1000` |
| A live `agy` process tty | `awk '{print $7}' /proc/1271524/stat` | `34822` |
| A live `agy` process cgroup | `cat /proc/1271524/cgroup` | `0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-org.kde.konsole-6463.scope` |
| A systemd user service sample | `systemctl --user list-units --type=service --no-pager` | Units end in `.service` (for example `app-com.mitchellh.ghostty.service`) |
| Toolchain | `go version` | `go1.27.2 linux/amd64` (CI/release target remains Go 1.23) |
| `strace` available | `command -v strace` | `/usr/bin/strace` |

**Inference:** a process in a `.scope` cgroup with a non-zero `tty_nr` is plausibly terminal-launched; a process in a `.service` cgroup with `tty_nr == 0` is plausibly a managed service. This is a heuristic, not proof. Fail closed when the evidence is mixed.

## External Facts

- `/proc/[pid]/status` exposes `Uid:` as four numeric fields: real, effective, saved set, and filesystem UID. The ownership comparison uses the real UID.
- `/proc/[pid]/stat` field 7 is `tty_nr`. A value of `0` means no controlling terminal; a non-zero value identifies a terminal device.
- `/proc/[pid]/cgroup` exposes the unified cgroup path on cgroup v2 systems. systemd user services and scopes use different suffixes (`.service` vs `.scope`).
- `SIGTERM` requests graceful termination; a process may ignore it. swapaudit does not send `SIGKILL`.
- Agent conversation persistence is agent-specific and is not guaranteed by `SIGTERM`.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| systemd services use `.service` cgroups and normally have no controlling terminal | Core of the fail-closed heuristic | Launch a disposable `systemd-run --user --unit=swapaudit-test.service sleep 300`; inspect cgroup/tty; confirm it is not cleanable |
| Terminal-launched agents keep a non-zero `tty_nr` after the shell dies | Distinguishes reparented sessions from daemons | Re-parent a disposable child with a pty, kill the shell, inspect `/proc`; do not use a real agent |
| Real UID is the right ownership check | Prevent cross-user signals | Run as root in a disposable container against a process owned by UID 1000 |
| Ambiguous evidence can safely return `false` | Prevents destructive false positives | Table tests for missing parent, missing cgroup, `TTYNr == 0`, `.service`, `.scope` |

## Unknowns

- Whether `agy` persists conversations to disk by default, and under which configuration. **Unknown.** The goal removes the claim rather than asserting either behavior.
- Whether agents launched by non-systemd init systems (OpenRC, runit, s6) expose a usable cgroup/tty distinction. **Unknown.** The heuristic fails closed, so the cost is missed cleanup, not a false kill.
- Whether containers with user namespaces report UIDs that match the host UID. **Unknown.** `--all-users` is the explicit escape hatch; document the risk.
- Whether a process can retain a stale non-zero `tty_nr` after its terminal disappears. **Unknown.** The confirmation prompt and dry-run are the backstop.

## Evidence Commands

```bash
grep -n "IsProcessOrphaned\|systemd\|init" internal/proc/proc.go
grep -n "syscall.Kill\|ReadStartTicks\|PID <= 1\|Conversations stay preserved" internal/cleaner/cleaner.go
grep -n "cleanAll\|cleanOrphaned\|cleanDormant" internal/app/app.go
grep -n "Uid:\|tty_nr\|cgroup" /proc/self/status /proc/self/stat /proc/self/cgroup
go test -race ./...
```

## File & Interface Contracts

- `internal/proc`: add `ParseProcStatusUID`, `ReadProcUID`, `ReadTTYNr`, `ReadProcessCgroup`, `OrphanEvidence`, `ClassifyOrphan`.
- `internal/cleaner`: add `CleanupFlags`, `CleanupOptions`, `CleanupReport`; add `AllUsers` behavior; change prompt text.
- `internal/app`: add `--all-users`, `--include-orphaned`; change `--clean-all` semantics; update help.
- `internal/model`: add `OrphanReason` only if the summary needs it; prefer deriving reasons from `OrphanEvidence` and passing them through cleaner-local structs.
- `scripts/check.sh`: add a claims sentry with a banned-phrase list.
- `docs/safety.md`: new operator-facing safety contract.
