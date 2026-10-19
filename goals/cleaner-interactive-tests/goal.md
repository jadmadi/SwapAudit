# Goal: Expand Cleaner Interactive Input and Confirmation Tests

## Goal Description
Add unit tests for AskConfirmationReader covering positive, negative, empty, and EOF inputs, and verify safe cleanup execution in cleaner_test.go.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: roadmap

## Files to Touch
- internal/cleaner/cleaner_test.go

## References
- **Shared Understanding & Fact Sheet**: [`goals/cleaner-interactive-tests/facts.md`](facts.md)
- **Execution Plan**: [`goals/cleaner-interactive-tests/plan.md`](plan.md)

## Done Condition
1. All requirements described in the goal and execution plan are implemented.
2. All automated unit and integration tests pass cleanly with `-race` (`go test -race ./...` or stack equivalent).
3. Zero architectural regressions; definitions of done satisfied.
