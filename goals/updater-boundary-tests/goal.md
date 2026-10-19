# Goal: Expand Updater Edge-Case and Download Boundary Tests

## Goal Description
Add unit tests for HTTP error status handling, malformed checksums.txt lines, oversized stream cutoffs in downloadFile, and UID-scoped cache path resolution in internal/updater/updater_test.go.

## Dependencies & Execution Order
- **Mode**: Dependent (requires prerequisite goals)
- **Depends On**: security-hardening-guards
- **Sequence**: -
- **Shape**: ship
- **Tier**: immediate

## Files to Touch
- internal/updater/updater_test.go

## References
- **Shared Understanding & Fact Sheet**: [`goals/updater-boundary-tests/facts.md`](facts.md)
- **Execution Plan**: [`goals/updater-boundary-tests/plan.md`](plan.md)

## Done Condition
1. All requirements described in the goal and execution plan are implemented.
2. All automated unit and integration tests pass cleanly with `-race` (`go test -race ./...` or stack equivalent).
3. Zero architectural regressions; definitions of done satisfied.
