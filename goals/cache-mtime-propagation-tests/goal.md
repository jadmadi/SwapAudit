# Goal: Expand Cache Nested Directory Mtime Propagation Tests

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_cache-mtime-propagation-tests_1792407014`


## Goal Description
Add unit tests for ScanTmpDirectoriesIn verifying that recently modified nested files inside an older parent folder update the lastActive age, and test symlink exclusion in subdirectories in cache_test.go.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: roadmap

## Files to Touch
- internal/cache/cache_test.go

## References
- **Shared Understanding & Fact Sheet**: [`goals/cache-mtime-propagation-tests/facts.md`](facts.md)
- **Execution Plan**: [`goals/cache-mtime-propagation-tests/plan.md`](plan.md)

## Done Condition
1. All requirements described in the goal and execution plan are implemented.
2. All automated unit and integration tests pass cleanly with `-race` (`go test -race ./...` or stack equivalent).
3. Zero architectural regressions; definitions of done satisfied.
