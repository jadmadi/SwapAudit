# Plan: Add a Stable Machine-Readable JSON Snapshot

## Execution Steps

- [x] **Phase 1 — Schema types and purity contract**
  - [x] Create the neutral `internal/snapshot` package (not `internal/app`) so
        `internal/live` can import the schema without an `app -> live -> app`
        cycle.
  - [x] Define `Snapshot`, `Tool`, `Thresholds`, `System`, `Totals`,
        `SummaryEntry`, and `Item` structs with explicit JSON tags and pointer
        types for every nullable field.
  - [x] Add a `SchemaVersion = 1` constant used by both the builder and the
        documentation test.
  - [x] Implement `BuildSnapshot(now, uptime, sys, items, maxAge,
        idleThreshold, noCommand)` with no clock, environment, filesystem, or
        network access.
  - [x] Initialize `Summary`, `Items`, and `Warnings` to empty slices so they
        marshal as `[]`, never `null`.
  - [x] Convert `time.Duration` to whole seconds; normalize `-1` idle to `null`.
  - [x] Convert an empty cache path to `null` and process `pid`/`start_ticks`
        to `null` for filesystem items.
  - [x] Compute `effective_size_bytes` from `model.ItemSize`.
  - [x] Unit-test every nullability rule in the FR-5 table.

- [x] **Phase 2 — Aggregation and canonical ordering**
  - [x] Build `summary` as one entry per category present, sorted by category
        ascending, using `SwapItem.Count`, `SwapBytes`, `SizeBytes`, and
        `Cleanable`.
  - [x] Build `totals` from raw items: item count, process swap, tmpfs cache
        bytes, cleanable effective bytes, and the swap/tmpfs split.
  - [x] Sort `items` by category ascending, effective size descending, name
        ascending, PID ascending with `null` last.
  - [x] Add a shuffled-input test that asserts the exact ordering.
  - [x] Add an empty-scan test asserting valid zero totals and empty arrays.

- [x] **Phase 3 — Encoder and stdout purity**
  - [x] Add a `writeSnapshot(w io.Writer, s Snapshot) error` helper using
        `json.Encoder` with `SetEscapeHTML(false)` and `SetIndent("", "  ")`.
  - [x] Ensure exactly one trailing newline and no other bytes.
  - [x] Marshal to a buffer before writing so an encode failure cannot emit a
        partial document.
  - [x] Add a fixture containing `&`, `<`, `>`, quotes, and a non-ASCII path and
        assert no HTML escaping.
  - [x] Add a stdout/stderr capture test asserting stdout parses and stderr is
        empty on success.

- [x] **Phase 4 — Application integration**
  - [x] Add `--json` and `--no-command` flags.
  - [x] Add a pure conflict validator covering every FR-2 combination with the
        exact messages; run it before `term.InitTerminal` and before the scan.
  - [x] Skip the update-check goroutine entirely when `--json` is set,
        regardless of `--no-update-check`.
  - [x] Dispatch to the JSON path before any human renderer.
  - [x] Reuse the `items` slice from the single `CollectSwapItems` call; do not
        scan twice.
  - [x] Add the `/proc/self/status` preflight and the exact hard-failure
        document with exit 1.
  - [x] Ensure all diagnostics go to stderr.
  - [x] Add an executor spy test proving `--json` never reaches the cleaner.
  - [x] Keep `TestRunDefaultExecution` and other existing tests passing.

- [x] **Phase 5 — Golden fixtures and tests**
  - [x] Create `internal/snapshot/testdata/snapshot_v1.json` from a frozen clock and
        fixed system/item inputs.
  - [x] Add a `-update` flag to regenerate the golden file, following standard
        Go golden-test practice with no third-party helpers.
  - [x] Add fixtures for: zero items; one process with known idle; one cache;
        unknown idle; a cleanable mix; a category with zero cleanable items.
  - [x] Add a round-trip test: encode, decode into `map[string]any`, and assert
        required keys and types.
  - [x] Add a determinism test calling `BuildSnapshot` twice with identical
        inputs.
  - [x] Add a benchmark for marshalling 500 items and record it in the evidence.

- [x] **Phase 6 — Documentation**
  - [x] Write `docs/json-schema.md` with the field table, nullability rules,
        ordering guarantee, `effective_size_bytes`, `idle_seconds: null`,
        `--no-command`, two consumer recipes, and a complete example.
  - [x] Add `--json` and `--no-command` to `PrintUsage` with one example.
  - [x] Add a README section linking to the schema document.
  - [x] Add a CHANGELOG entry under the unreleased heading.
  - [x] Document the additive-change rule and the v2 break rule.

- [x] **Phase 7 — Verification and evidence**
  - [x] Run `gofmt -l .` and fix formatting.
  - [x] Run `go vet ./...`.
  - [x] Run `go test -race ./internal/snapshot ./internal/app`.
  - [x] Run `go test -race ./...`.
  - [x] Run `./scripts/check.sh --pre-commit`.
  - [x] Run `./scripts/check.sh --full`.
  - [x] Capture a success run and verify stdout is pure JSON with
        `python3 -m json.tool`.
  - [x] Capture every conflict combination and verify exit 2 and the exact
        message.
  - [x] Capture a `--no-command` diff showing only `command` fields changed.
  - [x] Measure the 500-item marshal benchmark against NFR-3.
  - [x] Verify `internal/snapshot/snapshot.go` and its tests stay within the
        NFR-2 ceiling.
  - [x] Write `goals/json-snapshot-output/evidence.md` with a standalone
        `Verdict: PASS` and the required outputs.
  - [x] Check every completed task in this plan.
  - [x] Commit with
        `feat(json): add a versioned read-only JSON snapshot (goals/json-snapshot-output/goal.md)`.
  - [x] Certify with
        `sila goals certify json-snapshot-output --evidence=goals/json-snapshot-output/evidence.md`.
  - [x] Run `sila goals` and verify `unverified_count: 0` with a clean
        `git status`.
