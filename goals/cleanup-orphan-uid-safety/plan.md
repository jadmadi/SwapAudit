# Plan: Harden Orphan Detection, Process UID Safety, and Destructive Cleanup Claims

## Execution Steps

### Phase 1 — Capture evidence and freeze the contract

- [x] Run the live `/proc` evidence commands in `facts.md` and append the exact output to `goals/cleanup-orphan-uid-safety/evidence.md`.
- [x] Launch a disposable systemd user service (`systemd-run --user --unit=swapaudit-safety-test.service sleep 300`) and record its PID, `Uid:`, `tty_nr`, and cgroup.
- [x] Confirm the service is in a `.service` cgroup and has `tty_nr == 0`; stop and remove the unit afterward (`systemctl --user stop ...`, `systemctl --user reset-failed ...`).
- [x] Record the exact current prompt text and flag behavior in the evidence file before changing them.

### Phase 2 — Add process evidence parsers

- [x] Add `ParseProcStatusUID(io.Reader) (uint32, error)` and `ReadProcUID(pid int) (uint32, error)` to `internal/proc/proc.go`.
- [x] Add `ReadTTYNr(pid int) int64` with `)`-aware `/proc/[pid]/stat` parsing; do not break `ReadStartTicks`.
- [x] Add `ReadProcessCgroup(pid int) string` returning the `0::` path when present.
- [x] Add table tests for valid, missing, malformed, and whitespace-variant `Uid:` lines.
- [x] Add integration tests for `ReadProcUID(os.Getpid())`, `ReadProcUID(1)`, `ReadTTYNr(os.Getpid())`, and `ReadProcessCgroup(os.Getpid())`.
- [x] Run `go test -race ./internal/proc`.

### Phase 3 — Replace the orphan heuristic

- [x] Add `OrphanEvidence` and `ClassifyOrphan(pid, ppid int) (OrphanEvidence, bool)`.
- [x] Implement the ordered rules from R3: parent-alive, systemd-service, no-controlling-terminal, reparented-terminal-scope, ambiguous-evidence, evidence-unavailable.
- [x] Update `internal/app/app.go` to call `ClassifyOrphan` instead of `IsProcessOrphaned`.
- [x] Add pure table tests for every rule and for mixed evidence; assert the `Reason` string.
- [x] Add a manual test protocol in `docs/safety.md` for a disposable systemd user service and a reparented terminal process.
- [x] Run `go test -race ./internal/proc ./internal/app`.

### Phase 4 — Add UID enforcement and options structs

- [x] Add `CleanupFlags` and `CleanupOptions` to `internal/cleaner/cleaner.go`.
- [x] Add `var readProcessUID = proc.ReadProcUID` as a test seam.
- [x] Refactor `ExecuteCleanup`, `KillProcessTargets`, and `PruneTmpTargets` to the new signatures; compile all call sites.
- [x] In `KillProcessTargets`, read UID before start-ticks; skip `uid-unreadable` and `uid-mismatch`; warn once per group for `AllUsers`.
- [x] Ensure `--dry-run` runs the same UID/start-ticks eligibility checks but never calls `syscall.Kill`.
- [x] Add tests: UID mismatch skipped, `AllUsers` allows dry-run, unreadable UID skipped, start-ticks mismatch still skipped, existing file guards unchanged.
- [x] Add a `CleanupReport` struct and a one-line summary with exact reason counts.
- [x] Run `go test -race ./internal/cleaner`.

### Phase 5 — Change CLI flags and `--clean-all`

- [x] Add `--all-users` (default false) and `--include-orphaned` (default false) to `internal/app/app.go`.
- [x] Add `ResolveCleanupFlags(cleanAll, cleanOrphaned, includeOrphaned bool) CleanupFlags`.
- [x] Change `--clean-all` to set untouched + abandoned + dormant only.
- [x] Make `--clean-all --include-orphaned` include orphaned cleanup.
- [x] Emit the warning when `--include-orphaned` is used without `--clean-all` or `--clean-orphaned`.
- [x] Update `PrintQuickCommands`, `PrintUsage`, and the README flag table.
- [x] Add table tests for all eight flag combinations.
- [x] Run `go test -race ./internal/app`.

### Phase 6 — Remove the unverified claim and add the safety contract

- [x] Replace the prompt in `internal/cleaner/cleaner.go` with the R7 wording.
- [x] Write `docs/safety.md` with the exact behavior, limits, and recommended workflow.
- [x] Add a claims sentry to `scripts/check.sh` for the banned phrases; keep `docs/safety.md` from quoting them verbatim.
- [x] Update `README.md` with a "Process ownership and orphan safety" subsection.
- [x] Update `GOVERNANCE.md` anti-goal #4.
- [x] Update `CHANGELOG.md` under `Changed` and `Fixed`.
- [x] Run `grep -R "Conversations stay preserved" .` and confirm no matches.

### Phase 7 — Full verification and evidence

- [x] Run `gofmt -l .` and confirm empty output.
- [x] Run `go vet ./...`.
- [x] Run `go test -race ./...`.
- [x] Run `go test -cover ./internal/cleaner ./internal/proc` and record coverage.
- [x] Run `./scripts/check.sh --full` and record the final status.
- [x] Run the manual systemd-service and other-UID protocols in a disposable container; paste the output into `evidence.md`.
- [x] Run `./swapaudit --help` and record the new flag descriptions.
- [x] Run `./swapaudit --dry-run --clean-all --no-color --ascii` and confirm no orphan group.
- [x] Run `./swapaudit --dry-run --clean-all --include-orphaned --no-color --ascii` and confirm the orphan group appears without signals.
- [x] Complete `goals/cleanup-orphan-uid-safety/evidence.md` with commands, outputs, dates, and any deviations.

### Phase 8 — Attribution commit

- [x] Commit with a subject containing `(goals/cleanup-orphan-uid-safety/goal.md)` or `Goal: cleanup-orphan-uid-safety`, using `fix:` or `feat:`.
- [x] Run `sila goals --project=/home/jad/scripts/active/swapaudit` to reconcile the goal.
- [x] Do not push to a remote; local commit only unless the user explicitly authorizes a push.

## Tests to Add

| Test | File | What it proves |
| --- | --- | --- |
| `TestParseProcStatusUID` | `internal/proc/proc_test.go` | UID parsing is strict and handles malformed input |
| `TestReadProcUIDCurrentProcess` | `internal/proc/proc_test.go` | Real `/proc` read works for the current process |
| `TestReadTTYNrAndCgroup` | `internal/proc/proc_test.go` | `tty_nr` and cgroup parsing do not panic and return usable values |
| `TestClassifyOrphanRules` | `internal/proc/proc_test.go` | Every ordered rule and fail-closed reason |
| `TestKillProcessTargetsUIDMismatch` | `internal/cleaner/cleaner_test.go` | Other-UID targets are skipped by default |
| `TestKillProcessTargetsAllUsersDryRun` | `internal/cleaner/cleaner_test.go` | `AllUsers` allows preview and warns once |
| `TestKillProcessTargetsUIDUnreadable` | `internal/cleaner/cleaner_test.go` | Unreadable UID fails closed |
| `TestCleanupSummaryReasons` | `internal/cleaner/cleaner_test.go` | Exact summary counts and reason names |
| `TestResolveCleanupFlags` | `internal/app/app_test.go` | `--clean-all` excludes orphan; `--include-orphaned` includes it |
| `TestRunCleanAllNoOrphanGroup` | `internal/app/app_test.go` | Default `--clean-all` output has no orphan group |
| `TestRunCleanAllIncludeOrphaned` | `internal/app/app_test.go` | Explicit opt-in output includes the group in dry-run |
| `TestHelpMentionsAllUsers` | `internal/app/app_test.go` | Help text matches the new flags |
| Claims sentry | `scripts/check.sh` | Banned unverified phrases cannot reappear |

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -cover ./internal/cleaner ./internal/proc
./scripts/check.sh --full
grep -R "Conversations stay preserved" . || true
./swapaudit --help | grep -E -- '--all-users|--include-orphaned'
./swapaudit --dry-run --clean-all --no-color --ascii | grep -c "Orphaned Agent Processes" || true
./swapaudit --dry-run --clean-all --include-orphaned --no-color --ascii | grep -c "Orphaned Agent Processes" || true
```

## Rollback

- Revert the implementation commit. No data migration is involved.
- If the systemd/tty heuristic is too conservative, keep the UID and opt-in fixes and widen the evidence rules in a new goal with a fresh manual test.
- If the claims sentry blocks release documentation, narrow the banned-phrase list; do not reintroduce the claim.

## Completion Evidence

Before certifying, `goals/cleanup-orphan-uid-safety/evidence.md` must contain:

1. The exact live `/proc` observations and the disposable systemd service test.
2. The other-UID test result from a disposable container or a mocked UID seam.
3. The `go test -race`, `go vet`, coverage, and `scripts/check.sh --full` outputs.
4. The before/after help text and dry-run outputs.
5. Any deviation from this plan and why.
