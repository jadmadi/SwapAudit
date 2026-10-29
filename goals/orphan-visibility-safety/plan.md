# Plan: Make Orphaned and Dormant Agent Visibility Complete and Fail-Closed

## Execution Steps

### Phase 1 — Baseline and contract freeze

- [x] Capture the current output of `./swapaudit --no-color --ascii`, `--explain-agents` (does not exist yet), and `--dry-run --clean-orphaned` in `goals/orphan-visibility-safety/evidence.md`.
- [x] Record the current agent count, swap sum, and `abandoned-agents` row (or its absence).
- [x] Record the current `ClassifyOrphan` reasons for every agent with `--conversation`/`agy`/`chrome-devtools` on this host.
- [x] Confirm which agents have `VmSwap == 0` with:
  `for p in $(pgrep -f 'agy|devin|chrome-devtools'); do awk '/^VmSwap:/{print FILENAME,$2}' /proc/$p/status; done`
- [x] Run `go test -race ./...` and `go vet ./...`; record the baseline.
- [x] Add the V1–V6 contract to `docs/safety.md` before implementation.

### Phase 2 — Process snapshot and parent info

- [x] Add `ProcessSnapshot`, `ScanProcesses`, `ReadParentInfo`, and `ParseVmRSS` to `internal/proc/proc.go`.
- [x] Keep `ReadProcStatus`, `ReadStartTicks`, `ClassifyOrphan`, and `ReadProcessLastActive` unchanged or refactor them behind the snapshot.
- [x] Record `IdleKnown=false` on `/proc/[pid]/sched` errors; never coerce to a duration.
- [x] Count `vanished`, `permission-denied`, and `zero-swap`.
- [x] Add `TestScanProcessesIncludesZeroSwap`, `TestScanProcessesIdleUnknown`, `TestReadParentInfo`.
- [x] Run `go test -race ./internal/proc`.

### Phase 3 — Agent classification with reasons

- [x] Extract `IsAgent(comm, argv)` from `CategorizeProcess`.
- [x] Add `ClassifyAgent(snapshot, idleThreshold) (Category, cleanable bool, reason string)`.
- [x] Add `AgentPopulation` and a pure `BuildAgentPopulation(snapshots, idleThreshold)` helper.
- [x] Update `CollectSwapItems` or replace it with `CollectAuditData` that returns both swap items and agent candidates.
- [x] Ensure `CategorizeProcess` remains a wrapper or is removed after all callers compile.
- [x] Add `TestIsAgent`, `TestClassifyAgentReasons`, `TestAgentPopulation`.
- [x] Run `go test -race ./internal/triage ./internal/app`.

### Phase 4 — Visibility and evidence display

- [x] Always render `abandoned-agents`, including `0`.
- [x] Add the `Agents:` population line and `Orphan scan:` status line.
- [x] Add `IdleKnown`, `OrphanReason`, `ParentPID`, `ParentComm`, `ParentAlive`, `TTYNr`, `Cgroup`, `Signal` to the item model.
- [x] Add `--explain-agents`; print a table with PID, comm, parent, idle, swap, tty, cgroup, reason, decision.
- [x] Add a warning when `PermissionDenied > 0`; do not mark those agents cleanable.
- [x] Update the empty state to include the status lines.
- [x] Add `TestOrphanZeroRow`, `TestZeroSwapOrphanVisible`, `TestExplainAgents`, `TestPermissionWarning`.
- [x] Run `go test -race ./internal/app`.

### Phase 5 — Kill-time rechecks and signal reporting

- [x] Add `ParentAlive`, `ParentComm`, `ParentPID`, `OrphanReason`, `IdleKnown`, `Signal` to the cleanup target.
- [x] Add test seams:
  ```go
  var readParentInfo = proc.ReadParentInfo
  var classifyOrphan = proc.ClassifyOrphan
  var readIdle = proc.ReadProcessLastActive
  var killProcess = syscall.Kill
  ```
- [x] In `KillProcessTargets`, implement V4 in order: PID floor, UID, parent, orphan reclassification, idle, start ticks, signal.
- [x] Add `--allow-live-parent` to `CleanupOptions`/flags.
- [x] Print `signal=SIGTERM` in dry-run and live output.
- [x] Add skip reasons: `parent-alive`, `orphan-evidence-changed`, `idle-unknown`.
- [x] Add `TestParentAliveSkip`, `TestAllowLiveParent`, `TestOrphanEvidenceChanged`, `TestIdleUnknownSkip`, `TestSignalPrinted`, `TestNoSIGKILL`.
- [x] Run `go test -race ./internal/cleaner ./internal/app`.

### Phase 6 — Non-interactive behavior and warnings

- [x] Detect stdin TTY with `os.Stdin.Stat()` / `ModeCharDevice`.
- [x] If cleanup flags are set and neither `--dry-run` nor `--yes` is set and stdin is not a TTY, print the exact error and return `2` before any prompt.
- [x] Add warnings for `--dry-run`/`--yes` without cleanup flags.
- [x] Keep the per-group prompt for TTY input.
- [x] Add `TestNonTTYRequiresDryRunOrYes`, `TestDryRunNoCleanupWarning`, `TestYesNoCleanupWarning`, `TestTTYPrompt` (with a seam).
- [x] Run `go test -race ./internal/app ./internal/cleaner`.

### Phase 7 — Documentation and full verification

- [x] Update `docs/safety.md` with signal semantics, parent check, zero-swap visibility, idle-unknown, and non-TTY behavior.
- [x] Update `README.md` agent table and flags.
- [x] Update `CHANGELOG.md`.
- [x] Run `gofmt -l .`, `go vet ./...`, `go test -race ./...`.
- [x] Run the targeted tests.
- [x] Run the live read-only checks:
  - `./swapaudit --no-color --ascii`
  - `./swapaudit --no-color --ascii --explain-agents`
  - `./swapaudit --no-color --ascii --dry-run --clean-orphaned`
  - `printf '' | ./swapaudit --no-color --ascii --clean-all; echo $?` (expect 2)
- [x] Run `./scripts/check.sh --full`.
- [x] Record outputs in `evidence.md`.

### Phase 8 — Attribution commit

- [x] Commit with `feat:` or `fix:` and a subject containing `(goals/orphan-visibility-safety/goal.md)`.
- [x] Run `sila goals --project=/home/jad/scripts/active/swapaudit`.
- [x] Do not push to a remote unless the user explicitly authorizes it.

## Tests to Add

| Test | File | What it proves |
| --- | --- | --- |
| `TestScanProcessesIncludesZeroSwap` | `internal/proc/proc_test.go` | Zero-swap processes are not dropped |
| `TestScanProcessesIdleUnknown` | same | `IdleKnown=false` |
| `TestReadParentInfo` | same | Parent PID/comm/alive |
| `TestIsAgent` | `internal/triage/triage_test.go` | Detection list and argv |
| `TestClassifyAgentReasons` | same | Every category/reason |
| `TestAgentPopulation` | same | Counts tie to items |
| `TestZeroSwapOrphanVisible` | `internal/app/app_test.go` | Population + actionable |
| `TestOrphanZeroRow` | `internal/app/display_test.go` | Zero row always rendered |
| `TestExplainAgents` | same | Evidence table |
| `TestPermissionWarning` | same | Incomplete evidence warning |
| `TestParentAliveSkip` | `internal/cleaner/cleaner_test.go` | Default skip |
| `TestAllowLiveParent` | same | Explicit override + warning |
| `TestOrphanEvidenceChanged` | same | Reclassification at kill time |
| `TestIdleUnknownSkip` | same | Fail closed |
| `TestSignalPrinted` | same | `SIGTERM` in both modes |
| `TestNoSIGKILL` | same | No SIGKILL call |
| `TestNonTTYRequiresDryRunOrYes` | `internal/app/app_test.go` | Exit 2 + exact message |
| `TestDryRunNoCleanupWarning` | same | Warning, no prompt |
| `TestYesNoCleanupWarning` | same | Warning, no prompt |

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -race -run 'Test(ScanProcesses|IsAgent|ClassifyAgent|ZeroSwap|OrphanZeroRow|ParentAlive|OrphanEvidenceChanged|IdleUnknown|Signal|NonTTY|ExplainAgents)' ./internal/proc ./internal/triage ./internal/cleaner ./internal/app
./swapaudit --no-color --ascii | sed -n '/Category Breakdown/,$p'
./swapaudit --no-color --ascii --explain-agents | sed -n '1,50p'
./swapaudit --no-color --ascii --dry-run --clean-orphaned
./swapaudit --no-color --ascii --dry-run --clean-dormant
printf '' | ./swapaudit --no-color --ascii --clean-all; echo "exit=$?"   # expect 2
grep -R "SIGKILL" internal || true
./scripts/check.sh --full
```

## Rollback

- Revert the implementation commit. No persistent state.
- If the full process scan is too expensive, keep zero-swap visibility behind `--explain-agents` and keep cleanup swap-only; do not remove the zero-row/evidence display.
- Do not reintroduce silent non-TTY cleanup or remove the parent/evidence rechecks.

## Completion Evidence

`goals/orphan-visibility-safety/evidence.md` must contain:

1. Before/after agent population and orphan status lines.
2. The zero-swap agent inventory from this host.
3. The `ClassifyOrphan` reasons for each agent.
4. Dry-run output showing `SIGTERM` and the skip reasons.
5. The non-TTY exit-2 output and the `--yes`/`--dry-run` behavior.
6. `go test -race`, `go vet`, and `scripts/check.sh --full` outputs.
7. Any deviation from this plan and why.
