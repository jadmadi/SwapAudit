# Goal: Expand Proc Stat Parsing and Malformed Input Tests

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_proc-stat-edge-case-tests_1792407145`


## Goal Description
Add unit tests for ReadStartTicks and ReadProcessLastActive covering malformed proc stat files, missing closing parentheses, fewer than 20 fields, and absent se.exec_start lines in proc_test.go.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: nice-to-have

## Files to Touch
- internal/proc/proc_test.go

## References
- **Shared Understanding & Fact Sheet**: [`goals/proc-stat-edge-case-tests/facts.md`](facts.md)
- **Execution Plan**: [`goals/proc-stat-edge-case-tests/plan.md`](plan.md)

## Done Condition
1. All requirements described in the goal and execution plan are implemented.
2. All automated unit and integration tests pass cleanly with `-race` (`go test -race ./...` or stack equivalent).
3. Zero architectural regressions; definitions of done satisfied.
