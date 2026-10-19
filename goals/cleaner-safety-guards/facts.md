# Facts: Harden Process and Filesystem Cleanup Safety Guards

## Architectural Invariants & Constraints
- KillProcessTargets must never send signals to PID <= 1 or os.Getpid()
- PruneTmpTargets must reject any path outside /tmp/ or /var/tmp/
- PruneTmpTargets must re-verify file type via Lstat before calling os.RemoveAll
- Zero external dependencies; tests must pass with go test -race ./...

## File & Interface Contracts
- Relevant files:
  - `internal/cleaner/cleaner.go`
  - `internal/cleaner/cleaner_test.go`
