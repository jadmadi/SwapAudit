# Evidence: cleaner safety guards

Verdict: PASS

Date: 2026-10-20. Re-verified after the liveness and kill-time recheck work touched `internal/cleaner/cleaner.go`.

## Guards still enforced

- Dry-run never calls `os.RemoveAll` or `syscall.Kill`.
- Path confinement rejects empty, relative, root, `/tmp`, `/var/tmp`, and outside-temp paths.
- Top-level symlinks and non-current-UID file targets are rejected.
- Process targets skip PID `<= 1` and the auditor's own PID.
- UID is re-read; unreadable or mismatched UID fails closed unless `--all-users`.
- Parent liveness, orphan evidence, idle evidence, and start ticks are re-read before `SIGTERM`.
- Open-handle/cwd/mapping and incomplete-evidence states skip cache deletion.
- No `SIGKILL` is referenced.

## Verification

```text
$ go test -run "TestPrune|TestKill|TestDryRun|TestParent|TestOrphan|TestIdle|TestSignal|TestCleanup|TestNoSIGKILL" -v ./internal/cleaner
=== RUN   TestPruneTmpTargetsDryRun                  --- PASS
=== RUN   TestPruneTmpTargetsUnsafeGuards            --- PASS
=== RUN   TestKillProcessTargetsUnsafeGuards         --- PASS
=== RUN   TestKillProcessTargetsDryRun               --- PASS
=== RUN   TestKillProcessTargetsUIDMismatch          --- PASS
=== RUN   TestKillProcessTargetsAllUsersDryRun       --- PASS
=== RUN   TestKillProcessTargetsUIDUnreadable        --- PASS
=== RUN   TestPruneTmpTargetsLiveExecution           --- PASS
=== RUN   TestPruneFooterWithSkips                   --- PASS
=== RUN   TestPruneSkipsOpenHandleAndOpenedSinceScan --- PASS
=== RUN   TestPruneSkipsEvidenceIncomplete           --- PASS
=== RUN   TestDryRunNoRemove                         --- PASS
=== RUN   TestParentAliveSkipAndOverride             --- PASS
=== RUN   TestOrphanEvidenceChanged                  --- PASS
=== RUN   TestIdleUnknownSkip                        --- PASS
=== RUN   TestSignalPrinted                          --- PASS
=== RUN   TestCleanupMiddleTruncation                --- PASS
=== RUN   TestCleanupResultMatchesSummary            --- PASS
=== RUN   TestNoSIGKILL                              --- PASS
PASS
ok  github.com/jadmadi/swapaudit/internal/cleaner
```
