# Goal: Clean Swap and RAM Without Reboot via Guarded Swap Drain and Cache Reclaim

## Chain of Custody (Isnad)
- **Registered By**: Jad Madi, 2026-11-02
- **Spec Audited By**: Jad Madi plan review, 2026-11-02
- **Implemented By**: Jad Madi, commit `a4c685c`, 2026-11-02
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_no-reboot-memory-reclaim_1793630336`
## Goal Description
Add a root-gated reclaim command that drains active swap devices back into RAM and optionally drops clean page cache, with feasibility math, per-device restore guarantees, zram awareness, and live receipts.

## Audit Findings
- No `swapoff`, `swapon`, `drop_caches`, `memory.reclaim`, or swap-device
  parser existed at HEAD. The tool could free a process's swapped pages only by
  killing it; it could not force pages held by wanted processes back into RAM.
- The report showed aggregate swap only. It had no per-device used bytes, no
  page-cache reclaim estimate, and no PSI pressure.
- `docs/safety.md` promised UID checks, non-TTY exit 2, and fail-closed orphan
  evidence that were absent from the code after the fab5560 merge.
- Live false positive: `ollama serve` (UID 999, PPID 1,
  `/system.slice/ollama.service`, tty 0) was classified as an abandoned agent
  and was killable with `--clean-orphaned`.
- `--clean-all` implicitly enabled orphaned kills, contradicting the safety
  contract that orphan cleanup is never implicit.
- `/tmp` reclaim accounting used apparent size, not `Stat_t.Blocks * 512`.

## Deliverables
- `internal/reclaim`: pure planner, feasibility budget, partial-drain subset
  selection, guarded executor, restore path, and JSON receipt.
- `internal/proc`: `/proc/swaps`, `/proc/pressure/memory`, real UID, TTY,
  cgroup, and fail-closed orphan classifier.
- `swapaudit reclaim` with `--dry-run`, `--yes`, `--json`, `--include-zram`,
  `--reserve`, and `--timeout`.
- Reclaim readiness block in the main report.
- Real-UID process guard, fail-closed orphan evidence, narrower agent
  detection, explicit-only orphan cleanup, non-TTY preflight, and allocated
  block accounting.
- Tests for planning, partial selection, restore, failure, purity, UID, orphan,
  and non-TTY paths.
- `docs/reclaim.md`, updated `docs/safety.md`, README, metrics, JSON schema,
  comparison index, VPS guide, and changelog.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: roadmap
- **Memory Protocol**: required

## References
- **Shared Understanding & Fact Sheet**: [`goals/no-reboot-memory-reclaim/facts.md`](facts.md)
- **Execution Plan**: [`goals/no-reboot-memory-reclaim/plan.md`](plan.md)

## Memory Plan
- **Prime**: run `sila prime` (CLI) or `prime_context` (MCP) at task start and record the briefing timestamp.
- **Recall**: run `sila recall "<symptom or design question>"` before implementation; record each query and match count.
- **Writes**: record at least one `memo`, `lesson`, or `case` grounded to a file in this goal; use a `case` when the goal introduces a reusable mechanism.
- **Facts**: run `sila facts verify` before certification and record the exit code and violation delta.
- **Handoff**: run `sila handoff "<summary>" --open="..."` at session end and record the handoff ID.
- **Evidence**: embed a `sila/memory-writes/v1` block under `## Memory Writes`, per `memory-protocol-goal-contract`.

## Done Condition
1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the `implemented` transition.
2. Implementation is committed with `(goals/no-reboot-memory-reclaim/goal.md)` or `[goal: no-reboot-memory-reclaim]` attribution under a `feat:`, `fix:`, `refactor:`, `perf:`, or `chore:` prefix.
3. Goal is certified (`goals/no-reboot-memory-reclaim/certification.json` with `verified: true` or a signed `audit-receipt.json`) and any markdown evidence contains an explicit standalone `Verdict: PASS` declaration.
4. `sila goals --json` reports `status: "implemented"`, `verification_status: "verified"`, and `unverified_count: 0` for delivered goals, with a clean `git status`.
