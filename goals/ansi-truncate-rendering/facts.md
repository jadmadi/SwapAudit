# Facts: Implement ANSI-Aware Table Truncation and Styling Separation

## Architectural Invariants & Constraints
- Truncating a string containing ANSI escape codes must never drop the reset code or split an escape sequence
- Visible string width calculated by StrWidth must strictly match rendered columns
- RenderTable output must maintain stable column alignment in both ASCII and Unicode modes

## File & Interface Contracts
- Relevant files:
  - `internal/term/term.go`
  - `internal/term/term_test.go`
  - `internal/app/display.go`
