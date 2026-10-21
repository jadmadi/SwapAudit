# Goal: Add a Stable Machine-Readable JSON Snapshot

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit (personas 5, 25, 32)
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: Pending

## Goal Description

`swapaudit` has no machine-readable output. Every consumer — scripts, a tmux or
shell statusline, CI, an editor plugin, or a future live view — must either
scrape the human tables or re-implement `/proc` parsing. This goal adds a
read-only `--json` mode that emits exactly one documented JSON document,
versioned from day one, with canonical ordering so golden tests and diffs are
meaningful.

The encoder itself is small. The engineering work is in the contract: deciding
what is nullable, how durations and bytes are typed, how categories are ordered,
what happens on a partial scan, and how to keep human output and update notices
off stdout. This goal treats that contract as the deliverable.

### Desired outcome

- `swapaudit --json` writes one valid JSON document to stdout and nothing else.
- `swapaudit --json | jq .` (or `python3 -m json.tool`) succeeds.
- The document contains the system counters, thresholds, per-category totals,
  and every raw item, never truncated by `--limit` or aggregated by `--all`.
- Two runs on the same machine produce byte-identical documents except for
  `generated_at`, and a frozen-clock builder test produces byte-identical
  output across runs.
- Destructive flags cannot be combined with `--json`; the mode is provably
  read-only.
- A future `live-sampler-rates` can embed this document and add a `window`
  block without breaking v1 consumers.

### Non-goals

- No live sampling, rates, ETA, deltas, or streaming (owned by
  `live-sampler-rates`).
- No interactive picker (owned by `one-shot-action-picker`).
- No JSON output for cleanup results in v1. The cleaner remains human-only.
- No YAML, TOML, CSV, or Prometheus exposition formats.
- No HTTP server, daemon, or statusline renderer.
- No schema-code generation or third-party JSON library.

## Scope & Boundaries

### In scope

- `internal/snapshot/snapshot.go` (new): schema types and a pure
  `BuildSnapshot`. The package is deliberately neutral: `internal/app` and the
  dependent `internal/live` sampler both import it, so placing it in
  `internal/app` would create an import cycle.
- `internal/snapshot/snapshot_test.go` (new): golden and round-trip tests.
- `internal/app/app.go`: `--json`, `--no-command`, conflict validation, early
  dispatch, update-check suppression.
- `internal/app/display.go`: usage text.
- `internal/model/model.go`: only if a shared type is genuinely required; prefer
  keeping snapshot types local to `internal/app`.
- `docs/json-schema.md` (new): the human-readable contract.
- `README.md`, `CHANGELOG.md`.
- `goals/json-snapshot-output/evidence.md`.

### Out of scope

- `internal/live` and any sampler.
- Cleaner output or result serialization.
- Changes to `CollectSwapItems` behavior.
- Sorting or filtering semantics used by the human report.

## Requirements

### Functional requirements

**FR-1 — Flag and mode.** Add a boolean `--json` flag. It is a machine mode:
stdout is reserved for one JSON document, diagnostics go to stderr, and the
update check is disabled (`--json` implies `--no-update-check`; no network call
and no "new release" notice).

**FR-2 — Mutual exclusions.** These combinations exit 2 before scanning, with
the exact stderr messages below:

| Combination | Message |
| :--- | :--- |
| `--json` with any of `--clean-untouched`, `--clean-abandoned`, `--clean-dormant`, `--clean-orphaned`, `--clean-tmp`, `--clean-all` | `error: --json is read-only and cannot be combined with cleanup flags` |
| `--json` with `--dry-run` | `error: --json is read-only and cannot be combined with --dry-run` |
| `--json` with `--yes`/`-y` | `error: --json is read-only and cannot be combined with --yes` |
| `--json` with `--interactive` | `error: --json cannot be combined with --interactive` |
| `--json` with `--no-color`/`--ascii` | Accepted; both are irrelevant and must not alter the document |
| `--no-command` without `--json` | `error: --no-command requires --json` |

**FR-3 — Single document.** On success, stdout contains exactly one JSON object,
two-space indented, followed by exactly one `\n`. No banner, box, table, prompt,
quick-commands block, or update notice may precede or follow it. All
diagnostics, if any, go to stderr.

**FR-4 — Schema version 1.** The top-level object is:

```json
{
  "schema_version": 1,
  "tool": {
    "name": "swapaudit",
    "version": "dev",
    "commit": "none",
    "built": "unknown"
  },
  "generated_at": "2026-10-21T09:15:04+03:00",
  "uptime_seconds": 123456.78,
  "thresholds": {
    "max_age_seconds": 43200,
    "idle_threshold_seconds": 7200
  },
  "system": {
    "swap_total_bytes": 18321752064,
    "swap_used_bytes": 8971677696,
    "swap_free_bytes": 9350074368,
    "mem_total_bytes": 33549176832,
    "mem_used_bytes": 15874695168,
    "mem_available_bytes": 17674481664,
    "tmpfs_total_bytes": 16777216000,
    "tmpfs_used_bytes": 7838310400
  },
  "totals": {
    "items": 364,
    "process_swap_bytes": 2134818816,
    "tmpfs_cache_bytes": 4103471104,
    "cleanable_bytes": 605617408,
    "cleanable_process_swap_bytes": 207564800,
    "cleanable_tmpfs_bytes": 398052608
  },
  "summary": [
    {
      "category": "active-caches",
      "items": 339,
      "swap_bytes": 0,
      "size_bytes": 3331655680,
      "cleanable_items": 0,
      "cleanable_swap_bytes": 0,
      "cleanable_size_bytes": 0
    }
  ],
  "items": [
    {
      "pid": 1618347,
      "start_ticks": 123456789,
      "name": "chrome-devtools",
      "command": "chrome-devtools-mcp --port 9222",
      "category": "dormant-agents",
      "swap_bytes": 94057267,
      "size_bytes": 0,
      "effective_size_bytes": 94057267,
      "age_seconds": 104880,
      "idle_seconds": 43980,
      "last_active_at": "2026-10-20T21:02:04+03:00",
      "path": null,
      "cleanable": true
    }
  ],
  "warnings": []
}
```

**FR-5 — Types and nullability.** The contract is exact:

| Field | Type | Null? | Rule |
| :--- | :--- | :---: | :--- |
| `schema_version` | integer | no | Always `1`. |
| `tool.version` | string | no | `version.Version` (for example `dev`). |
| `tool.commit` | string | no | `version.Commit`. |
| `tool.built` | string | no | `version.Date`. |
| `generated_at` | string | no | RFC3339 with numeric offset, second precision. |
| `uptime_seconds` | number | no | `/proc/uptime` first field; the only float. |
| `thresholds.*` | integer | no | Truncated to whole seconds. |
| `system.*` | integer | no | Bytes; never negative; zero when unknown. |
| `totals.*` | integer | no | Bytes or counts; never negative. |
| `summary[].category` | string | no | Exact `model.Category` value. |
| `summary[]` | array | no | Sorted by `category` ascending; empty array, never null. |
| `items[].pid` | integer | yes | `null` for filesystem caches. |
| `items[].start_ticks` | integer | yes | `null` for filesystem caches. |
| `items[].name` | string | no | Display name, already derived by `CollectSwapItems`. |
| `items[].command` | string | no | Full command line or path; `""` with `--no-command`. |
| `items[].category` | string | no | Exact `model.Category`. |
| `items[].swap_bytes` | integer | no | Process swap; `0` for caches. |
| `items[].size_bytes` | integer | no | Cache size; `0` for processes. |
| `items[].effective_size_bytes` | integer | no | `size_bytes` if positive, else `swap_bytes`. |
| `items[].age_seconds` | integer | yes | Whole seconds; `null` when a process has `start_ticks == 0` or a cache has no mtime. `0` is a valid age. |
| `items[].idle_seconds` | integer | yes | Whole seconds; `null` when `/proc/<pid>/sched` is unreadable. Never `-1`. |
| `items[].last_active_at` | string | yes | RFC3339 with offset; `null` exactly when `idle_seconds` is `null`. |
| `items[].path` | string | yes | Absolute path for caches; `null` for processes. |
| `items[].cleanable` | boolean | no | Whether the item is actionable. |
| `warnings` | array of strings | no | Empty array, never null. |
| `error` | object | only on failure | Present only in the hard-failure document. |

Negative `idle_seconds` from `proc.ReadProcessLastActive` must be normalized to
`null`; `-1` must never appear in the output.

**FR-6 — Canonical ordering.** JSON ignores `--sort`, `--limit`, and `--all`.
Items are ordered by:

1. `category` ascending (byte order of the category string),
2. `effective_size_bytes` descending,
3. `name` ascending,
4. `pid` ascending with `null` last.

This order is part of the contract and is asserted by the golden test.

**FR-7 — Summary construction.** `summary` has one entry per category present in
the scan, sorted ascending. Counts use `SwapItem.Count` (currently 1 per raw
item). Bytes sum the raw `SwapBytes` and `SizeBytes`; cleanable variants sum only
items with `Cleanable=true`.

**FR-8 — Totals.** `totals.items` is `len(items)`. `process_swap_bytes` sums all
`swap_bytes`. `tmpfs_cache_bytes` sums all `size_bytes`. `cleanable_bytes` is the
sum of `effective_size_bytes` over cleanable items, which matches the human
report's "Cleanable" figure. The two split fields disambiguate what part is swap
versus tmpfs.

**FR-9 — `--no-command`.** Add `--no-command`, valid only with `--json`; using
it alone exits 2 with `error: --no-command requires --json`. It sets every
`items[].command` to `""` while keeping the field present. Rationale:
command lines can contain tokens or paths that should not be logged by a
statusline or CI artifact. Without the flag, behavior matches the human report.

**FR-10 — Hard failure.** If `/proc/self/status` cannot be read (non-Linux or a
severely broken procfs), emit exactly one object:

```json
{
  "schema_version": 1,
  "error": {
    "code": "proc_unavailable",
    "message": "cannot read /proc/self/status"
  }
}
```

and exit 1. Partial scans do not fail: unreadable individual processes are
already skipped by `CollectSwapItems`. In v1 `warnings` is always an empty
array; it is reserved for future scan diagnostics because the current scan does
not surface per-process read failures.

**FR-11 — Determinism.** Extract a pure function:

```go
func BuildSnapshot(now time.Time, uptimeSeconds float64, sys model.SystemSwap,
    items []model.SwapItem, maxAge, idleThreshold time.Duration,
    noCommand bool) Snapshot
```

`app.Run` calls it with `time.Now()`. Tests call it with a fixed `now` and fixed
inputs. `BuildSnapshot` must not read the clock, environment, or filesystem.

**FR-12 — Encoding.** Use `encoding/json` with `Encoder.SetEscapeHTML(false)` and
`SetIndent("", "  ")`. No HTML escaping of `<`, `>`, or `&` in commands or paths.
Write to `os.Stdout` only after successful marshalling; if marshalling fails,
write the error to stderr and exit 1 without partial stdout.

**FR-13 — Exit codes.** `0` success; `1` hard failure or encode failure; `2` flag
conflict or invalid flag combination. No other codes.

**FR-14 — Documentation.** Add `docs/json-schema.md` with the full schema table,
nullability rules, ordering guarantee, a worked example, two consumer recipes
(statusline and disk-usage check), and the `--no-command` privacy note. Link it
from README and `PrintUsage`.

### Non-functional requirements

- **NFR-1 — Dependency floor.** Standard library only; Go 1.23; no schema
  generator.
- **NFR-2 — Size ceiling.** `snapshot.go` at most 300 lines; tests at most 400.
- **NFR-3 — Cost.** The JSON path must not add a second scan. It reuses the
  `items` slice already collected. Marshalling 500 items must stay under 20 ms
  on this workstation (measured in the evidence).
- **NFR-4 — Stream purity.** A test captures stdout and stderr separately and
  asserts stdout parses as JSON with no leading/trailing bytes and stderr is
  empty on success.
- **NFR-5 — Read-only.** `--json` must not call `ExecuteCleanup`, write files, or
  signal processes. Assert with an executor spy in the app test.
- **NFR-6 — Stability.** Adding a field is a minor schema change; removing or
  retyping a field requires `schema_version: 2`. Document this rule.
- **NFR-7 — Portability.** The schema must remain valid when built for Linux
  only; no platform-specific types leak into JSON (no `syscall.Stat_t`).

## Acceptance Criteria

1. `go build ./...`, `go vet ./...`, `go test -race ./...` pass.
2. `./scripts/check.sh --full` passes.
3. `go run ./cmd/swapaudit --json --no-update-check > /tmp/s.json` exits 0 and
   `/tmp/s.json` parses with `python3 -m json.tool`.
4. stdout contains no ANSI escape, no box glyph, no "Cleanable", and no update
   notice.
5. `--json` combined with each cleanup flag, `--dry-run`, `--yes`, and
   `--interactive` exits 2 with the exact message.
6. `--json --no-command` empties every `command` field while keeping the key.
7. Golden test with a frozen clock and fixed inputs produces an exact byte match
   at `internal/snapshot/testdata/snapshot_v1.json`.
8. Golden fixtures cover: zero items, one process with known idle, one cache,
   unknown idle (`null`), a cleanable mix, and a category with zero cleanable
   items.
9. `BuildSnapshot` is pure: a test calls it twice with identical inputs and
   compares bytes.
10. `items` ordering matches FR-6 for a shuffled input; `summary` is sorted by
    category.
11. `warnings` is `[]`, not `null`, in the success document.
12. The hard-failure fixture produces the exact error document and exit 1.
13. `docs/json-schema.md`, README, and `PrintUsage` document the mode, the
    ordering guarantee, nullability, and `--no-command`.
14. No file exceeds the NFR-2 ceiling.

## Evidence Requirements

`goals/json-snapshot-output/evidence.md` with a standalone `Verdict: PASS` and:

- `go test -race ./...` and `./scripts/check.sh --full` output;
- the exact golden-file diff command and result;
- a captured stdout/stderr pair for a success run proving stdout is pure JSON
  and stderr is empty;
- `python3 -m json.tool < /tmp/s.json` output;
- conflict-exit transcripts for every FR-2 row;
- a `--no-command` diff showing only `command` fields changed;
- a marshalling benchmark or timing measurement against NFR-3;
- `git diff --stat`;
- confirmation that `snapshot.go` and its tests stay within NFR-2.

## Dependencies & Execution Order

- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: 2
- **Shape**: ship
- **Tier**: roadmap

## Files to Touch

- internal/snapshot/snapshot.go
- internal/snapshot/snapshot_test.go
- internal/snapshot/testdata/snapshot_v1.json
- internal/app/app.go
- internal/app/app_test.go
- internal/app/display.go
- docs/json-schema.md
- README.md
- CHANGELOG.md
- goals/json-snapshot-output/evidence.md

## Persona & Audit Plan

- **Persona 5 — senior-fullstack-engineer:** schema typing, purity of
  `BuildSnapshot`, encoder configuration, exit codes.
- **Persona 25 — legal-compliance-counsel:** command-line privacy, `--no-command`
  behavior, and whether the document can be logged safely.
- **Persona 32 — mystery-auditor:** try to smuggle ANSI or a banner into stdout,
  combine `--json` with a destructive flag, produce nondeterministic ordering,
  emit `null` where the schema says array, and leak `-1` idle values.
- **Commands:** `sila persona get 5`, `sila persona get 25`,
  `sila persona get 32`, `sila hire assess 5,25,32`.
- **Artifacts:** `goals/json-snapshot-output/evidence.md` (or a sibling
  `persona-review.md`).

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation | Test |
| :--- | :---: | :---: | :--- | :--- |
| Human output contaminates stdout | Medium | High | Early dispatch before any render; stdout/stderr capture test | Stream-purity test |
| Schema drifts between docs and code | Medium | High | `schema_version` + golden file + docs table generated from the same constants | Golden test |
| Nondeterministic ordering | Medium | Medium | Canonical sort in `BuildSnapshot` | Shuffled-input test |
| `null` versus zero ambiguity | Medium | High | Explicit pointer types and the FR-5 table | Fixture per nullable field |
| Privacy leak in `command` | Medium | Medium | `--no-command` + docs | Diff test |
| JSON mode triggers the update check | Low | Medium | Implied `--no-update-check`; no term notice | stdout purity test |
| Encoder escapes HTML in paths | Low | Low | `SetEscapeHTML(false)` | Golden fixture with `&` |
| Large scan marshal cost | Low | Low | Single pass, no re-scan; benchmark | Timing evidence |

## Open Questions / Deferred Decisions

- Should `schema_version` be a string (`"1"`) or an integer? The goal uses an
  integer; consumers can compare numerically.
- Should `generated_at` use UTC (`Z`) instead of local offset? The goal uses
  local offset for human correlation; consumers should parse, not string-match.
- Should the document include the resolved `/tmp` patterns? Deferred; thresholds
  are included, patterns are not. Add in schema v2 if consumers need them.
- Should cleanup results gain a JSON shape? Deferred; the mode is read-only by
  design.
- Should `--json` accept `--sort`/`--limit` for a convenience projection?
  Deferred; the contract favors a stable full document.

## References

- **Shared Understanding & Fact Sheet**:
  [`goals/json-snapshot-output/facts.md`](facts.md)
- **Execution Plan**: [`goals/json-snapshot-output/plan.md`](plan.md)
- `internal/app/app.go` — flag parsing and render order.
- `internal/app/display.go` — human output that must be bypassed.
- `internal/model/model.go` — item fields and category values.
- `internal/proc/proc.go` — the source of every system and process number.
- `internal/version/version.go` — build metadata.
- `goals/live-sampler-rates/goal.md` — the dependent consumer of this schema.
- `goals/one-shot-action-picker/goal.md` — mutually exclusive mode.

## Done Condition

1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the
   `implemented` transition.
2. Implementation is committed with `(goals/json-snapshot-output/goal.md)` or
   `[goal: json-snapshot-output]` attribution under a `feat:`, `fix:`,
   `refactor:`, `perf:`, or `chore:` prefix.
3. Goal is certified (`goals/json-snapshot-output/certification.json` with
   `verified: true` or a signed `audit-receipt.json`) and any markdown evidence
   contains an explicit standalone `Verdict: PASS` declaration.
4. `sila goals --json` reports `status: "implemented"`,
   `verification_status: "verified"`, and `unverified_count: 0` for delivered
   goals, with a clean `git status`.
5. `go test -race ./...`, `./scripts/check.sh --full`, `sila facts verify`, and
   `sila gate` pass.
