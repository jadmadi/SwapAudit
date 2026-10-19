# Facts: Harden Self-Updater Stream Limits and Temporary File Confinement

## Architectural Invariants & Constraints
- Ground all implementations in official repository standards.
- Ensure deterministic verification before closing goal.

## File & Interface Contracts
- Relevant files:
  - `internal/updater/updater.go`
  - `internal/updater/updater_test.go`
  - `scripts/build`
