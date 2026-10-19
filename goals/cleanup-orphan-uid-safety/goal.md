# Goal: Harden Orphan Detection, Process UID Safety, and Destructive Cleanup Claims

## Chain of Custody (Isnad)

- **Registered By**: Business review agent, 2026-10-19
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: Pending Mystery Audit

## Goal Description

Fix the three highest-risk trust defects in swapaudit's destructive path:

1. `IsProcessOrphaned` currently treats any process whose parent is `systemd` or `init` as orphaned, so `--clean-orphaned` can kill a legitimate systemd-managed agent.
2. `KillProcessTargets` has no UID ownership check, so a root run can signal processes owned by other users.
3. The dormant-cleanup prompt claims "Conversations stay preserved on disk" without evidence, creating false reassurance before a `SIGTERM`.

The goal is not to add agent coverage or new cleanup categories. It is to make the existing destructive path fail closed, observable, and honest. All behavior changes must be documented and covered by tests that do not require killing a real user process.

## Business Context

- The business review (`docs/business-reviews/2026-10-19-swapaudit-full-review.md`) ranks these as **Critical** trust and data-loss gaps (G-03, G-04, G-11).
- A single false-positive kill can destroy a developer's in-progress agent session and permanently damage trust in a tool whose entire promise is "safe cleanup."
- The v0.1.0 release must not ship until these guards are in place. This goal is a prerequisite for `standalone-v0-1-0-release`.

## Dependencies & Execution Order

- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: 1
- **Shape**: ship
- **Tier**: immediate

## Files to Touch

- `internal/proc/proc.go`
- `internal/proc/proc_test.go`
- `internal/cleaner/cleaner.go`
- `internal/cleaner/cleaner_test.go`
- `internal/app/app.go`
- `internal/app/app_test.go`
- `internal/app/display.go`
- `internal/model/model.go` (only if an evidence/reason field is required)
- `README.md`
- `GOVERNANCE.md`
- `CHANGELOG.md`
- `docs/safety.md` (new)
- `scripts/check.sh` (claims sentry)
- `goals/cleanup-orphan-uid-safety/*` (this package)

## Requirements

### R1 — Parse and expose process ownership

Add to `internal/proc/proc.go`:

```go
// ParseProcStatusUID returns the real UID from the Uid: line in /proc/[pid]/status.
func ParseProcStatusUID(r io.Reader) (uint32, error)

// ReadProcUID returns the real UID of pid.
func ReadProcUID(pid int) (uint32, error)
```

- Parse the first numeric field of the `Uid:` line (real UID). Do not use the effective or saved UID for the ownership comparison.
- Return an error if the `Uid:` line is missing, has fewer than one field, or the value is not a valid `uint32`.
- Tests must cover: valid line, missing line, malformed line, multiple whitespace variants, and `ReadProcUID` against the current process and PID 1.

### R2 — Parse terminal attachment and cgroup evidence

Add to `internal/proc/proc.go`:

```go
// ReadTTYNr returns field 7 (tty_nr) from /proc/[pid]/stat, or 0 if unavailable.
func ReadTTYNr(pid int) int64

// ReadProcessCgroup returns the unified cgroup path from /proc/[pid]/cgroup, or "" if unavailable.
func ReadProcessCgroup(pid int) string
```

- Reuse the existing `)`-aware `/proc/[pid]/stat` parsing approach used by `ReadStartTicks`; do not split on spaces before the `comm` field.
- `ReadTTYNr` returns `0` on any error and does not panic.
- `ReadProcessCgroup` returns the `0::` unified hierarchy path when present; otherwise return the first non-empty path. Tests use fixture readers, not the live `/proc`, where possible.

### R3 — Replace the orphan heuristic with fail-closed evidence

Replace `IsProcessOrphaned(ppid int) bool` with:

```go
type OrphanEvidence struct {
    PPID       int
    ParentComm string
    Cgroup     string
    TTYNr      int64
    Reason     string
}

// ClassifyOrphan returns true only when the process is a plausible
// terminal-launched agent whose parent shell has died.
func ClassifyOrphan(pid, ppid int) (OrphanEvidence, bool)
```

Classification rules, in order:

1. If the parent process exists and its `comm` is **not** `systemd` or `init`, return `false` with `Reason: "parent-alive"`.
2. If the parent is missing, or its `comm` is `systemd`/`init`:
   - If the cgroup contains `.service` and does not contain `.scope`, return `false` with `Reason: "systemd-service"`.
   - If `TTYNr == 0`, return `false` with `Reason: "no-controlling-terminal"`.
   - If the cgroup contains `.scope` or `session-`, return `true` with `Reason: "reparented-terminal-scope"`.
   - Otherwise return `false` with `Reason: "ambiguous-evidence"`.
3. Any read error returns `false` with `Reason: "evidence-unavailable"`.

The rule is intentionally conservative: **ambiguous evidence must not authorize a kill.** The cgroup/tty heuristic is documented in `docs/safety.md`, including its limits on non-systemd init systems and containers.

### R4 — Enforce UID ownership before signaling

Add to `internal/cleaner/cleaner.go` a package-level seam for tests:

```go
var readProcessUID = proc.ReadProcUID
```

In `KillProcessTargets`, before the start-ticks check:

1. Read the target UID.
2. If the UID cannot be read, skip with reason `uid-unreadable`.
3. If `targetUID != uint32(os.Geteuid())` and `AllUsers` is false, skip with reason `uid-mismatch`.
4. If `AllUsers` is true and the target UID differs, print one warning per group:
   `WARNING: --all-users is set; signaling processes owned by other UIDs.`
5. If the process is root-owned and the current effective UID is not root, the signal will fail with `EPERM`; print the existing `failed to kill` error and continue. Do not silently ignore it.

The UID check must run in both live and `--dry-run` modes so the preview matches the real action.

### R5 — Add explicit cleanup options and preserve file-deletion behavior

Introduce a small options struct rather than adding a seventh boolean parameter:

```go
type CleanupFlags struct {
    Untouched       bool
    Abandoned       bool
    Dormant         bool
    Orphaned        bool
    IncludeOrphaned bool
}

type CleanupOptions struct {
    DryRun   bool
    Yes      bool
    AllUsers bool
    PathWidth int
    Prefix   string
}

func ExecuteCleanup(items []model.SwapItem, flags CleanupFlags, opts CleanupOptions)
func KillProcessTargets(targets []model.SwapItem, groupTitle, prompt string, opts CleanupOptions) int64
func PruneTmpTargets(targets []model.SwapItem, groupTitle, prompt string, opts CleanupOptions) int64
```

- File path confinement, symlink rejection, and UID checks in `PruneTmpTargets` remain unchanged.
- The existing tests are updated to the new signatures without weakening any assertion.
- No new external dependency.

### R6 — Change `--clean-all` so orphan cleanup is opt-in

- `--clean-all` sets `Untouched`, `Abandoned`, and `Dormant` only.
- `--clean-all --include-orphaned` also sets `Orphaned`.
- `--clean-orphaned` continues to work on its own.
- `--include-orphaned` without `--clean-all` or `--clean-orphaned` is a no-op with a one-line warning:
  `warning: --include-orphaned has no effect without --clean-all or --clean-orphaned`
- `PrintQuickCommands` and the help text must reflect the new semantics.
- Add a pure helper `ResolveCleanupFlags(cleanAll, cleanOrphaned, includeOrphaned bool) CleanupFlags` and table tests for all eight combinations.

### R7 — Replace the unverified conversation claim

- Replace the prompt at `internal/cleaner/cleaner.go:246` with:
  `End dormant agent processes? This sends SIGTERM; unsaved in-memory state may be lost. Verify the agent's persistence behavior first.`
- Add `docs/safety.md` covering:
  - What file cleanup does (`os.RemoveAll`).
  - What process cleanup does (`SIGTERM`, no `SIGKILL` fallback).
  - UID scope and `--all-users`.
  - The orphan heuristic and its limits.
  - Agent persistence: "swapaudit does not verify that any agent persists conversations to disk. Test your agent with a disposable session before using `--clean-dormant`."
  - Recommended workflow: `--dry-run` first, review every target, then run one category at a time.
- Add a claims sentry to `scripts/check.sh` that fails on banned unverified phrases. Start with:
  - `Conversations stay preserved`
  - `conversations stay preserved`
  - `100% safe`
  - `guaranteed safe`
  The sentry must exclude `docs/safety.md` if it quotes the banned phrase as a warning; use a narrow allowlist or phrase the doc without the exact banned string.

### R8 — Add a cleanup summary with skip reasons

`ExecuteCleanup` prints a final summary in both live and dry-run modes:

```text
Cleanup summary: 2 pruned, 1 ended, 3 skipped (1 uid-mismatch, 1 start-ticks-mismatch, 1 systemd-service)
```

- Count reasons: `uid-unreadable`, `uid-mismatch`, `start-ticks-mismatch`, `process-gone`, `systemd-service`, `no-controlling-terminal`, `ambiguous-evidence`, `parent-alive`, `path-rejected`, `symlink-rejected`, `not-owned`, `remove-failed`, `kill-failed`.
- Do not print a reason that did not occur.
- The summary is derived from a small `CleanupReport` struct; no global counters.
- Tests assert exact counts for a mixed fixture.

### R9 — Add help and documentation

- `--all-users`: "Allow signaling processes owned by other UIDs. Requires root for other users; default false."
- `--include-orphaned`: "Include orphaned-agent cleanup when using --clean-all. Orphan cleanup is never implicit."
- `--clean-orphaned`: "End orphaned agent processes only when cgroup/tty evidence indicates a reparented terminal session."
- `README.md` gets a "Process ownership and orphan safety" subsection.
- `GOVERNANCE.md` anti-goal #4 becomes: "Unsafe automated process killing: swapaudit never terminates system daemons, active user shells, systemd services, or processes owned by another UID unless `--all-users` is explicitly set."
- `CHANGELOG.md` records the behavior change under `Changed` and the safety fixes under `Fixed`.

### R10 — Preserve exit codes and failure behavior

- A skipped target is not a failure; the process exits `0` if the audit itself succeeded.
- Invalid flag combinations exit `2` with a clear stderr message.
- `SIGTERM` failure prints the PID, name, and error; the summary counts `kill-failed`.
- `--dry-run` never calls `syscall.Kill` or `os.RemoveAll`.

### R11 — Tests must not kill real processes

- Use `readProcessUID` overrides and non-existent PIDs.
- For live-mode tests, use `PID 999999` or a child process created and exited by the test; never signal PID 1, the test process, or another user's process.
- The systemd/orphan classification tests use fixture strings and pure helpers, not the live systemd state.
- A manual verification protocol in `docs/safety.md` covers the one test that cannot be automated: launching a disposable `systemd --user` service and confirming it is not marked cleanable.

## Non-Goals

- No new agent runtimes (Claude Code, Codex, Cursor, Windsurf, Cline, Aider, OpenCode, Gemini). That is a separate feature goal and is frozen until demand validation.
- No persistent audit log file, JSON output, or machine-readable cleanup report. The summary is human-readable only.
- No `SIGKILL` fallback or process-group killing.
- No change to `/tmp` cache classification or `/var/tmp` scanning.
- No remote push, release, or repository creation. This goal is local code and tests only.

## Acceptance Criteria

- **AC-01**: `go test -race ./...` passes.
- **AC-02**: `go vet ./...` passes.
- **AC-03**: `go test -cover ./internal/cleaner ./internal/proc` shows no coverage decrease; every new branch in R1–R8 has at least one test.
- **AC-04**: `grep -R "Conversations stay preserved"` returns no matches in the repository.
- **AC-05**: `./swapaudit --help` lists `--all-users` and `--include-orphaned` with the exact semantics above.
- **AC-06**: `./swapaudit --dry-run --clean-all --no-color --ascii` does not print an "Orphaned Agent Processes" group.
- **AC-07**: `./swapaudit --dry-run --clean-all --include-orphaned --no-color --ascii` may print the orphan group, but never signals a process.
- **AC-08**: A fixture with a `.service` cgroup and `TTYNr == 0` is classified `false` with `systemd-service`.
- **AC-09**: A fixture with a `.scope` cgroup, a live tty, and a `systemd`/`init` parent is classified `true` with `reparented-terminal-scope`.
- **AC-10**: A target with a UID different from `os.Geteuid()` is skipped as `uid-mismatch` when `AllUsers` is false.
- **AC-11**: The same target is allowed (dry-run) when `AllUsers` is true, and the warning is printed exactly once per group.
- **AC-12**: `scripts/check.sh --full` passes, including the new claims sentry.
- **AC-13**: `README.md`, `GOVERNANCE.md`, `CHANGELOG.md`, and `docs/safety.md` describe the new behavior consistently.
- **AC-14**: The manual systemd-service protocol in `docs/safety.md` is executed on this host (or a disposable container) and its output is pasted into the goal's completion evidence.

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

Add the exact outputs to `goals/cleanup-orphan-uid-safety/evidence.md` (new, local, not a product artifact) before certifying the goal.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| The cgroup/tty heuristic still misclassifies a terminal-attached systemd service | Fail closed on ambiguous evidence; require `--include-orphaned`; print the evidence reason; document the limit |
| `--clean-all` no longer kills orphaned agents, surprising existing users | CHANGELOG `Changed`, help text, and a one-line warning when `--include-orphaned` is absent |
| The UID check blocks container roots with user namespaces | Document `--all-users`; test as root in a disposable container; report the effective UID |
| The options-struct refactor breaks call sites | Compile-driven refactor; update all tests; no behavior change to file pruning |
| The claims sentry blocks legitimate documentation | Use a narrow banned-phrase list and an explicit allowlist for `docs/safety.md`; keep the list in `scripts/check.sh` |
| The summary adds noise to scripted output | The summary is one line at the end; `--dry-run` remains parseable enough for v0.1.0; JSON is a separate goal |

## Rollback

- Revert the single implementation commit. There is no data migration.
- If the new heuristic proves too conservative in the field, keep the UID check and the opt-in `--clean-all` change, and widen the orphan evidence rules in a follow-up goal with a new manual test.
- Do not reintroduce the "Conversations stay preserved" claim without an external, reproducible verification.

## References

- Business review: `docs/business-reviews/2026-10-19-swapaudit-full-review.md` (§5, §6.2, §7 G-03/G-04/G-11)
- Existing safety goal: `goals/cleaner-safety-guards/goal.md`
- `proc(5)`: `/proc/[pid]/status` `Uid:` field and `/proc/[pid]/stat` `tty_nr`
- `systemd` cgroup layout: user services use `.service`; terminal sessions use `.scope`
- Live evidence captured 2026-10-19:
  - `agy` PID 1271524: `Uid: 1000 1000 1000 1000`, `tty_nr=34822`, cgroup `.../app.slice/app-org.kde.konsole-6463.scope`
  - A systemd user service uses a `.service` cgroup and normally has no controlling terminal
