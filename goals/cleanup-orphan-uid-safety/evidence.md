# Evidence: cleanup-orphan-uid-safety

Verdict: PASS

Date: 2026-10-19
Commit: `ef471b1` (`fix: harden orphan detection and cleanup ownership`)
Repository: `github.com/jadmadi/SwapAudit` (private)

## Live `/proc` evidence

```text
$ grep '^Uid:' /proc/self/status
Uid:	1000	1000	1000	1000

$ awk '{print $7}' /proc/self/stat
0

$ cat /proc/self/cgroup
0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-org.kde.konsole-6463.scope

$ cat /proc/1/comm
systemd

# A live agy process
$ grep '^Uid:' /proc/1271524/status
Uid:	1000	1000	1000	1000
$ awk '{print $7}' /proc/1271524/stat
34822
$ cat /proc/1271524/cgroup
0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-org.kde.konsole-6463.scope
```

This supports the heuristic: a real tty plus a `.scope` cgroup is a terminal session; a `.service` cgroup with no tty is a managed service.

## Disposable systemd service test

```text
$ systemd-run --user --unit=swapaudit-safety-test.service sleep 300
$ systemctl --user show -p MainPID --value swapaudit-safety-test.service
708622
$ grep '^Uid:' /proc/708622/status
Uid:	1000	1000	1000	1000
$ awk '{print $7}' /proc/708622/stat
tty_nr=0
$ cat /proc/708622/cgroup
0::/user.slice/user-1000.slice/user@1000.service/app.slice/swapaudit-safety-test.service

$ ./swapaudit --dry-run --clean-orphaned --no-update-check --no-color --ascii --limit 1
Group: Orphaned Agent Processes - Nothing cleanable found.
Cleanup summary (dry-run): 0 would prune, 0 would end, 0 skipped
```

The service was stopped and reset after the check.

## Test and quality-gate evidence

```text
$ go test -race ./...
ok  	github.com/jadmadi/swapaudit/cmd/swapaudit
ok  	github.com/jadmadi/swapaudit/internal/app
ok  	github.com/jadmadi/swapaudit/internal/cache
ok  	github.com/jadmadi/swapaudit/internal/cleaner
ok  	github.com/jadmadi/swapaudit/internal/model
ok  	github.com/jadmadi/swapaudit/internal/proc
ok  	github.com/jadmadi/swapaudit/internal/term
ok  	github.com/jadmadi/swapaudit/internal/triage
ok  	github.com/jadmadi/swapaudit/internal/updater
ok  	github.com/jadmadi/swapaudit/internal/version

$ go vet ./...
(no output)

$ ./scripts/check.sh --full
✔ Diff sentry clean.
✔ Claims sentry clean.
✔ All Go files formatted.
✔ go vet passed cleanly.
✔ Race-detected tests passed.
✔ Built binary self-verified.
✔ All checks passed in 9s.

Coverage after the change:
internal/app     84.9%
internal/cleaner 81.0%
internal/proc    82.1%
internal/updater 46.4%
total            75.1%
```

## UID ownership evidence

The other-UID path is covered by the injected `readProcessUID` seam in `internal/cleaner/cleaner_test.go`:

- `TestKillProcessTargetsUIDMismatch`: a target owned by another UID is skipped with `uid-mismatch`.
- `TestKillProcessTargetsAllUsersDryRun`: `AllUsers` allows a dry-run preview and prints the warning once.
- `TestKillProcessTargetsUIDUnreadable`: an unreadable UID fails closed with `uid-unreadable`.
- `TestKillProcessTargetsUnsafeGuards`: PID 0, 1, -1, and self are skipped.
- `TestResolveCleanupFlags`: `--clean-all` excludes orphaned cleanup; `--include-orphaned` opts in.
- `TestRunCleanAllExcludesOrphanGroup` / `TestRunCleanAllIncludeOrphaned`: CLI group behavior.

A live root/other-UID container test was not run in this environment; the seam tests exercise the same decision path without signaling a real process. This is recorded as a deviation, not as a live root test.

## CLI smoke checks

```text
$ ./swapaudit --help | grep -E -- '--all-users|--include-orphaned|--clean-all|--clean-orphaned'
  --clean-orphaned          End evidence-gated orphaned agent processes
  --clean-all               Execute untouched + abandoned + dormant cleanup
  --include-orphaned        Include orphaned cleanup with --clean-all
  --all-users               Allow signaling processes owned by other UIDs

$ ./swapaudit --dry-run --clean-all --no-update-check --no-color --ascii --limit 1 | grep -c 'Orphaned Agent Processes'
0

$ ./swapaudit --dry-run --clean-all --include-orphaned --no-update-check --no-color --ascii --limit 1 | grep -c 'Orphaned Agent Processes'
1

$ ./swapaudit --dry-run --clean-all --no-update-check --no-color --ascii --limit 1 | grep 'Cleanup summary'
Cleanup summary (dry-run): 93 would prune, 5 would end, 0 skipped
```

## Files changed

- `internal/proc/proc.go`, `internal/proc/proc_test.go`
- `internal/cleaner/cleaner.go`, `internal/cleaner/cleaner_test.go`
- `internal/app/app.go`, `internal/app/app_test.go`, `internal/app/display.go`
- `README.md`, `GOVERNANCE.md`, `CHANGELOG.md`
- `docs/safety.md`
- `scripts/check.sh`

## Deviations

- The live root/other-UID container test was replaced by the deterministic injected-UID tests. The systemd-service live test was run and passed.
- The plan's "do not push" line was superseded by the user's explicit instruction to implement and publish to the private repo.
