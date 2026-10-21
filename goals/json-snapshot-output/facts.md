# Facts: Add a Stable Machine-Readable JSON Snapshot

## 1. Grounded Evidence (live, 2026-10-21)

### 1.1 There is no machine-readable output

- `grep -rn "encoding/json" --include='*.go' .` finds only
  `internal/updater/updater.go:11` and its `json.MarshalIndent` at `86`, used for
  the update-check cache file. `internal/app` has no JSON imports.
- Every report renderer writes with `fmt.Printf`/`fmt.Println` to stdout:
  `display.go` `PrintHeader`, `PrintActionableSection`, `PrintMainTable`,
  `PrintCategorySummary`, `PrintQuickCommands`; `term.go` `PrintBoxHeader`,
  `RenderTable`, `PrintUpdateNotice`.
- `app.Run` renders unconditionally after the scan (`app.go:221-224`) and then
  prints either the cleaner output or `PrintQuickCommands` (`227-242`).
- The update notice is printed to stdout after cleanup (`244-258`). It must be
  suppressed in JSON mode or it will corrupt the document.

### 1.2 Scan and item semantics

- `CollectSwapItems` (`app.go:40-115`) appends
  `cache.ScanTmpDirectories(...)` first, then process items with
  `SwapBytes > 0`.
- Filesystem items carry `SizeBytes`, `Path`, `Age`, `LastActive`,
  `LastActiveAt`; `SwapBytes` stays `0`.
- Process items carry `SwapBytes`, `Age`, `LastActive`, `LastActiveAt`; `SizeBytes`
  stays `0`.
- `model.ItemSize` (`model.go:55-59`) resolves to `SizeBytes` when positive,
  else `SwapBytes`. The JSON schema exposes this as `effective_size_bytes`.
- `Count` is always `1` for raw scan items; aggregation happens only in
  `triage.AggregateShells` for the human table. JSON should not aggregate.
- If `os.ReadDir("/proc")` fails, `CollectSwapItems` returns only the tmp items
  (`app.go:47-50`). This is a partial scan, not a crash.
- `proc.ReadProcessLastActive` returns `-1` when `/proc/<pid>/sched` cannot be
  read (`proc.go:217-239`). `CollectSwapItems` then computes
  `lastActiveAt := now.Add(-lastActive)`, which is effectively `now` for `-1`
  (`app.go:71-72`). JSON must normalize the `-1` to `null`, not emit a negative
  number.
- `Age` and `LastActive` are `time.Duration`; JSON uses whole seconds via
  `int64(d / time.Second)`.
- `LastActiveAt` is a `time.Time`. Use RFC3339 with the local numeric offset.

### 1.3 Runtime inputs and determinism

- `app.Run` uses `time.Now()` at `app.go:214` and `proc.ReadUptime()` at `215`.
  Neither is injectable today, so a pure `BuildSnapshot` function is the test
  seam.
- `CollectSwapItems` already accepts `now`, `uptimeSec`, thresholds, and
  patterns, so the scan is deterministic for a fixed clock and filesystem.
- The human report branches on `term.TermWidth()` (`80..120`, default `100`) and
  on `w >= 100`. JSON must not depend on width at all.

### 1.4 Flags and exit codes today

- Flags are declared in `app.go:135-160`.
- `--sort` is validated at `205-212`; an invalid value exits `2`.
- `--version` exits `0` at `171-174`.
- Unknown flags exit `2` via `flag.ContinueOnError` at `162-169`.
- There is no `--json`, `--no-command`, or format flag.
- `--yes` only affects cleaner prompts; `--dry-run` only affects cleaner
  behavior.
- `os.Exit` is only in `cmd/swapaudit/main.go:12`, which forwards
  `app.Run(os.Args[1:])`.

### 1.5 Build metadata

- `internal/version/version.go` exposes `Version = "dev"`, `Commit = "none"`,
  `Date = "unknown"`, and `String()` returns
  `swapaudit <version> (commit: <commit>, built: <date>)`.
- The JSON schema should carry the three raw fields, not the formatted string,
  so consumers do not parse prose.

### 1.6 Test and tooling facts

- `internal/app/app_test.go` captures stdout/stderr with `os.Pipe` and asserts
  substrings (`TestRunVersion`, `TestRunInvalidSort`,
  `TestRunDryRunCleanAll`, `TestRunDefaultExecution`).
- `TestRunDefaultExecution` asserts that `Quick Maintenance Commands` appears
  when no cleanup flag is passed. This test will still pass after the picker
  because test stdout is a pipe (no TTY) and the picker is skipped.
- There is no `testdata/` directory and no golden-file test in the repository
  today. This goal introduces the first one.
- `go.mod` is `go 1.23.0` with no dependencies.
- `scripts/check.sh --pre-commit` runs scoped formatting, diff sentry, and
  modified-package tests; `--full` adds `go vet`, `golangci-lint`, the race
  detector, coverage, and a build check.

## 2. Architectural Invariants & Constraints

- `--json` is read-only. It must never call `cleaner.ExecuteCleanup`, write
  files outside stdout, or signal processes.
- Stdout is a data channel in JSON mode. No banner, box, color, progress, or
  update notice may be written to it.
- One scan per invocation. JSON must reuse the `items` slice already collected.
- The schema is versioned. Additive fields are allowed within v1; removing or
  retyping a field requires `schema_version: 2`.
- Nullability is explicit. Use pointers for `pid`, `start_ticks`, `path`,
  `age_seconds`, `idle_seconds`, and `last_active_at`; never overload `0`, `""`,
  or `-1` as "unknown".
- Arrays are never `null`: `summary`, `items`, and `warnings` must be empty
  arrays when empty.
- Ordering is canonical and independent of the human report's `--sort`,
  `--limit`, and `--all`.
- `BuildSnapshot` is pure: no clock, environment, filesystem, or network access.
- No `syscall` types leak into the JSON schema.
- HTML escaping is disabled so paths and commands round-trip exactly.

## 3. File & Interface Contracts

### `internal/snapshot/snapshot.go`

The package is placed outside `internal/app` on purpose. `internal/live` (the
dependent sampler goal) must import the schema and builder, while `internal/app`
imports `internal/live` to wire the `sample` subcommand. Keeping the schema in
`internal/app` would create a cycle: `app -> live -> app`. A neutral
`internal/snapshot` package keeps the graph acyclic and is the only acceptable
location for the v1 schema types.

```go
type Snapshot struct {
    SchemaVersion int             `json:"schema_version"`
    Tool          Tool            `json:"tool"`
    GeneratedAt   string          `json:"generated_at"`
    UptimeSeconds float64         `json:"uptime_seconds"`
    Thresholds    Thresholds      `json:"thresholds"`
    System        System          `json:"system"`
    Totals        Totals          `json:"totals"`
    Summary       []SummaryEntry  `json:"summary"`
    Items         []Item          `json:"items"`
    Warnings      []string        `json:"warnings"`
}

func BuildSnapshot(now time.Time, uptimeSeconds float64, sys model.SystemSwap,
    items []model.SwapItem, maxAge, idleThreshold time.Duration,
    noCommand bool) Snapshot
```

- `BuildSnapshot` must initialize `Summary`, `Items`, and `Warnings` to empty
  slices, never leave them `nil`.
- `GeneratedAt` is `now.Format(time.RFC3339)`.
- `UptimeSeconds` is the raw float from `/proc/uptime`.
- Thresholds are `int64(maxAge / time.Second)` and
  `int64(idleThreshold / time.Second)`.

### `internal/app/app.go`

- Add `jsonOutput` and `noCommand` flags.
- Validate conflicts before `term.InitTerminal`, before the update check, and
  before `CollectSwapItems`.
- When `jsonOutput`, skip `term.InitTerminal`'s human rendering decisions, skip
  the update goroutine entirely, build the snapshot, encode it, and return.
- Never call `PrintHeader`, `PrintActionableSection`, `PrintMainTable`,
  `PrintCategorySummary`, `PrintQuickCommands`, or `PrintUpdateNotice` in JSON
  mode.
- All JSON-mode errors go to stderr.

### `docs/json-schema.md`

Must contain:

1. The schema version and the additive-change rule.
2. A table of every field with type, nullability, units, and source.
3. The canonical ordering rule.
4. The `effective_size_bytes` rule.
5. The `idle_seconds: null` rule.
6. The `--no-command` privacy note.
7. Two recipes: a shell statusline using `python3 -c` or `jq`, and a CI check
   that fails when `cleanable_bytes` exceeds a threshold.
8. One complete example document.

## 4. External Contracts & Failure Modes

| Condition | Detection | Expected behavior |
| :--- | :--- | :--- |
| Cleanup flag with `--json` | flag validation | Exit 2, exact message, no scan |
| `--dry-run`/`--yes` with `--json` | flag validation | Exit 2, exact message |
| `--interactive` with `--json` | flag validation | Exit 2, exact message |
| `/proc/self/status` missing | `os.Stat` preflight | Error document, exit 1 |
| Individual process unreadable | empty command name from scan | Item still emitted if swap > 0; `warnings` gains `partial_proc_scan` |
| `idle_seconds` unavailable | `ReadProcessLastActive` returns `-1` | Emit `null` |
| Cache path empty | item from scan has empty `Path` | Emit `null`, not `""` |
| Zero items | empty scan | Valid document with `items: []`, `summary: []`, zero totals |
| Encode failure | `json.Encoder` error | Stderr diagnostic, exit 1, no partial stdout |
| Broken pipe | write error | Exit 1; do not retry |
| Time zone | local offset | RFC3339 with numeric offset; no UTC coercion |
| Non-ASCII path | UTF-8 | Emitted as UTF-8; `SetEscapeHTML(false)` |

## 5. Verification Matrix

| Test | Command | Expected |
| :--- | :--- | :--- |
| Build | `go build ./...` | Pass |
| Vet | `go vet ./...` | Pass |
| Unit + golden | `go test -race ./internal/snapshot ./internal/app` | Pass |
| Full race | `go test -race ./...` | Pass |
| Pre-commit | `./scripts/check.sh --pre-commit` | Pass |
| Full gate | `./scripts/check.sh --full` | Pass |
| Valid JSON | `go run ./cmd/swapaudit --json --no-update-check \| python3 -m json.tool` | Exit 0 |
| Stream purity | capture stdout/stderr | stdout is one JSON doc; stderr empty |
| Conflicts | each FR-2 combination | Exit 2, exact message |
| No command | `--json --no-command` | Every `command` is `""` |
| Determinism | two `BuildSnapshot` calls with fixed inputs | Byte-identical |
| Ordering | shuffled fixture | FR-6 order |
| Nullability | fixtures | No `-1`, no `null` arrays |
| Hard failure | fixture with proc preflight forced false | Exact error document, exit 1 |
| Marshal cost | benchmark or `time` | Under 20 ms for 500 items |

## 6. Open Questions / Deferred Decisions

- Should `generated_at` be UTC? The goal uses the local offset for human
  correlation; a future v2 may add `generated_at_unix`.
- Should thresholds include the resolved tmp patterns? Deferred.
- Should `--json` support a compact (`--json-compact`) form for statuslines?
  Deferred; consumers can minify.
- Should the schema include a `partial` boolean in addition to `warnings`?
  Deferred; warnings are sufficient for v1.
- Should cleanup results be serializable? Explicitly out of scope.

## 7. References

- `internal/app/app.go`
- `internal/app/display.go`
- `internal/model/model.go`
- `internal/proc/proc.go`
- `internal/cache/cache.go`
- `internal/version/version.go`
- `internal/updater/updater.go` (the only existing JSON usage)
- `goals/live-sampler-rates/goal.md` (dependent consumer)
