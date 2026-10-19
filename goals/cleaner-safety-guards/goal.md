# Goal: Harden Process and Filesystem Cleanup Safety Guards

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_cleaner-safety-guards_1792402779`


## Goal Description
Add defensive bounds to process termination and filesystem pruning in cleaner.go: guard against killing PID <= 1 and PID == self in KillProcessTargets, enforce strict /tmp/ and /var/tmp/ path containment and pre-removal Lstat symlink checks in PruneTmpTargets, and add negative unit tests for all boundary conditions.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: immediate

## Files to Touch
- internal/cleaner/cleaner.go
- internal/cleaner/cleaner_test.go

## References
- **Shared Understanding & Fact Sheet**: [`goals/cleaner-safety-guards/facts.md`](facts.md)
- **Execution Plan**: [`goals/cleaner-safety-guards/plan.md`](plan.md)

## Done Condition
1. All requirements described in the goal and execution plan are implemented.
2. All automated unit and integration tests pass cleanly with `-race` (`go test -race ./...` or stack equivalent).
3. Zero architectural regressions; definitions of done satisfied.
