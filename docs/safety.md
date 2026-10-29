# Safety contract

SwapAudit can delete files and send `SIGTERM`. This document defines what it is allowed to do, what it refuses to do, and how to verify the destructive paths before using them.

## Non-negotiable rules

- `--dry-run` must never call `os.RemoveAll` or `syscall.Kill`.
- Process targets must belong to the current real UID unless `--all-users` is explicitly set.
- Orphan cleanup is never implicit. `--clean-all` covers untouched caches, abandoned runs, and dormant agents only. Use `--clean-all --include-orphaned` or `--clean-orphaned` to include orphaned agents. Orphaned caches have a separate `--clean-orphaned-caches` opt-in.
- Ambiguous orphan evidence must fail closed. A process is not orphaned merely because its parent is `systemd` or `init`.
- A cache with an open descriptor, a process cwd, or a file-backed mapping is never cleanable. Orphaned caches fail closed when the `/proc` index is truncated or a same-UID fd table is unreadable.
- File cleanup must stay inside `/tmp/`, `/var/tmp/`, or `os.TempDir()`, reject symlink targets, and require current-UID ownership.
- A process is skipped if its UID cannot be read, its start ticks changed, its parent is alive when the classification requires a dead parent, its orphan evidence changed, its idle evidence is unavailable for a dormant target, or it no longer exists.
- Cleanup from a non-TTY stdin requires `--dry-run` or `--yes`. Otherwise the command exits `2` before scanning or mutating anything.
- SwapAudit does not verify that an agent persists conversations to disk. Treat `SIGTERM` as potentially destructive to unsaved in-memory state.

## What each action does

| Action | Tool call | Reversible? |
| --- | --- | --- |
| Prune a cache directory | `os.RemoveAll` inside a confined path, after a liveness recheck | No |
| End a dormant agent | `SIGTERM` after UID, parent, idle, and start-ticks checks | No; agent-specific |
| End an orphaned agent | `SIGTERM` after UID, parent, orphan-evidence, and start-ticks checks | No; agent-specific |
| Dry-run | Prints the planned action and the signal name only | Yes |

SwapAudit does not send `SIGKILL`, does not kill process groups, and does not install a daemon or background service. Dry-run and live output both print `SIGTERM` (`would-signal` and `signaled` respectively).

## Zero-swap visibility

Agents are scanned regardless of `VmSwap`. A zero-swap orphan or dormant agent appears in the agent population, the orphan status line, and the actionable table with `0 B` swap. Ending it is an explicit process action, not a memory-reclaim claim.

## Non-interactive behavior

| Flags | TTY | Behavior | Exit |
| --- | --- | --- | --- |
| cleanup + `--dry-run` | any | Preview; no prompt; no mutation | 0 |
| cleanup + `--yes` | any | Execute; no prompt | 0/3 |
| cleanup, neither | yes | Prompt per group | 0 |
| cleanup, neither | no | `error: cleanup requires --dry-run or --yes when stdin is not a terminal` | 2 |
| `--dry-run` without cleanup | any | Warning; normal audit | 0 |
| `--yes` without cleanup | any | Warning; normal audit | 0 |

A declined group counts as `user-declined` and exits `0`; the audit succeeded and the user chose not to act. Operational skips (`open-handle`, `parent-alive`, `process-gone`) also exit `0`. `kill-failed` and `remove-failed` produce exit `3` after the remaining targets are processed.

## Kill-time rechecks

For every process target, in order:

1. Refuse PID `<= 1` and the auditor's own PID.
2. Re-read the UID; skip `uid-unreadable` or `uid-mismatch` unless `--all-users`.
3. Re-read parent liveness. A live parent that is not `systemd`/`init` skips `parent-alive`; dormant targets may override only with `--allow-live-parent` and a warning. Orphaned targets always skip.
4. For orphaned targets, re-run `ClassifyOrphan`; a changed result skips `orphan-evidence-changed`.
5. For dormant targets, re-read idle evidence; unknown idle skips `idle-unknown`.
6. Re-read start ticks; a snapshot with no start ticks is `start-ticks-unknown`, a vanished process is `process-gone`, and a changed value is `start-ticks-mismatch`.
7. Send `SIGTERM` (skipped in dry-run) and print the signal.

## Cache deletion recheck

`PruneTmpTargets` rechecks the open index before `RemoveAll`:

- `target` mode for orphaned caches: a fresh index for each target.
- `group` mode for untouched and abandoned caches: one index per group.

Skip reasons: `open-handle`, `cwd-inside`, `mapped-inside`, `evidence-incomplete`, `opened-since-scan`. Dry-run performs the same checks without mutation. See [`cache-liveness.md`](cache-liveness.md).

## UID ownership

The default comparison is the real UID (`os.Getuid()`), not the effective UID. This means:

- A normal user run only targets processes owned by that user.
- `sudo swapaudit` can still target the invoking user's processes because the real UID is preserved; root-owned and other-user processes are skipped by default.
- `--all-users` is required to signal a process owned by another UID. It prints a warning once per group. Running as a non-root user with `--all-users` will usually fail with `EPERM`; the failure is reported and counted as `kill-failed`.

## Orphan evidence

`ClassifyOrphan` uses three signals:

1. The parent process is alive and is not `systemd`/`init` → not orphaned.
2. The cgroup path is a `.service` and not a `.scope` → managed service, not orphaned.
3. The process has a non-zero controlling terminal (`tty_nr`) and a `.scope`/`session-` cgroup → plausible reparented terminal session.

Anything else is `ambiguous-evidence` and is not cleanable. This is deliberately conservative. It may miss a real orphan on a non-systemd init system or in a container; a missed cleanup is safer than a false-positive kill.

The live evidence on the reference host: an `agy` process had a non-zero `tty_nr` and a `.scope` cgroup, while systemd user services use `.service` cgroups and normally have no controlling terminal.

## Agent persistence

SwapAudit sends `SIGTERM` and does not inspect the agent's conversation store. The old prompt's claim that conversations persist on disk was not verified and has been removed. Test your agent with a disposable session before using `--clean-dormant` or `--clean-orphaned`.

## Recommended workflow

```bash
# 1. Preview everything
swapaudit --dry-run --clean-all

# 2. Review cache targets
swapaudit --dry-run --clean-untouched
swapaudit --dry-run --clean-abandoned
swapaudit --dry-run --clean-orphaned-caches

# 3. Review process targets, including orphan evidence
swapaudit --dry-run --clean-dormant
swapaudit --dry-run --clean-orphaned

# 4. Execute one category at a time as your normal user
swapaudit --clean-untouched
swapaudit --clean-abandoned
swapaudit --clean-orphaned-caches
swapaudit --clean-dormant
```

Avoid `--clean-all` as root until you have reviewed a dry-run. Avoid `--clean-orphaned` on systems with a non-systemd init or an unusual cgroup layout unless you have verified the heuristic. Orphaned-cache cleanup is opt-in because an idle build cache may still be about to be reused. In a pipeline, pass `--dry-run` or `--yes` explicitly.

## Manual verification protocol

The automated tests cover the classifier and UID gates with fixtures. To verify the live systemd-service path on a disposable host:

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

For the other-UID path, use a disposable container:

```bash
# As root in a container, with a process owned by UID 1000:
./swapaudit --dry-run --clean-orphaned --no-color --ascii
# Expect uid-mismatch for the UID 1000 process.
./swapaudit --dry-run --clean-orphaned --all-users --no-color --ascii
# Expect the --all-users warning and the target to be previewed, not signaled.
```
