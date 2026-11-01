# Goal: Compare git-describe dev builds as newer than their base tag

## Goal Description
Fix update checks so git-describe versions like v0.1.0-59-g2c13a5e are treated as dev snapshots after v0.1.0 rather than prereleases older than it, preventing false update notices and downgrade offers.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## References
- **Shared Understanding & Fact Sheet**: [`goals/updater-dev-version-compare/facts.md`](facts.md)
- **Execution Plan**: [`goals/updater-dev-version-compare/plan.md`](plan.md)

## Files to Touch
- `internal/updater/updater.go`
- `internal/updater/updater_test.go`

## Memory Plan
- **Prime**: run `sila prime` (CLI) or `prime_context` (MCP) at task start and record the briefing timestamp.
- **Recall**: run `sila recall "<symptom or design question>"` before implementation; record each query and match count.
- **Writes**: record at least one `memo`, `lesson`, or `case` grounded to a file in this goal; use a `case` when the goal introduces a reusable mechanism.
- **Facts**: run `sila facts verify` before certification and record the exit code and violation delta.
- **Handoff**: run `sila handoff "<summary>" --open="..."` at session end and record the handoff ID.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`, per `memory-protocol-goal-contract`.

## Done Condition
1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the `implemented` transition.
2. Implementation is committed with `(goals/updater-dev-version-compare/goal.md)` or `[goal: updater-dev-version-compare]` attribution under a `feat:`, `fix:`, `refactor:`, `perf:`, or `chore:` prefix.
3. Goal is certified (`goals/updater-dev-version-compare/certification.json` with `verified: true` or a signed `audit-receipt.json`) and any markdown evidence contains an explicit standalone `Verdict: PASS` declaration.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0` for delivered goals, with a clean `git status`.
