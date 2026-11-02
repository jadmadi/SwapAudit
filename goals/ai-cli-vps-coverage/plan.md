# Plan: AI CLI VPS Coverage

- [x] Task 1: Add `commandcode*`, `command-code*`, `opencode*`, `muse-*` to `DefaultTmpPatterns` in `internal/cache/cache.go`.
- [x] Task 2: Add `commandcode`, `cmd`, `command-code` matching to `CategorizeProcess` in `internal/triage/triage.go`.
- [x] Task 3: Add table-driven unit tests for `commandcode` and `cmd` in `internal/triage/triage_test.go`.
- [x] Task 4: Run race tests, linters, and binary compilation (`go test -race ./...`, `./scripts/lint`, `make build`).
- [x] Task 5: Verify against live `/tmp` artifacts (`/tmp/commandcode*`, `/tmp/muse-*`) via `./swapaudit --clean-all --dry-run`.
- [x] Task 6: Document VPS automated cron and watch setup recipes in docs/vps-ai-cli-setup.md.
