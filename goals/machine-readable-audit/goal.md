# Goal: Add a Stable Machine-Readable Audit and Cleanup Contract

## Chain of Custody (Isnad)

- **Registered By**: UI review follow-up, 2026-10-19
- **Spec Audited By**: CommandCode, 2026-10-20 (acceptance criteria checked against tests and evidence)
- **Implemented By**: CommandCode, 2026-10-20 (see `evidence.md`)
- **Mystery Audited By**: Evidence certification, 2026-10-20 (see `certification.json`; independent mystery audit not run)

## Goal Description

The audit is currently human-only. There is no `--json`, no stable schema, and no documented exit-code or non-interactive contract. The review asks for `--json` and `--yes` for scripting; `--yes` already exists, and `--dry-run` already behaves as a modifier, but neither is locked by tests or documented as a contract.

This goal adds a versioned `--json` audit and cleanup result, defines the `--dry-run`/`--yes`/non-TTY matrix, assigns exit codes, and validates the schema with golden fixtures. It consumes the accounting model from Goal 1 and the group/identity model from Goal 4; it does not change classification or cleanup eligibility.

## Business Context

- A memory auditor is most useful when it can feed a timer, a dashboard, a CI check, or a shell prompt. The product's governance already mentions JSON schemas, but no user-facing JSON exists.
- The review explicitly asks for `--json` and `--yes` for scripting. Without a stable schema, users will scrape the human table, which breaks on every formatting change.
- A machine-readable reconciliation is also the best regression test for Goals 1–4: every number and group can be asserted exactly.
- This goal is lower priority than the correctness and safety goals. It should land after the accounting, cache, orphan, and layout contracts are stable.
- This goal must not add a network schema registry, a daemon, or telemetry. It is a local stdout contract.

## Verified Findings (2026-10-19, current tree)

### F1 — No user-facing JSON

- `internal/app/app.go:160-180` defines the flags; there is no `json` flag.
- `internal/app/display.go` writes human tables directly to `fmt.Printf`; there is no structured renderer.
- `grep -R '"json"' internal` only finds updater state and tests.
- `GOVERNANCE.md` mentions JSON schemas, but no `--json` flag or schema file exists.

### F2 — `--yes` exists; `--dry-run` is already a modifier

- `internal/app/app.go:170-171` defines `--dry-run` and `--yes`.
- `internal/app/app.go:229-234` passes them to `cleaner.ExecuteCleanup`.
- `internal/cleaner/cleaner.go:123-131`, `:216-224` skip prompts for `--dry-run`/`--yes`.
- The review's "make `--dry-run` a modifier" is already true; the gap is the lack of tests and a documented contract.
- `--yes` without a cleanup flag currently has no effect and no warning; `--dry-run` without a cleanup flag prints quick commands. Goal 3 adds warnings.

### F3 — Exit codes are not a contract

- `Run` returns `0` on success, `2` on flag errors (`internal/app/app.go:186-205`), and `0` for skipped cleanup.
- There is no code for a cleanup operational failure or a partial result.
- Scripts cannot distinguish "nothing to reclaim", "user declined", "some targets failed", and "invalid flags" from stdout alone.

### F4 — Non-TTY behavior is undefined

- Current `AskConfirmationReader` returns `false` on EOF (`internal/cleaner/cleaner.go:81-89`).
- A pipeline without `--yes` silently skips and exits 0.
- Goal 3 defines exit 2 for this case. This goal locks the JSON variant: `--json` cleanup requires `--yes` or `--dry-run`.

### F5 — No schema or golden fixture

- No `testdata/*.golden.json`, no `schema_version`, no `docs/json-schema.md`.
- No deterministic-output test; the audit's map iteration/sorting is not yet asserted at the JSON boundary.

## Dependencies & Execution Order

- **Mode**: Dependent (consumes `AuditReport`, group/identity fields, and the non-TTY contract)
- **Depends On**: `accounting-reconciliation`, `output-information-architecture`
- **Sequence**: 5
- **Shape**: ship
- **Tier**: roadmap
- **Blocks**: none

## Files to Touch

- `internal/report/json.go` (new) — versioned schema structs and encoder
- `internal/report/json_test.go` (new)
- `internal/report/accounting.go` — ensure all fields needed by JSON are exported/stable
- `internal/app/app.go` — `--json` flag, output routing, exit codes, cleanup guard
- `internal/app/app_test.go`
- `internal/app/display.go` — no JSON in display; route to `report.WriteJSON`
- `internal/cleaner/cleaner.go` — structured cleanup result (if live cleanup with `--json` is supported)
- `internal/cleaner/cleaner_test.go`
- `docs/json-schema.md` (new), `docs/metrics.md` (cross-reference)
- `README.md`, `CHANGELOG.md`
- `goals/machine-readable-audit/*` (this package)

## JSON Contract (over-specified)

### J1 — Top-level shape

`swapaudit --json` writes exactly one JSON object to stdout, followed by a newline. No ANSI, no banner, no table, no progress text on stdout. Diagnostics go to stderr.

```json
{
  "schema_version": 1,
  "generated_at": "2026-10-19T22:30:00Z",
  "host": {
    "kernel_release": "6.x.y",
    "uptime_seconds": 123456
  },
  "thresholds": {
    "max_age_seconds": 43200,
    "idle_threshold_seconds": 7200,
    "orphan_cache_age_seconds": 3600
  },
  "system": {
    "swap": {"total_bytes": 18321752064, "used_bytes": 8065630208, "free_bytes": 10256121856},
    "ram": {
      "total_bytes": 33549176832,
      "used_bytes": 17612824576,
      "available_bytes": 15936352256,
      "definition": "used = total - available"
    },
    "tmpfs": {
      "path": "/tmp",
      "total_bytes": 16774586368,
      "used_bytes": 5596966912,
      "free_bytes": 11177619456
    }
  },
  "attribution": {
    "swap": {
      "attributed_bytes": 3071897600,
      "unattributed_bytes": 4993732608,
      "overlap_bytes": 0,
      "reasons": [
        {"reason": "kernel-or-shmem", "bytes": 0, "count": 0}
      ]
    },
    "tmpfs": {
      "attributed_bytes": 4456448000,
      "unattributed_bytes": 1140518912,
      "overlap_bytes": 0,
      "reasons": [
        {"reason": "other-uid", "bytes": 800000000, "count": 3}
      ]
    }
  },
  "reclaim": {
    "tmpfs_bytes": 1181116006,
    "swap_bytes": 854065152,
    "upper_bound": true,
    "note": "tmpfs and swap pools may overlap; tmpfs pages may be swapped"
  },
  "population": {
    "raw_items": 804,
    "aggregated_shells": 66,
    "aggregated_rows": 1,
    "displayed": 12,
    "hidden_by_limit": 717,
    "hidden_by_size_filter": 55,
    "cleanable": 150,
    "protected": 654,
    "processes": 225,
    "processes_with_swap": 225,
    "agents": 12,
    "agents_zero_swap": 5,
    "agents_dormant": 5,
    "agents_orphaned": 0,
    "agents_idle_unknown": 2
  },
  "categories": [
    {
      "category": "dormant-agents",
      "pool": "process-swap",
      "entities": 5,
      "attributed_bytes": 854065152,
      "reclaimable_bytes": 854065152,
      "share_of_attributed": 1.0,
      "share_of_system": 0.106
    }
  ],
  "groups": [
    {
      "key": "TestGoalsStatsFlagConflicts*",
      "category": "abandoned-runs",
      "count": 62,
      "total_bytes": 39072051,
      "min_idle_seconds": 45000,
      "max_idle_seconds": 46800,
      "representative": {"name": "TestGoalsStatsFlagConflicts11102814295", "path": "/tmp/..."}
    }
  ],
  "items": [
    {
      "kind": "process",
      "id": "a1b2c3d4",
      "pid": 1430533,
      "start_ticks": 123456,
      "uid": 1000,
      "name": "agy",
      "path": "",
      "category": "dormant-agents",
      "cleanable": true,
      "swap_bytes": 610533376,
      "allocated_bytes": 0,
      "apparent_bytes": 0,
      "age_seconds": 122400,
      "idle_seconds": 13200,
      "idle_known": true,
      "open_handles": 0,
      "cwd_inside": false,
      "mapped_inside": false,
      "parent_pid": 938403,
      "parent_comm": "bash",
      "parent_alive": true,
      "orphan_reason": "parent-alive",
      "evidence": "idle=3h40m parent=alive",
      "signal": "SIGTERM"
    },
    {
      "kind": "cache",
      "id": "e5f6a7b8",
      "pid": 0,
      "start_ticks": 0,
      "uid": 1000,
      "name": "go-build3514075730",
      "path": "/tmp/go-build3514075730",
      "category": "orphaned-caches",
      "cleanable": false,
      "swap_bytes": 0,
      "allocated_bytes": 474480640,
      "apparent_bytes": 474480640,
      "age_seconds": 46560,
      "idle_seconds": 46560,
      "idle_known": true,
      "open_handles": 0,
      "cwd_inside": false,
      "mapped_inside": false,
      "cache_kind": "build",
      "evidence": "idle=12h56m handles=0 kind=build; use --clean-orphaned-caches"
    }
  ],
  "cleanup": null
}
```

### J2 — Field rules

- `schema_version` is an integer and starts at `1`. A breaking change bumps it.
- All byte fields are JSON integers (`int64`), never floats or strings.
- All duration fields are integer seconds; `age_seconds`/`idle_seconds` are non-negative; unknown idle is `null` plus `idle_known: false` (or 0 with the flag; choose `null` to avoid ambiguity). **Decision: use `null` for unknown.**
- Timestamps are RFC3339 UTC.
- `reasons` is a sorted array, not a map, for deterministic output.
- `categories` and `groups` are deterministically sorted: category order from Goal 1, groups by `total_bytes desc, key asc`.
- `items` is sorted by the active sort mode (`size` or `age`) then by `id`/`path` for determinism.
- Full names and full paths are always present; no ellipsis, no truncation.
- `cleanup` is `null` for an audit-only run; if cleanup flags are set, see J4.
- Unknown fields are allowed for forward compatibility; consumers must ignore them.
- The JSON must not include internal error messages unless they are part of a documented `errors` array. **Decision: add `"errors": []` only when non-empty; each entry has `code`, `message`, `target`.**

### J3 — `--json` flag semantics

- `swapaudit --json` runs the audit and prints the object. It never prompts.
- `--json` implies `--no-color`; `--ascii` is irrelevant.
- `--json` with `--check-updates` must not print a human update notice on stdout; if an update is available, include `"update": {"current": "...", "latest": "..."}` or write the notice to stderr. **Decision: `update` is an optional object; no human notice on stdout.**
- `--json` errors (invalid flags, permission failure, unsupported platform) are printed as a JSON object with `"errors"` and a non-zero exit, or as plain stderr before any stdout? **Decision: flag/usage errors are plain stderr + exit 2; runtime report errors are JSON + exit 1/3.** Keep it simple: usage errors before encoding are stderr + 2; encode/write failures are stderr + 1.
- `--json` is not allowed on `upgrade` in this goal; `swapaudit upgrade` remains human-only. `--json upgrade` exits 2 with a clear message.

### J4 — Cleanup JSON

To keep the contract small and safe:

- `--json --dry-run --clean-*` prints the audit object plus:
  ```json
  "cleanup": {
    "dry_run": true,
    "groups": [
      {"group": "Untouched Build Caches", "targets": 9, "bytes": 820000000, "would_prune": 9, "would_end": 0, "skipped": 0, "reasons": []}
    ],
    "summary": {"pruned": 9, "ended": 0, "skipped": 0, "reasons": []}
  }
  ```
- `--json --clean-*` without `--dry-run` requires `--yes`; otherwise exit 2 with a clear error. This prevents a JSON pipeline from hanging on a prompt.
- Live cleanup with `--json --yes` emits the same `cleanup` object after the actions complete. There is no streaming event protocol in v1; a future schema can add NDJSON.
- `cleanup` is `null` when no cleanup flags are set.
- Every action/skip reason uses the same strings as the human summary.

### J5 — Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Audit succeeded; cleanup (if requested) completed with no operational failures. Skips/declines are not failures. |
| `1` | Runtime/report failure: cannot read required `/proc`/`statfs`, JSON encode/write failure, or unexpected internal error. |
| `2` | Usage error: invalid flag/value, invalid combination, `--json` with `upgrade`, `--json` live cleanup without `--yes`, non-TTY cleanup without `--dry-run`/`--yes` (Goal 3). |
| `3` | Cleanup partial failure: at least one `remove-failed` or `kill-failed` occurred; the rest of the cleanup continued. |

- `--strict` is out of scope; code `3` is the operational-failure signal.
- Document the matrix in `docs/json-schema.md` and `PrintUsage`.

### J6 — `--dry-run` and `--yes` matrix

| Flags | Cleanup flags | TTY | Behavior | Exit |
| --- | --- | --- | --- | --- |
| `--dry-run` | yes | any | Preview; no prompt; no mutation | 0 |
| `--dry-run` | no | any | Warning; normal audit + quick commands | 0 |
| `--yes` | yes | any | Execute; no prompt | 0/3 |
| `--yes` | no | any | Warning; normal audit | 0 |
| neither | yes | yes | Prompt per group | 0 |
| neither | yes | no | Usage error | 2 |
| `--json --dry-run` | yes | any | JSON with `cleanup.dry_run=true` | 0 |
| `--json` + cleanup | yes | any | Requires `--yes` or `--dry-run` | 2 if missing |
| `--json --yes` | yes | any | Execute and emit `cleanup` | 0/3 |

- Flag order must not matter: `--dry-run --clean-all` and `--clean-all --dry-run` are equivalent.
- `--json --dry-run` must not read stdin.
- `--json --yes --clean-*` must never prompt.

## Requirements

### R1 — Schema types and encoder

- Implement `internal/report/json.go`:
  ```go
  type JSONReport struct { ... }
  func ToJSON(r AuditReport) JSONReport
  func WriteJSON(w io.Writer, r AuditReport) error
  ```
- Use explicit struct fields with `json` tags; do not marshal internal structs directly if their field names/units could drift.
- `WriteJSON` must be deterministic: sorted reasons/categories/groups/items, fixed field order, no maps at the top level.
- Nullable durations use `*int64` or `json.Number`? Use `*int64` with `omitempty` off so `null` is emitted for unknown.
- Write one object and a newline.

### R2 — Wire `--json` into `Run`

- Add `--json` to the flag set.
- If `--json` is set, skip all human rendering and call `report.WriteJSON(os.Stdout, r)`.
- Route diagnostics to stderr; stdout is JSON only.
- `--json upgrade` exits 2.
- `--json` with cleanup flags:
  - If `--dry-run`: build the cleanup plan without mutation and include it.
  - If `--yes`: execute and include the result.
  - Otherwise: exit 2.
- Add `--json` to `PrintUsage` and README.

### R3 — Cleanup result struct

- Add a structured `CleanupResult` to `internal/cleaner` (not just `CleanupStats`):
  ```go
  type CleanupResult struct {
      DryRun bool
      Groups []CleanupGroupResult
      Summary CleanupStats
  }
  type CleanupGroupResult struct {
      Group string
      Targets int
      Bytes int64
      WouldPrune int
      WouldEnd int
      Pruned int
      Ended int
      Skipped int
      Reasons []ReasonCount
  }
  ```
- `ExecuteCleanup` returns `CleanupResult`; the human renderer and JSON renderer consume the same result.
- Do not change the human output semantics; only add the structured return.
- Tests assert the JSON result equals the human summary counts.

### R4 — Exit codes

- Introduce named constants:
  ```go
  const (
      ExitOK = 0
      ExitRuntime = 1
      ExitUsage = 2
      ExitCleanupPartial = 3
  )
  ```
- `Run` returns the correct code for the J5 matrix.
- A cleanup with `remove-failed`/`kill-failed` returns 3, even if some targets succeeded.
- Usage errors return 2 before any mutation.
- Add tests for each code.

### R5 — Determinism and golden fixtures

- Add `internal/report/testdata/audit-v1.golden.json` with a fixture that covers:
  - Both pools, unattributed reasons, a group, a zero-swap agent, an in-use cache, an orphaned cache, a zero row.
- `TestJSONGolden` compares the encoded output after replacing `generated_at`/host-specific fields with fixed values.
- `TestJSONDeterministic` encodes the same report twice and compares bytes.
- `TestJSONFullNames` asserts no `...`/`…` in names/paths.
- `TestJSONNoANSI` asserts no `\x1b`.
- `TestJSONSchemaFields` unmarshals into a map and asserts required keys/types.

### R6 — Documentation

- `docs/json-schema.md`: the J1 schema, field units, nullability, sorting, exit codes, cleanup matrix, and a `jq` example.
- `README.md`: `--json` in the CLI table and an example pipeline.
- `CHANGELOG.md`: `Added` JSON, `Changed` exit codes if any, `Fixed` non-TTY behavior.
- `PrintUsage`: `--json`, exit codes, and the cleanup guard.

## Non-Goals

- No NDJSON event stream in v1; cleanup is a final object.
- No JSON schema registry, remote schema, or version negotiation.
- No `--json` for `upgrade`.
- No change to the human output except the flags/help/exit codes.
- No new classification, cache liveness, or agent visibility.
- No new dependency; `encoding/json` only.

## Acceptance Criteria

- **AC-01**: `go test -race ./...` and `go vet ./...` pass.
- **AC-02**: `swapaudit --json | jq -e '.schema_version == 1'` exits 0 (jq optional; a Go test is mandatory).
- **AC-03**: JSON contains all required top-level keys and no ANSI; stdout parses as a single object.
- **AC-04**: JSON totals tie exactly to the `AuditReport` and the human reconciliation line.
- **AC-05**: `--json --clean-all` without `--yes`/`--dry-run` exits 2 and emits no partial JSON.
- **AC-06**: `--json --dry-run --clean-all` exits 0, emits `cleanup.dry_run=true`, and mutates nothing.
- **AC-07**: `--json --yes --clean-all` exits 0/3 and emits the same counts as the human summary.
- **AC-08**: `--json upgrade` exits 2.
- **AC-09**: Golden and deterministic tests pass; full names/paths survive.
- **AC-10**: `--dry-run` works in any flag order; `--yes` without cleanup warns (Goal 3) and exits 0.
- **AC-11**: Exit codes 0/1/2/3 are tested for representative cases.
- **AC-12**: `docs/json-schema.md` exists and defines every field/unit/nullability.
- **AC-13**: `scripts/check.sh --full` passes; `go.mod` unchanged.

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

## Test Matrix

| Test | File | Proves |
| --- | --- | --- |
| `TestJSONGolden` | `internal/report/json_test.go` | Full schema fixture |
| `TestJSONDeterministic` | same | Stable bytes/order |
| `TestJSONFullNames` | same | No truncation |
| `TestJSONNoANSI` | same | Clean stdout |
| `TestJSONSchemaFields` | same | Required keys/types |
| `TestJSONReconciliation` | same | JSON totals tie to header |
| `TestJSONCleanupRequiresYes` | `internal/app/app_test.go` | Exit 2, no output |
| `TestJSONDryRun` | same | `cleanup.dry_run=true`, no mutation |
| `TestJSONYesCleanup` | same | Cleanup object and exit 0/3 |
| `TestJSONUpgradeRejected` | same | Exit 2 |
| `TestExitCodes` | same | 0/1/2/3 |
| `TestDryRunAnyOrder` | same | Flag-order independence |
| `TestYesWithoutCleanup` | same | Warning, exit 0 |
| `TestCleanupResultMatchesSummary` | `internal/cleaner/cleaner_test.go` | Structured counts equal human counts |

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Schema churn breaks consumers | `schema_version`; golden tests; document breaking-change policy |
| JSON is slow or memory-heavy on 100k items | Stream with `json.Encoder`, cap output? The audit already builds items; measure with a benchmark |
| Nullable durations are awkward | Use `*int64` + `idle_known`; document; test |
| Cleanup JSON encourages unsafe automation | Require `--dry-run` or `--yes`; default live cleanup is opt-in; exit 3 for partial failure |
| Human and JSON counts diverge | Both consume `AuditReport`/`CleanupResult`; add equality tests |
| `jq` is not installed in CI | The Go schema tests are mandatory; shell `jq` checks are optional |
| `--json` mixes update notices into stdout | Route notices to stderr or the optional `update` object |

## Rollback

- Revert the implementation commit. The human CLI remains unchanged.
- If the cleanup JSON is too complex, ship `--json` audit only and defer cleanup JSON; keep the exit-code matrix and the non-TTY guard.
- Do not scrape the human output as a substitute; fix the schema instead.

## References

- UI review screenshot and claim list, 2026-10-19.
- Verification report: `docs/reviews/2026-10-19-swapaudit-ui-review.md`.
- `internal/app/app.go:160-205`, `:229-250` — flags, error returns, cleanup wiring.
- `internal/app/display.go` — human renderer; no JSON.
- `internal/cleaner/cleaner.go:81-89`, `:123-131`, `:216-224`, `:376-410` — prompts and summary.
- `internal/report/accounting.go` (Goal 1) — the report fields.
- `internal/triage/group.go` (Goal 4) — the group model.
- `docs/metrics.md` (Goal 1) — metric definitions reused by the schema.
