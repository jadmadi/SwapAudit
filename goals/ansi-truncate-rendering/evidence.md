# Evidence: ANSI-aware truncation and styling separation

Verdict: PASS

Date: 2026-10-20. Re-verified after the accounting/UI work touched `internal/term/term.go`.

## What changed since the original audit

The terminal package was refactored so every escape sequence is built by one guarded constructor, `ansi(code string)`. No raw ANSI literal remains outside that function. `colorEnabled` still gates every styling helper, and `InitTerminal` still disables color for non-TTY stdout, `NO_COLOR`, and `TERM=dumb`.

`Truncate`, `TruncateMiddle`, and `TruncateMiddleANSI` remain ANSI-aware and append a reset when styling was opened. Widths are measured with `StrWidth`, which ignores escape sequences.

## Verification

```text
$ go test -run "TestTruncate|TestStrWidth|TestColorsAndGlyphs|TestFitColumns|TestTableOverflow" -v ./internal/term
=== RUN   TestTruncate                  --- PASS
=== RUN   TestTruncateANSI              --- PASS
=== RUN   TestStrWidth                  --- PASS
=== RUN   TestColorsAndGlyphs           --- PASS
=== RUN   TestTruncateMiddleWidths      --- PASS
=== RUN   TestTruncateMiddleANSI        --- PASS
=== RUN   TestFitColumnsAndTableWidth   --- PASS
=== RUN   TestTableOverflow             --- PASS
PASS
ok  github.com/jadmadi/swapaudit/internal/term

$ grep -n 'func ansi' internal/term/term.go
122:func ansi(code string) string {
```

`TestTruncate` asserts no output exceeds the requested width; `TestTruncateANSI` asserts a reset suffix and no color bleed; `TestFitColumnsAndTableWidth` asserts exact table widths at 80/100/120 with no trailing whitespace.
