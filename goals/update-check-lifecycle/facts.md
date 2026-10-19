# Facts: Harden Background Update Check Lifecycle and Goroutine Hygiene

## Architectural Invariants & Constraints
- Startup must load ~/.cache/swapaudit/update_check.json synchronously without network latency
- Background update check goroutines must not outlive app.Run()
- Tests must not leak background goroutines (goleak compliant)
- Atomic state file writing must not leave temporary files on early exit

## File & Interface Contracts
- Relevant files:
  - `internal/app/app.go`
  - `internal/app/app_test.go`
  - `internal/updater/updater.go`
  - `internal/updater/updater_test.go`
