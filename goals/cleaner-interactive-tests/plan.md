# Plan: Expand Cleaner Interactive Input and Confirmation Tests

## Execution Steps
- [x] **Phase 1**: Add TestAskConfirmationReaderVariants covering yes, y, no, n, empty, and EOF
- [x] **Phase 2**: Add TestPruneTmpTargetsLiveExecution verifying safe deletion in temporary directory
- [x] **Phase 3**: Verify test suite with race detector: go test -race ./internal/cleaner
