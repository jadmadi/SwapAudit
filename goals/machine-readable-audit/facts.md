# Facts: Add a Stable Machine-Readable Audit and Cleanup Contract

## Architectural Invariants & Constraints

- **stdout is data, stderr is diagnostics.** `--json` prints exactly one JSON object on stdout; no banner, progress, color, or human update notice.
- **One report source.** JSON and human output consume the same `AuditReport`/`CleanupResult`; no display-time recomputation.
- **Integer bytes and seconds.** No floats for byte counts; no ambiguous duration strings; unknown idle is `null` + `idle_known: false`.
- **Deterministic output.** Sorted reasons/categories/groups/items; no maps at the top level; fixed struct field order.
- **Versioned schema.** `schema_version` starts at 1; a breaking change bumps it and is documented.
- **Cleanup is explicit.** `--json` live cleanup requires `--yes` or `--dry-run`; no prompt can hang a pipeline.
- **Exit codes are a contract.** 0 success, 1 runtime, 2 usage, 3 cleanup partial failure.
- **No new dependency.** `encoding/json`, `crypto/sha256`, standard library only.
- **No network schema, no telemetry, no daemon.**
- **Full identity.** JSON carries full names/paths and short IDs; no truncation.

## Observed Facts (Repository)

| Fact | Evidence | Consequence |
| --- | --- | --- |
| No `--json` flag | `internal/app/app.go:160-180` | No user-facing structured output |
| No structured renderer | `internal/app/display.go` uses `fmt.Printf` tables | Scraping is the only automation path today |
| `--yes` exists | `internal/app/app.go:170-171`; `internal/cleaner/cleaner.go:123-131`, `:216-224` | Reuse, do not add a second flag |
| `--dry-run` is a modifier | same | Lock it with tests; no new flag |
| Exit codes are 0/2 only | `internal/app/app.go:186-205` | No operational-failure signal |
| Non-TTY prompts return false | `internal/cleaner/cleaner.go:81-89` | Silent skip/exit 0 in pipelines |
| Cleanup summary is human-only | `internal/cleaner/cleaner.go:376-410` | Need a structured result |
| Goal 1 has `AuditReport` | `internal/report/accounting.go` (planned) | JSON can be a pure projection |
| Goal 4 has groups/IDs | `internal/triage/group.go` (planned) | JSON can reuse them |
| `encoding/json` is stdlib | `go.mod` has no dependencies | No new module needed |

## Observed Facts (Live Host, 2026-10-19)

| Observation | Command | Value |
| --- | --- | --- |
| Human output is the only format | `./swapaudit --no-color --ascii` | Banner, tables, quick commands |
| Current usage error | `./swapaudit --bad-flag; echo $?` | stderr `error: flag provided but not defined: -bad-flag`, exit 2 |
| Current dry-run | `./swapaudit --no-color --ascii --dry-run --clean-all` | Human preview, exit 0 |
| Current non-TTY cleanup | `printf '' | ./swapaudit --no-color --ascii --clean-all; echo $?` | Silent skip, exit 0 (to be changed by Goal 3) |
| `jq` availability | `command -v jq` | Unknown; Go tests are mandatory, jq checks optional |
| Audit size | `./swapaudit --no-color --ascii --all --limit 0` | 805 items in the current sample; JSON must remain practical |

## External Facts

- `encoding/json` emits struct fields in declaration order; maps are sorted by key in modern Go, but use slices for deterministic order regardless.
- `json.Encoder.Encode` appends a newline; do not add a second one.
- RFC3339 UTC timestamps are unambiguous; use `time.RFC3339`.
- `int64` JSON numbers are exact up to 2^53 in JavaScript; byte counts on current hosts are well below that. Document the limit.
- `jq` is useful for examples but is not a build dependency.
- `SIGTERM`/`SIGKILL` semantics are unrelated to the JSON format; the cleanup object reports the signal used.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| Consumers can ignore unknown fields | Forward compatibility | Golden test + `json.Unmarshal` into a subset struct |
| Integer seconds are sufficient | CLI durations are human-scale | Golden fixture with 1s/1h/12h |
| One final cleanup object is enough for v1 | Avoids NDJSON complexity | Dry-run/live tests |
| Exit code 3 is the right partial-failure signal | Distinct from usage/runtime | Test one remove failure + one success |
| `--json` should not include the quick commands | Machine output should be data only | Assert absence of human text |
| Full names are worth the bytes | Automation needs exact paths | `TestJSONFullNames` |
| The schema can reuse Goal 4's group model | Avoid duplicate grouping logic | `TestJSONGroups` |

## Unknowns

- How large a JSON document gets on a host with 100k `/tmp` entries. **Unknown.** Add a benchmark; if needed, document a future `--json=summary` mode.
- Whether consumers need stable item ordering by name/path rather than by size. **Design choice.** Default to the active sort, then path/ID; document.
- Whether unknown idle should be `null` or a sentinel. **Decision:** `null` + `idle_known=false`.
- Whether live cleanup JSON should stream events. **Decision:** no in v1; a future NDJSON schema can add it.
- Whether `--json` should include the update check result. **Decision:** optional `update` object; no human notice on stdout.

## Evidence Commands

```bash
grep -n "--json\|--dry-run\|--yes" internal/app/app.go
grep -n "AskConfirmation\|os.Err\|return 0\|return 2" internal/app/app.go internal/cleaner/cleaner.go
./swapaudit --bad-flag; echo $?
printf '' | ./swapaudit --no-color --ascii --clean-all; echo $?
command -v jq || true
go test -race ./...
```

## File & Interface Contracts

- `internal/report/json.go` (new): `JSONReport`, `JSONSystem`, `JSONAttribution`, `JSONCategory`, `JSONGroup`, `JSONItem`, `JSONCleanup`, `JSONError`, `ToJSON`, `WriteJSON`.
- `internal/report/accounting.go` (Goal 1): the `AuditReport` fields the JSON projection reads.
- `internal/app/app.go`: `--json`, output routing, exit codes, cleanup guard.
- `internal/cleaner/cleaner.go`: `CleanupResult`, `CleanupGroupResult`, `ReasonCount`; `ExecuteCleanup` returns the result.
- `internal/triage/group.go` (Goal 4): group model reused by JSON.
- `docs/json-schema.md` (new): the J1–J6 contract.
- `README.md`, `CHANGELOG.md`: flag, examples, exit codes.
- `go.mod`: unchanged.
