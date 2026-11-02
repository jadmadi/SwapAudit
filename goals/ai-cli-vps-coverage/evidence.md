# Evidence: AI CLI VPS Coverage

## Verification Command
```bash
./scripts/lint && go test -v -race ./... && ./swapaudit --clean-all --dry-run
```

## Test Results
- `internal/triage`: Table-driven tests pass for `commandcode`, `cmd`, `muse`, `opencode`, `agy`, and desktop daemons.
- `internal/cache`: Default tmp patterns include `commandcode*`, `command-code*`, `opencode*`, `muse-*`.
- Race detector: 15/15 packages passed with 0 data races.
- Linters: gofmt and go vet clean.

## Live Detection Evidence
`./swapaudit --clean-all --dry-run` successfully identified:
- `/tmp/commandcode-1000` (105.5 MB)
- `/tmp/commandcode` (704.1 KB)
- `/tmp/muse-out`, `/tmp/muse-r`, `/tmp/muse-live`, `/tmp/muse-shell-sandbox-*`
- `/tmp/opencode` (41.8 KB)
- Orphaned PID 2171409 (`muse-bin-1.4.14 upgrade`)

Verdict: PASS
