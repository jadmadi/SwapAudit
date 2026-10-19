# Facts: Align Makefile Install Target with Hardened Build Script

## Architectural Invariants & Constraints
- make install must install atomically to ~/.local/bin/swapaudit
- make install must preserve existing Makefile build, test, lint targets

## File & Interface Contracts
- Relevant files:
  - `Makefile`
