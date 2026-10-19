# Plan: Harden Background Update Check Lifecycle and Goroutine Hygiene

## Execution Steps
- [x] Phase 1: Read update cache state synchronously in app.Run to avoid spawning goroutines when cache is fresh
- [x] Phase 2: Pass context with cancellation or waitgroup to background updater routine in app.Run
- [x] Phase 3: Ensure app.Run cancels or waits on in-flight update checks before exiting
- [x] Phase 4: Update app_test.go to verify zero goroutines are leaked during CLI runs
- [x] Phase 5: Verify test suite with race detector: go test -race ./internal/app ./internal/updater
