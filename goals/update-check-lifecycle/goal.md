# Goal: Harden Background Update Check Lifecycle and Goroutine Hygiene

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_update-check-lifecycle_1792402797`


## Goal Description
Refactor release update checks so cached state is read synchronously from disk on startup, remote checks run with bounded lifecycle and cancellation context, avoiding orphaned goroutines or truncated state file writes when Run() exits.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: immediate

## Files to Touch
- internal/app/app.go
- internal/app/app_test.go
- internal/updater/updater.go
- internal/updater/updater_test.go

## References
- **Shared Understanding & Fact Sheet**: [`goals/update-check-lifecycle/facts.md`](facts.md)
- **Execution Plan**: [`goals/update-check-lifecycle/plan.md`](plan.md)

## Done Condition
1. All requirements described in the goal and execution plan are implemented.
2. All automated unit and integration tests pass cleanly with `-race` (`go test -race ./...` or stack equivalent).
3. Zero architectural regressions; definitions of done satisfied.
