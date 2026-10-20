# Goal: Implement ANSI-Aware Table Truncation and Styling Separation

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: CommandCode, 2026-10-20 (acceptance criteria re-checked against tests and evidence)
- **Implemented By**: CommandCode, 2026-10-20 (see `evidence.md`)
- **Mystery Audited By**: @mystery-auditor | Report: `receipt_ansi-truncate-rendering_1792402811`


## Goal Description
Fix string truncation in term.Truncate to track and preserve ANSI escape code sequences without breaking terminal styling when colored columns exceed their allocated width, or format column colors in RenderTable after alignment.

## Dependencies & Execution Order
- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: -
- **Shape**: ship
- **Tier**: roadmap

## Files to Touch
- internal/term/term.go
- internal/term/term_test.go
- internal/app/display.go

## References
- **Shared Understanding & Fact Sheet**: [`goals/ansi-truncate-rendering/facts.md`](facts.md)
- **Execution Plan**: [`goals/ansi-truncate-rendering/plan.md`](plan.md)

## Done Condition
1. All requirements described in the goal and execution plan are implemented.
2. All automated unit and integration tests pass cleanly with `-race` (`go test -race ./...` or stack equivalent).
3. Zero architectural regressions; definitions of done satisfied.
