# Plan: Add a Stable Machine-Readable Audit and Cleanup Contract

## Execution Steps

### Phase 1 — Contract freeze

- [x] Capture the current human output and the current exit codes for:
  - `./swapaudit --no-color --ascii`
  - `./swapaudit --bad-flag; echo $?` (expect 2)
  - `printf '' | ./swapaudit --no-color --ascii --clean-all; echo $?` (current behavior)
  - `./swapaudit --dry-run --clean-all; echo $?`
- [x] Write `docs/json-schema.md` with the J1–J6 contract before implementation.
- [x] Decide and record the nullability of unknown idle (`null` + `idle_known=false`).
- [x] Run `go test -race ./...` and `go vet ./...`; record the baseline.

### Phase 2 — JSON types and encoder

- [x] Add `internal/report/json.go` with explicit `JSONReport`, `JSONSystem`, `JSONAttribution`, `JSONCategory`, `JSONGroup`, `JSONItem`, `JSONCleanup`, `JSONError`.
- [x] Implement `ToJSON(AuditReport) JSONReport` and `WriteJSON(io.Writer, AuditReport) error`.
- [x] Use integer bytes and seconds, RFC3339 UTC, sorted arrays, no maps at the top level.
- [x] Add `TestJSONSchemaFields`, `TestJSONFullNames`, `TestJSONNoANSI`, `TestJSONDeterministic`.
- [x] Add a golden fixture `internal/report/testdata/audit-v1.golden.json` with fixed `generated_at`/host fields.
- [x] Add `TestJSONGolden`.
- [x] Run `go test -race ./internal/report`.

### Phase 3 — Wire `--json` into the app

- [x] Add `--json` to the flag set.
- [x] If `--json` is set, skip human rendering and call `WriteJSON(os.Stdout, report)`.
- [x] Route diagnostics to stderr; stdout is JSON only.
- [x] Reject `--json upgrade` with exit 2.
- [x] Add `--json` to `PrintUsage`.
- [x] Add `TestJSONOutputIsSingleObject`, `TestJSONUpgradeRejected`.
- [x] Run `go test -race ./internal/app`.

### Phase 4 — Cleanup result and JSON cleanup

- [x] Add `CleanupResult`, `CleanupGroupResult`, `ReasonCount` to `internal/cleaner`.
- [x] Refactor `ExecuteCleanup` to return `CleanupResult` while preserving the human output.
- [x] Add `CleanupResult` to `JSONReport`; `null` when no cleanup flags.
- [x] `--json --dry-run --clean-*` builds a plan without mutation.
- [x] `--json --clean-*` without `--yes`/`--dry-run` exits 2 before mutation.
- [x] `--json --yes --clean-*` executes and emits the result.
- [x] Add `TestCleanupResultMatchesSummary`, `TestJSONCleanupRequiresYes`, `TestJSONDryRun`, `TestJSONYesCleanup`.
- [x] Run `go test -race ./internal/cleaner ./internal/app`.

### Phase 5 — Exit codes and flag matrix

- [x] Add `ExitOK/ExitRuntime/ExitUsage/ExitCleanupPartial` constants.
- [x] Return 3 when `remove-failed`/`kill-failed` occurred, even if other targets succeeded.
- [x] Return 2 for usage errors before mutation; return 1 for report/encode failures.
- [x] Ensure `--dry-run` works in any flag order; `--yes` without cleanup warns.
- [x] Add `TestExitCodes`, `TestDryRunAnyOrder`, `TestYesWithoutCleanup`.
- [x] Run `go test -race ./internal/app ./internal/cleaner`.

### Phase 6 — Documentation and verification

- [x] Finish `docs/json-schema.md` with examples and the exit-code matrix.
- [x] Update `README.md`, `CHANGELOG.md`, and `PrintUsage`.
- [x] Run `gofmt -l .`, `go vet ./...`, `go test -race ./...`.
- [x] Run the targeted tests.
- [x] Run the `jq` checks if `jq` is available; otherwise record that the Go schema tests are the mandatory gate.
- [x] Run `./scripts/check.sh --full`.
- [x] Record outputs in `evidence.md`.

### Phase 7 — Attribution commit

- [x] Commit with `feat:` and a subject containing `(goals/machine-readable-audit/goal.md)`.
- [x] Run `sila goals --project=/home/jad/scripts/active/swapaudit`.
- [x] Do not push to a remote unless the user explicitly authorizes it.

## Tests to Add

| Test | File | What it proves |
| --- | --- | --- |
| `TestJSONGolden` | `internal/report/json_test.go` | Full schema fixture |
| `TestJSONDeterministic` | same | Stable bytes/order |
| `TestJSONFullNames` | same | No truncation |
| `TestJSONNoANSI` | same | Clean stdout |
| `TestJSONSchemaFields` | same | Required keys/types |
| `TestJSONReconciliation` | same | JSON totals tie to header |
| `TestJSONOutputIsSingleObject` | `internal/app/app_test.go` | One object, no human text |
| `TestJSONUpgradeRejected` | same | Exit 2 |
| `TestJSONCleanupRequiresYes` | same | Exit 2, no partial output |
| `TestJSONDryRun` | same | `cleanup.dry_run=true`, no mutation |
| `TestJSONYesCleanup` | same | Cleanup object and exit 0/3 |
| `TestExitCodes` | same | 0/1/2/3 |
| `TestDryRunAnyOrder` | same | Flag-order independence |
| `TestYesWithoutCleanup` | same | Warning, exit 0 |
| `TestCleanupResultMatchesSummary` | `internal/cleaner/cleaner_test.go` | Structured counts equal human counts |

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -race -run 'TestJSON|TestExitCodes|TestDryRun|TestYes' ./internal/report ./internal/app ./internal/cleaner
./swapaudit --json | jq -e '.schema_version == 1 and (.system.swap.used_bytes > 0)'
./swapaudit --json | jq -e '.attribution.swap.attributed_bytes + .attribution.swap.unattributed_bytes == .system.swap.used_bytes'
./swapaudit --json --clean-all >/tmp/json-clean.out 2>/tmp/json-clean.err; echo $?   # expect 2
./swapaudit --json --dry-run --clean-all | jq -e '.cleanup.dry_run == true'
./swapaudit --json upgrade; echo $?   # expect 2
./swapaudit --json | grep -c $'\x1b' || true   # expect 0
./scripts/check.sh --full
```

## Rollback

- Revert the implementation commit. The human CLI remains unchanged.
- If cleanup JSON is too complex, ship audit-only JSON and defer the cleanup object; keep the exit codes and non-TTY guard.
- Do not scrape the human output as a substitute.

## Completion Evidence

`goals/machine-readable-audit/evidence.md` must contain:

1. The schema golden fixture and the `jq` outputs.
2. The exit-code matrix results.
3. The cleanup JSON dry-run and live (`--yes`) outputs.
4. The JSON/no-ANSI/full-name checks.
5. `go test -race`, `go vet`, and `scripts/check.sh --full` outputs.
6. Any deviation from this plan and why.
