# Plan: Implement ANSI-Aware Table Truncation and Styling Separation

## Execution Steps
- [x] Phase 1: Update term.Truncate to parse ANSI sequences and append reset code when truncating colored text
- [x] Phase 2: Separate raw cell values from column styling in display.go so RenderTable formats after width truncation
- [x] Phase 3: Add unit tests in term_test.go for Truncate with 8-bit and 24-bit ANSI sequences
- [x] Phase 4: Verify with go test -race ./internal/term
