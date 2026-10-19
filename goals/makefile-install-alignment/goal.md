# Goal: Align Makefile Install Target with Hardened Build Script

## Goal Description
Update Makefile install target to invoke scripts/build --install rather than plain cp, gaining atomic installation, writable permission checks, and PATH shadowing detection.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: nice-to-have

## Files to Touch
- Makefile

## References
- **Shared Understanding & Fact Sheet**: [`goals/makefile-install-alignment/facts.md`](facts.md)
- **Execution Plan**: [`goals/makefile-install-alignment/plan.md`](plan.md)

## Done Condition
1. All requirements described in the goal and execution plan are implemented.
2. All automated unit and integration tests pass cleanly with `-race` (`go test -race ./...` or stack equivalent).
3. Zero architectural regressions; definitions of done satisfied.
