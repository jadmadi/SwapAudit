# Safety contract

SwapAudit can delete files, send `SIGTERM`, disable and re-enable swap areas,
and write to `/proc/sys/vm/drop_caches`. This document defines what it is
allowed to do, what it refuses to do, and how to verify the destructive paths.

## Non-negotiable rules

- `--dry-run` must never call `os.RemoveAll`, `syscall.Kill`, `swapoff`,
  `swapon`, `sync`, `drop_caches`, or any other mutating command.
- Process targets must belong to the current real UID. There is no cross-user
  override.
- Orphan cleanup is never implicit. `--clean-all` covers untouched caches,
  abandoned runs, and dormant agents only. Use `--clean-orphaned` explicitly.
- Ambiguous orphan evidence must fail closed. A process is not orphaned merely
  because its parent is `systemd` or `init`.
- File cleanup must stay inside `/tmp/`, `/var/tmp/`, or `os.TempDir()`, reject
  symlink targets, and require current-UID ownership.
- Cache cleanup does not yet inspect open file descriptors, process working
  directories, or file-backed mappings. Do not prune a cache directory while a
  build or test is actively using it. This is a known gap, not a guarantee.
- A process is skipped if its start ticks changed, its UID cannot be read or
  does not match the current real UID, or it no longer exists.
- Cleanup from a non-TTY stdin requires `--dry-run` or `--yes`. Otherwise the
  command exits `2` before scanning or mutating anything.
- SwapAudit does not verify that an agent persists conversations to disk. Treat
  `SIGTERM` as potentially destructive to unsaved in-memory state.
- Swap drain requires effective UID `0`, a feasible headroom calculation, and a
  successful `swapon` for every device that was disabled. A device left disabled
  produces exit `3` and a recovery command.

## What each action does

| Action | Tool call | Reversible? |
| --- | --- | --- |
| Prune a cache directory | `os.RemoveAll` inside a confined path | No |
| End a dormant agent | `SIGTERM` after UID and start-ticks checks | No; agent-specific |
| End an orphaned agent | `SIGTERM` after UID and start-ticks checks | No; agent-specific |
| Drain swap | `swapoff <device>` then `swapon [-p priority] <device>` | Yes if RAM headroom holds; the executor restores every device |
| Drop page cache | `sync()` then write `3` to `/proc/sys/vm/drop_caches` | Not applicable; cache is regenerated |
| Dry-run | Prints the planned action only | Yes |

SwapAudit does not send `SIGKILL`, does not kill process groups, does not
install a daemon or background service, and does not write kernel tunables such
as `swappiness` or `vfs_cache_pressure`.

## Zero-swap visibility

Agents are scanned regardless of `VmSwap`. A zero-swap orphan or dormant agent
appears in the agent population, the orphan status line, and the actionable
table with `0 B` swap. Ending it is an explicit process action, not a
memory-reclaim claim.

## Non-interactive behavior

| Flags | TTY | Behavior | Exit |
| --- | --- | --- | --- |
| cleanup + `--dry-run` | any | Preview; no prompt; no mutation | 0 |
| cleanup + `--yes` | any | Execute; no prompt | 0 |
| cleanup, neither | yes | Prompt per group | 0 |
| cleanup, neither | no | `error: cleanup requires --dry-run or --yes when stdin is not a terminal` | 2 |
| reclaim + `--yes` | any | Execute; no prompt | 0/3 |
| reclaim, neither | yes | Prompt once | 0 |
| reclaim, neither | no | `error: reclaim requires --yes when stdin is not a terminal` | 2 |
| `--dry-run` without cleanup | any | Warning; normal audit | 0 |
| `--yes` without cleanup | any | Warning; normal audit | 0 |

A declined group counts as `user-declined` and exits `0`; the audit succeeded and
the user chose not to act. Per-target failures are printed but do not change the
audit exit code. `reclaim` uses exit `3` when a swap device could not be
restored or a reclaim command failed.

## Kill-time rechecks

For every process target, in order:

1. Refuse PID `<= 1` and the auditor's own PID.
2. Re-read start ticks. A vanished process is `process-gone`; a changed value is
   `start-ticks-mismatch`.
3. Re-read the real UID. An unreadable UID is `uid-unreadable`; a different UID
   is `uid-mismatch`.
4. Send `SIGTERM` (skipped in dry-run).

Parent liveness and orphan evidence are evaluated during the scan by
`ClassifyOrphan`. They are not re-read at kill time.

## Cache deletion recheck

`PruneTmpTargets` verifies each target immediately before `RemoveAll`:

- The cleaned path is absolute and under `/tmp/`, `/var/tmp/`, or
  `os.TempDir()`.
- `Lstat` succeeds and the target is not a symlink.
- `Stat_t.Uid` equals `os.Getuid()`.
- The target is not the temp root itself.

There is no open-descriptor index at this revision. A cache that is already open
by a running build can still be removed. Use `--dry-run` and review the target
list before pruning on a busy workstation.

## UID ownership

The comparison is the real UID (`os.Getuid()`), not the effective UID:

- A normal user run only targets processes owned by that user.
- `sudo swapaudit` can still target the invoking user's processes because the
  real UID is preserved; root-owned and other-user processes are skipped.
- There is no `--all-users` override. Cross-user signalling is not supported.

## Orphan evidence

`ClassifyOrphan` uses three signals:

1. The parent process is alive and is not `systemd`/`init` → not orphaned.
2. The cgroup path is a `.service` and not a `.scope` → managed service, not
   orphaned.
3. The process has a non-zero controlling terminal (`tty_nr`) and a
   `.scope`/`session-` cgroup → plausible reparented terminal session.

Anything else is `ambiguous-evidence` and is not cleanable. This is deliberately
conservative. It may miss a real orphan on a non-systemd init system or in a
container; a missed cleanup is safer than a false-positive kill.

Live evidence on the reference host: `ollama serve` has UID 999, PPID 1,
`/system.slice/ollama.service`, and `tty_nr=0`, so it is a service, not an
orphaned agent. `muse-bin` has a `.scope` cgroup but `tty_nr=0`, so it is not
orphaned either.

## Agent persistence

SwapAudit sends `SIGTERM` and does not inspect the agent's conversation store.
Test your agent with a disposable session before using `--clean-dormant` or
`--clean-orphaned`.

## Swap drain safety

See [`reclaim.md`](reclaim.md) for the full contract. The invariants are:

- Feasibility is `MemAvailable - used swap - reserve >= 0`.
- If the full footprint does not fit, only a subset whose used bytes fit the
  budget is selected; the rest remains active and is reported as skipped.
- Every device is re-enabled before the next device is disabled.
- Headroom is re-read before each `swapoff`.
- zram is skipped unless `--include-zram` is explicit.
- A failed or interrupted restore exits `3` and prints the exact `swapon`
  recovery command.
- No swap options from `/etc/fstab` are replayed; the device and priority are.

## Recommended workflow

```bash
# 1. Preview everything
swapaudit --dry-run --clean-all
swapaudit reclaim --dry-run

# 2. Review process targets, including orphan evidence
swapaudit --dry-run --clean-dormant
swapaudit --dry-run --clean-orphaned

# 3. Execute one category at a time as your normal user
swapaudit --clean-untouched
swapaudit --clean-abandoned
swapaudit --clean-dormant

# 4. Clean swap without rebooting
sudo swapaudit reclaim --drain-swap

# 5. Optionally release clean page cache
sudo swapaudit reclaim --drop-caches
```

Avoid `--clean-orphaned` on systems with a non-systemd init or an unusual
cgroup layout unless you have verified the heuristic. Avoid `--clean-all` as
root until you have reviewed a dry-run. In a pipeline, pass `--dry-run` or
`--yes` explicitly.

## Manual verification protocol

The automated tests cover the classifier, the UID gate, the reclaim math, and
the executor contract with an injected fake. To verify the live systemd-service
path on a disposable host:

```bash
systemd-run --user --unit=swapaudit-safety-test.service sleep 300
systemctl --user show -p MainPID swapaudit-safety-test.service
# Inspect the PID's Uid, tty_nr, and cgroup:
grep '^Uid:' /proc/<pid>/status
awk '{print $7}' /proc/<pid>/stat
cat /proc/<pid>/cgroup
# The cgroup should end in .service and tty_nr should be 0.
# A dry-run orphan cleanup must not classify it as cleanable:
./swapaudit --dry-run --clean-orphaned --no-color --ascii
systemctl --user stop swapaudit-safety-test.service
systemctl --user reset-failed swapaudit-safety-test.service
```

For the swap-drain path, use the disposable swap-file protocol in
[`reclaim.md`](reclaim.md#verification-protocol).
