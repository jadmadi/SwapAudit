# Goal: Make Orphaned and Dormant Agent Visibility Complete and Fail-Closed

## Chain of Custody (Isnad)

- **Registered By**: UI review follow-up, 2026-10-19
- **Spec Audited By**: CommandCode, 2026-10-20 (acceptance criteria checked against tests and evidence)
- **Implemented By**: CommandCode, 2026-10-20 (see `evidence.md`)
- **Mystery Audited By**: Evidence certification, 2026-10-20 (see `certification.json`; independent mystery audit not run)

## Goal Description

The current tree already contains the evidence-gated orphan classifier and the UID/start-ticks safety work, so the review's claim that `--clean-all` implicitly kills orphans and that process kills lack UID checks is **outdated**. Two real defects remain:

1. **Invisible agents.** `CollectSwapItems` drops every process with `VmSwap == 0` (`internal/app/app.go:63-65`). An orphaned or dormant agent that has not been swapped out is therefore absent from the audit, the actionable table, and the cleanup path. `--clean-orphaned` and `--clean-dormant` can report success while a zero-swap orphan remains alive.
2. **Implicit zero categories.** `PrintCategorySummary` renders `abandoned-agents` only when `counts[c] > 0` (`internal/app/display.go:368-369`). A zero orphan count is indistinguishable from a broken detector. There is no evidence line, no parent-liveness column, and no signal line in cleanup output.

This goal makes agent visibility independent of swap, always shows the orphan status and evidence, and makes termination fail closed by re-checking parent liveness, orphan evidence, idle evidence, UID, and start ticks immediately before `SIGTERM`. It also defines non-interactive confirmation behavior and prints the signal used.

## Business Context

- The review's priority says: "fix #5 first ... then add the safety checks before anyone relies on `--clean-all`."
- The current code is safer than the screenshot suggests, but a zero-swap orphan is still a blind spot. A user running `--clean-orphaned` cannot know whether the tool found nothing or never looked.
- The reviewed host has 5 dormant agents and 0 orphaned agents. "0 orphaned" is a valid, useful result only if the evidence and population are displayed.
- A false-positive `SIGTERM` is irreversible for an in-memory agent session. Parent liveness and idle evidence must be re-checked at kill time, not trusted from the scan snapshot.
- This goal must land before the v0.1.0 release; it is the visibility half of the safety story.

## Verified Findings (2026-10-19, current tree)

### F1 — Swap filter hides zero-swap agents

- `internal/app/app.go:63-65`:
  ```go
  swapBytes, ppid := proc.ReadProcStatus(pid)
  if swapBytes <= 0 {
      continue
  }
  ```
- `internal/app/app.go:75-77` classifies only the survivors. `CollectSwapItems` returns a single `[]model.SwapItem` used by both display and cleanup (`internal/app/app.go:212-234`).
- `internal/cleaner/cleaner.go:332-356` builds orphaned/dormant targets from that same slice.
- Consequence: an agent with `VmSwap == 0` is never classified, never displayed, and never killed. The tool cannot distinguish "no orphaned agents" from "orphaned agents with no swap".

### F2 — Zero categories are omitted

- `internal/app/display.go:356-388` checks `counts[c] > 0` before rendering each process category.
- `abandoned-agents` is in `swapCats` (`internal/app/display.go:349-354`) but is omitted when zero.
- The empty-state text is only `nothing to reclaim` (`internal/app/display.go:67-69`); it does not say "0 orphaned".
- Quick commands mention `--clean-orphaned` (`internal/app/display.go:451`) without a status line.

### F3 — Orphan evidence is not displayed

- `internal/proc/proc.go:207-253` defines `OrphanEvidence{PPID, ParentComm, Cgroup, TTYNr, Reason}` and `ClassifyOrphan`.
- `internal/app/app.go:75` discards the evidence: `_, isOrphaned := proc.ClassifyOrphan(pid, ppid)`.
- `model.SwapItem` has no `OrphanReason`, `ParentAlive`, `ParentComm`, `TTYNr`, or `Cgroup` field.
- `internal/app/display.go` has no evidence column or `--explain` output.
- Consequence: a user cannot see why a process was or was not considered orphaned.

### F4 — Parent liveness is not re-checked at kill time

- `internal/cleaner/cleaner.go:228-269` re-reads UID and start ticks, then calls `syscall.Kill(p.PID, syscall.SIGTERM)`.
- It does not re-read PPID, parent comm, cgroup, tty, or idle time.
- A process classified as dormant at scan time can be actively running at kill time; a process classified as orphaned can have a live parent if the PID was reused or the parent state changed.
- `ClassifyOrphan` is not called again.

### F5 — Idle evidence can be unknown

- `internal/proc/proc.go:331-356` returns `-1` when `/proc/[pid]/sched` is unreadable.
- `internal/triage/triage.go:47-50` treats `lastActive >= 0 && lastActive > idleThreshold` as dormant; `-1` is silently "not dormant".
- No count or warning exposes `idle-unknown` processes.
- Consequence: on a hardened host, dormant cleanup silently does nothing, and the user cannot tell why.

### F6 — Signal is not printed

- `internal/cleaner/cleaner.go:264-268` sends `SIGTERM`.
- The output line (`:271-280`) prints `ended`/`would end` and the byte estimate, but not the signal.
- Dry-run does not say what it would send.
- There is no `SIGKILL` fallback (good), but the operator cannot see that from the output.

### F7 — Non-interactive confirmation behavior is undefined

- `internal/cleaner/cleaner.go:123-131` and `:216-224` call `AskConfirmation` unless `--yes` or `--dry-run`.
- `AskConfirmationReader` (`:81-89`) returns `false` on EOF.
- In a non-TTY pipeline without `--yes`, cleanup silently skips every group and exits 0. A script cannot distinguish a decline from a failure or a success.
- The review's "make `--clean-all` default to dry-run or require confirmation" is partially satisfied by prompts, but the non-TTY behavior is not explicit.

### F8 — What is already fixed (do not re-fix)

- `--clean-all` does not include orphaned cleanup; `ResolveCleanupFlags` is at `internal/app/app.go:119-137`, and `--include-orphaned` is required (`:133-135`).
- Process UID ownership is checked in both dry-run and live mode (`internal/cleaner/cleaner.go:234-250`).
- Start ticks are re-read before `SIGTERM` (`:252-268`).
- The "Conversations stay preserved" claim is gone; the dormant prompt warns that unsaved state may be lost (`:353`).
- The review's claim #7 ("orphaned promised but never shown") is therefore **partially outdated**: the category exists and is evidence-gated, but zero counts and zero-swap agents are still invisible.

## Dependencies & Execution Order

- **Mode**: Dependent (uses cache liveness contracts and the report model from Goals 1–2)
- **Depends On**: `cache-liveness-classification`
- **Sequence**: 3
- **Shape**: ship
- **Tier**: immediate
- **Blocks**: `output-information-architecture`

## Files to Touch

- `internal/triage/triage.go` — extract `IsAgent`, add `AgentEvidence`, keep shell aggregation
- `internal/triage/triage_test.go`
- `internal/proc/proc.go` — `ReadParentInfo`, `ReadIdleEvidence`, `ReadRSS` (optional), `ProcessSnapshot`
- `internal/proc/proc_test.go`
- `internal/app/app.go` — full process scan, agent candidates, flags, non-TTY guard
- `internal/app/app_test.go`
- `internal/app/display.go` — zero rows, status/evidence line, `--explain-agents`, signal text
- `internal/app/display_test.go`
- `internal/cleaner/cleaner.go` — parent/evidence rechecks, signal reporting, skip reasons, non-TTY behavior
- `internal/cleaner/cleaner_test.go`
- `internal/model/model.go` — `OrphanReason`, `ParentAlive`, `ParentPID`, `ParentComm`, `IdleKnown`, `Signal`
- `internal/model/model_test.go`
- `README.md`, `CHANGELOG.md`, `docs/safety.md`
- `goals/orphan-visibility-safety/*` (this package)

## Agent Visibility Contract (over-specified)

### V1 — Separate scan from swap filtering

```go
// internal/proc/proc.go
type ProcessSnapshot struct {
    PID            int
    PPID           int
    StartTicks     int64
    UID            uint32
    Comm           string
    Argv           []string
    VmSwap         int64
    RSS            int64
    Age            time.Duration
    Idle           time.Duration
    IdleKnown      bool
    Orphan         OrphanEvidence
    Orphaned       bool
    ParentAlive    bool
    ParentComm     string
}

func ScanProcesses(now time.Time, uptimeSec float64) []ProcessSnapshot
```

- `ScanProcesses` reads every numeric `/proc/[pid]` it can access. It skips the auditor's own PID.
- It never filters by `VmSwap`. It records `VmSwap == 0` and increments `zero-swap`.
- It records `IdleKnown=false` when `/proc/[pid]/sched` is unreadable; do not coerce `-1` to "not idle".
- It records `Orphan`/`Orphaned` from `ClassifyOrphan`.
- It records `RSS` from `/proc/[pid]/status:VmRSS` or `statm` for display only; this goal does not use RSS for ranking.
- Errors are counted by reason; a vanished process is `vanished`, not a fatal error.

```go
// internal/triage/triage.go
func IsAgent(comm string, argv []string) bool
func ClassifyAgent(p proc.ProcessSnapshot, idleThreshold time.Duration) (model.Category, bool, string)
```

- `IsAgent` preserves the current detection set and argument heuristics (`agy`, `devin`, `chrome-devtools-mcp`, `antigravity`, `--conversation`).
- `ClassifyAgent` returns `(category, cleanable, reason)`:
  - `Orphaned && IsAgent` → `CatAbandonedAgent`, cleanable, reason from `OrphanEvidence.Reason`.
  - `IsAgent && IdleKnown && Idle > idleThreshold` → `CatDormantAgent`, cleanable, reason `idle-threshold`.
  - `IsAgent && !IdleKnown` → `CatActiveProcess`, not cleanable, reason `idle-unknown`.
  - `IsAgent` otherwise → `CatActiveProcess`, not cleanable, reason `active`.
  - Non-agent → `CatActiveProcess`, not cleanable, reason `not-agent`.
- `CategorizeProcess` can remain as a wrapper for compatibility; it must not lose the reason.

### V2 — Populations

Add to `report.Population` (Goal 1) or a new `AgentPopulation`:

```go
type AgentPopulation struct {
    TotalProcesses     int
    AccessibleProcesses int
    Agents             int
    AgentsWithSwap     int
    AgentsZeroSwap     int
    DormantAgents      int
    OrphanedAgents     int
    ParentAliveAgents  int
    IdleUnknownAgents  int
    ActiveAgents       int
    Vanished           int
    PermissionDenied   int
}
```

- The audit must print one line:
  `Agents: 12 detected · 7 with swap · 5 zero-swap · 0 orphaned · 5 dormant · 2 idle-unknown · 0 vanished`.
- The line is printed even when all counts are zero.
- The summary includes `abandoned-agents` with `0` when zero; never omit it.

### V3 — Evidence display

Add to `model.SwapItem` (or a parallel `AgentItem`):

```go
type SwapItem struct {
    // existing fields...
    IdleKnown    bool
    OrphanReason string
    ParentPID    int
    ParentComm   string
    ParentAlive  bool
    TTYNr        int64
    Cgroup       string
    Signal       string // "SIGTERM" for cleanable processes
}
```

- `--explain-agents` prints a table (respecting `--limit`/`--all`):
  `PID | COMM | PPID/PARENT | IDLE | SWAP | TTY | CGROUP | REASON | DECISION`.
- The default summary adds an `orphan-status` line:
  `Orphan scan: 12 agents · 0 orphaned (evidence-gated) · 9 parent-alive · 2 idle-unknown · 1 active`.
- If `PermissionDenied > 0`, print `WARN: N processes could not be inspected; idle/orphan evidence is incomplete`. Do not mark them cleanable.
- The empty state becomes:
  `nothing to reclaim` plus the agent/population lines. It must not be a bare checkmark.

### V4 — Kill-time rechecks

`KillProcessTargets` must perform, in order, for every target:

1. `PID > 1` and `PID != os.Getpid()`; else `pid-floor`.
2. Read UID; if unreadable → `uid-unreadable`; if other UID and `!AllUsers` → `uid-mismatch`; warn once if `AllUsers`.
3. Read parent info; if parent is alive and not `systemd`/`init`:
   - For `CatDormantAgent`: `parent-alive`; skip unless `--allow-live-parent` is set. If allowed, print `WARN: live parent <comm> (PID <ppid>)` and require confirmation/`--yes`.
   - For `CatAbandonedAgent`: `parent-alive`; always skip, regardless of `--allow-live-parent`, because the orphan evidence is no longer valid.
4. For `CatAbandonedAgent`, call `proc.ClassifyOrphan` again; if `!orphaned` → `orphan-evidence-changed`; skip.
5. Re-read idle; if `IdleKnown == false` → `idle-unknown`; skip dormant targets. (An orphaned target does not require idle evidence.)
6. Re-read start ticks; if 0 → `process-gone`; if different → `start-ticks-mismatch`.
7. Send `SIGTERM`; print `signal=SIGTERM` in both dry-run and live output.

- The dry-run must run steps 1–6 (including parent/evidence checks) and print the same skip reasons, but skip step 7.
- The `--allow-live-parent` flag is explicit and default false. It is never implied by `--clean-all`, `--yes`, or `--include-orphaned`.
- No `SIGKILL`, no process-group kill, no retry. Document this in `docs/safety.md`.

### V5 — Non-interactive confirmation

- If any cleanup flag is set, neither `--dry-run` nor `--yes` is set, and stdin is not a terminal:
  - Print `error: cleanup requires --dry-run or --yes when stdin is not a terminal` to stderr.
  - Exit `2` before scanning/cleanup groups (or after the audit but before any mutation). Prefer before any prompt.
- If stdin is a terminal, keep the current per-group prompt.
- `--dry-run` without a cleanup flag prints `warning: --dry-run has no effect without a cleanup flag` and does not prompt.
- `--yes` without a cleanup flag prints `warning: --yes has no effect without a cleanup flag`.
- A declined group counts `user-declined` and exits `0` (the audit succeeded; the user chose not to act).
- A skipped/failed target exits `0` unless the goal's exit-code contract says otherwise; this goal keeps `0` for operational skips and `2` for usage errors.

### V6 — Signal and result reporting

- Add `Signal` to the target and print:
  `[dry-run] OK would-signal SIGTERM PID 1430533: agy 582.3 MB`
  `OK signaled SIGTERM PID 1430533: agy 582.3 MB`
- The cleanup summary remains: `5 would end, 0 skipped`; add `signals: SIGTERM` only if a process group ran.
- `KillProcessTargets` returns a `[]KillResult` or uses `CleanupStats` with `Ended` and reasons. Do not add a global counter.

## Requirements

### R1 — Full process scan

- Implement `ProcessSnapshot`, `ScanProcesses`, and `ReadParentInfo`.
- Preserve `ReadProcStatus`, `ReadStartTicks`, `ClassifyOrphan`, and `ReadProcessLastActive` behavior.
- Add `ParseVmRSS` or equivalent; keep it optional and display-only.
- Tests with fake `/proc` data where possible; live `/proc` tests only for the current process and PID 1.

### R2 — Agent classification with reasons

- Implement `IsAgent`, `ClassifyAgent`, and `AgentPopulation`.
- Update `CollectSwapItems` to return both the swap items and agent candidates, or add `CollectAuditData` returning a struct.
- The display and cleanup must use the agent candidate list, not the swap-only list.
- Keep `CategorizeProcess` as a wrapper or remove it after updating all callers; no behavior regression.

### R3 — Zero rows and evidence

- Always render `abandoned-agents`, even at zero.
- Add the agent population line and orphan status line.
- Add `--explain-agents`; default table remains compact.
- Add `idle-unreadable`/`permission-denied` warnings.
- Update the empty state.

### R4 — Kill-time rechecks and signal

- Implement V4 in `KillProcessTargets`.
- Add `readParentInfo`, `classifyOrphan`, `readIdle`, and `killProcess` seams for tests.
- Add skip reasons: `parent-alive`, `orphan-evidence-changed`, `idle-unknown`.
- Add `Signal: "SIGTERM"` and print it.
- Ensure dry-run performs the same checks without signaling.

### R5 — Non-interactive behavior and flags

- Implement V5.
- Add `--allow-live-parent` (default false).
- Add warnings for `--dry-run`/`--yes` without a cleanup flag.
- Update `PrintUsage`, `PrintQuickCommands`, README, and `docs/safety.md`.
- Add tests that use a non-TTY stdin (e.g., a `bytes.Buffer`) and a TTY-like seam if needed.

### R6 — Tests must not kill real processes

- Use `PID 999999` or a child process the test creates and reaps.
- Never signal PID 1, the test process, or another user's process.
- For live-mode tests, use the `killProcess` seam to record the signal instead of sending it, unless the test is explicitly isolated.
- Fake `/proc` fixtures for `ScanProcesses`; live `/proc` only for read-only assertions.

### R7 — Documentation

- `docs/safety.md`: signal semantics, parent check, zero-swap visibility, idle-unknown, non-TTY behavior.
- `README.md`: agent detection list and flags; no claim that zero-swap agents are cleanable by default (they are visible; `--clean-orphaned` can target them).
- `CHANGELOG.md`: `Added` zero-swap visibility, `Changed` non-TTY behavior, `Fixed` zero-row omission.

## Non-Goals

- No new agent runtimes (Claude Code, Codex, Cursor, Windsurf, Cline, Aider, OpenCode, Gemini). The current list is unchanged; a future goal can extend it.
- No `SIGKILL`, process groups, or cgroups.
- No persistent audit log or JSON (Goal 5).
- No change to `--clean-all` including orphaned agents; it remains opt-in.
- No PSS/RSS-based ranking; RSS is display-only.
- No change to cache liveness (Goal 2) or table grouping (Goal 4).

## Acceptance Criteria

- **AC-01**: `go test -race ./...` and `go vet ./...` pass.
- **AC-02**: A fixture with an orphaned agent and `VmSwap == 0` appears in the agent population, the orphan status line, and the actionable table (0 B swapped); `--clean-orphaned --dry-run` lists it.
- **AC-03**: `abandoned-agents` is rendered with `0` on a host with no orphans; the output includes the orphan status line.
- **AC-04**: `--explain-agents` prints `Parent`, `Idle`, `TTY`, `Cgroup`, `Reason`, and `Decision` for every agent.
- **AC-05**: A dormant target whose parent is alive is skipped as `parent-alive` by default; `--allow-live-parent` permits it with a warning and still re-checks start ticks/UID.
- **AC-06**: An orphaned target whose `ClassifyOrphan` result changes at kill time is skipped as `orphan-evidence-changed`.
- **AC-07**: A target whose idle evidence is unreadable at kill time is skipped as `idle-unknown`.
- **AC-08**: Dry-run and live output both print `SIGTERM`; `grep -R "SIGKILL" internal` returns no matches.
- **AC-09**: Non-TTY cleanup without `--dry-run`/`--yes` exits 2 with the exact error; with `--dry-run` it exits 0 and mutates nothing; with `--yes` it proceeds.
- **AC-10**: `--dry-run`/`--yes` without a cleanup flag print a warning and do not prompt.
- **AC-11**: Zero-swap visibility does not change the swap headline or the `VmSwap` sum; the population line reports `zero-swap`.
- **AC-12**: `scripts/check.sh --full` passes; `go.mod` unchanged.

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -race -run 'Test(ScanProcesses|IsAgent|ClassifyAgent|ZeroSwap|OrphanZeroRow|ParentAlive|OrphanEvidenceChanged|IdleUnknown|Signal|NonTTY|ExplainAgents)' ./internal/proc ./internal/triage ./internal/cleaner ./internal/app
./swapaudit --no-color --ascii | sed -n '/Category Breakdown/,$p'
./swapaudit --no-color --ascii --explain-agents | sed -n '1,40p'
./swapaudit --no-color --ascii --dry-run --clean-orphaned
./swapaudit --no-color --ascii --dry-run --clean-dormant
printf '' | ./swapaudit --no-color --ascii --clean-all; echo "exit=$?"   # expect 2
grep -R "SIGKILL" internal || true
./scripts/check.sh --full
```

## Test Matrix

| Test | File | Proves |
| --- | --- | --- |
| `TestScanProcessesIncludesZeroSwap` | `internal/proc/proc_test.go` | Zero-swap process is neither dropped nor misclassified |
| `TestScanProcessesIdleUnknown` | same | `IdleKnown=false` and counts |
| `TestReadParentInfo` | same | PPID/parent comm/alive |
| `TestIsAgent` | `internal/triage/triage_test.go` | Detection list and argv heuristics |
| `TestClassifyAgentReasons` | same | Orphan/dormant/active/idle-unknown/not-agent |
| `TestAgentPopulation` | same | Counts tie to the item list |
| `TestZeroSwapOrphanVisible` | `internal/app/app_test.go` | Zero-swap orphan in population/actionable |
| `TestOrphanZeroRow` | `internal/app/display_test.go` | `abandoned-agents | 0` always rendered |
| `TestExplainAgents` | same | Evidence table |
| `TestParentAliveSkip` | `internal/cleaner/cleaner_test.go` | `parent-alive` reason |
| `TestAllowLiveParent` | same | Explicit override + warning |
| `TestOrphanEvidenceChanged` | same | Re-classification at kill time |
| `TestIdleUnknownSkip` | same | `idle-unknown` reason |
| `TestSignalPrinted` | same | `SIGTERM` in dry-run and live |
| `TestNonTTYRequiresDryRunOrYes` | `internal/app/app_test.go` | Exit 2 and exact message |
| `TestDryRunNoCleanupWarning` | same | Warning, no prompt |
| `TestYesNoCleanupWarning` | same | Warning, no prompt |
| `TestNoSIGKILL` | `internal/cleaner/cleaner_test.go` | No SIGKILL constant/call |

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Zero-swap agents could be killed even though they free no swap | They are visible; cleanup prints `0 B`; `--clean-orphaned`/`--clean-dormant` remain opt-in; document that the action is about process cleanup, not swap reclaim |
| Parent-alive check may block a legitimate dormant cleanup | `--allow-live-parent` is explicit; default fail closed; print the parent PID/comm |
| Re-reading `/proc` can hit a vanished process | Treat as `process-gone`; skip, do not fail the audit |
| Idle-unknown on hardened hosts makes dormant cleanup do nothing | Surface the count and warning; do not claim a clean result |
| Non-TTY exit 2 breaks scripts that relied on silent skip | Document the breaking change in CHANGELOG; scripts can add `--dry-run` or `--yes` |
| Evidence display adds noise | `--explain-agents` is opt-in; the default line is one row |
| Zero-swap agents inflate the actionable table with 0 B rows | Cap/group in Goal 4; label them `0 B swapped`; keep them visible |
| Killing a process with a live parent can break the parent | Default skip; explicit override; warning; no SIGKILL |

## Rollback

- Revert the implementation commit. No persistent state.
- If the full process scan is too expensive, keep the zero-swap orphan *visibility* behind `--explain-agents` and keep cleanup swap-only; do not remove the zero-row/evidence display.
- Do not reintroduce silent non-TTY cleanup or remove the parent/evidence rechecks.

## References

- UI review screenshot and claim list, 2026-10-19.
- Verification report: `docs/reviews/2026-10-19-swapaudit-ui-review.md`.
- `internal/app/app.go:63-77`, `:212-234` — swap-only collection and cleanup wiring.
- `internal/app/display.go:67-69`, `:349-388`, `:451` — empty state, zero-row omission, quick command.
- `internal/proc/proc.go:207-253`, `:331-356` — orphan evidence and idle.
- `internal/triage/triage.go:31-51` — agent detection and dormant classification.
- `internal/cleaner/cleaner.go:81-89`, `:123-131`, `:216-269`, `:332-356` — confirmation, UID, start ticks, SIGTERM, groups.
- `goals/cleanup-orphan-uid-safety/goal.md` — the safety work already implemented; do not undo it.
