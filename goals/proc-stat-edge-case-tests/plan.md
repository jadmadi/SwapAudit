# Plan: Expand Proc Stat Parsing and Malformed Input Tests

## Execution Steps
- [x] **Phase 1**: Add TestReadStartTicksMalformedStat for malformed proc stat formats
- [x] **Phase 2**: Add TestReadProcessLastActiveMalformedSched for missing sched keys
- [x] **Phase 3**: Verify test suite with race detector: go test -race ./internal/proc
