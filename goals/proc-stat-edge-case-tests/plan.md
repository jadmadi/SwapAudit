# Plan: Expand Proc Stat Parsing and Malformed Input Tests

## Execution Steps
- [ ] **Phase 1**: Add TestReadStartTicksMalformedStat for malformed proc stat formats
- [ ] **Phase 2**: Add TestReadProcessLastActiveMalformedSched for missing sched keys
- [ ] **Phase 3**: Verify test suite with race detector: go test -race ./internal/proc
