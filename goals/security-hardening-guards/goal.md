# Goal: Harden Self-Updater Stream Limits and Temporary File Confinement

## Goal Description
Harden self-updater and build script against denial of service and temporary file collisions: enforce a 50MB stream limit on release binary downloads and 1MB on checksum files in updater.go, scope fallback cache state filenames to the current UID, and enforce fail-closed temporary file creation in scripts/build.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: immediate

## Files to Touch
- internal/updater/updater.go
- internal/updater/updater_test.go
- scripts/build

## References
- **Shared Understanding & Fact Sheet**: [`goals/security-hardening-guards/facts.md`](facts.md)
- **Execution Plan**: [`goals/security-hardening-guards/plan.md`](plan.md)

## Done Condition
1. All requirements described in the goal and execution plan are implemented.
2. All automated unit and integration tests pass cleanly with `-race` (`go test -race ./...` or stack equivalent).
3. Zero architectural regressions; definitions of done satisfied.
