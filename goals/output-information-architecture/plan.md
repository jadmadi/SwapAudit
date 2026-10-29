# Plan: Redesign Audit Tables for Identity, Grouping, Reconciliation, and Layout

## Execution Steps

### Phase 1 — Baseline and contract freeze

- [x] Capture the current output at 100 columns and a PTY at 80/120 in `goals/output-information-architecture/evidence.md`.
- [x] Mark the duplicate rows between Actionable and Top Occupants; record their names, sizes, and categories.
- [x] Record the truncated names and their full paths; list the distinguishing suffixes.
- [x] Count repeated patterns (`TestGoalsStatsFlagConflicts*`, `go-build*`, `sila_mcp_test_*`).
- [x] Run `go test -race ./...` and `go vet ./...`; record the baseline.
- [x] Add the I1–I7 contract to `docs/metrics.md` or a new `docs/output-layout.md` before implementation.

### Phase 2 — Middle truncation and short IDs

- [x] Implement `TruncateMiddle` and `TruncateMiddleANSI` in `internal/term/term.go`.
- [x] Add property tests for widths 0..80, digits suffixes, ANSI, Unicode, and exact-width strings.
- [x] Add `ShortID(item)` to `internal/model` (SHA-256 of path or `pid:startticks`, first 8 hex).
- [x] Add `TestShortID`.
- [x] Replace `term.Truncate` with `TruncateMiddle` in `display.go` and `cleaner.go`.
- [x] Add `TestCleanupMiddleTruncation` with five shared-prefix paths.
- [x] Run `go test -race ./internal/term ./internal/model ./internal/cleaner`.

### Phase 3 — Grouping

- [x] Create `internal/triage/group.go` with `Group`, `NormalizeGroupKey`, `GroupItems`, `ExpandGroup`.
- [x] Implement the normalization rules from I2.
- [x] Add `TestNormalizeGroupKey`, `TestGroupItems`, `TestExpandGroup`, `TestGroupCategoryTies`.
- [x] Wire grouping into the actionable and protected tables.
- [x] Add `--no-group`, `--expand`, `--collapse`; wire them through `ViewOptions`.
- [x] Add a group row renderer with `×N`, total, idle range, and action.
- [x] Run `go test -race ./internal/triage ./internal/app`.

### Phase 4 — Consolidate tables and verdict

- [x] Choose Layout A or B; document the choice in the goal evidence.
- [x] Remove cleanable rows from the protected table (Layout A) or the duplicate section (Layout B).
- [x] Add `TestNoDuplicateRows`.
- [x] Add `FormatVerdict(report)` and the golden tests.
- [x] Add the pressure legend and threshold labels.
- [x] Add `TestThresholdLabels`, `TestPressureLegend`.
- [x] Run `go test -race ./internal/app`.

### Phase 5 — Layout contract

- [x] Add `TableWidth(cols)` and a `Layout` helper; remove `w-82`, `w-68`, `w-61` magic offsets.
- [x] Trim trailing whitespace in `RenderTable`.
- [x] Add `TestTableWidthInvariant` at 80/100/120, ASCII/Unicode, and with `NO_COLOR`.
- [x] Add `TestNoTrailingWhitespace`.
- [x] Replace the PID column with the ID/PID policy from I6.
- [x] Add `--wide`/`--no-truncate` and `TestWideMode`.
- [x] Run `go test -race ./internal/term ./internal/app`.

### Phase 6 — Cleanup output and docs

- [x] Use middle truncation and short IDs in cleanup output.
- [x] Add grouped cleanup footers for repeated patterns.
- [x] Update `PrintUsage`, `PrintQuickCommands`, README, `CHANGELOG.md`, and `docs/output-layout.md`.
- [x] Run `gofmt -l .`, `go vet ./...`, `go test -race ./...`.

### Phase 7 — Full verification and evidence

- [x] Capture 80/100/120 output with a PTY harness or a test-only width override.
- [x] Capture `--ascii`, `--no-color`, `--wide`, `--no-group`, `--all`.
- [x] Confirm no duplicate rows.
- [x] Confirm group totals equal category totals.
- [x] Confirm every line's visible width.
- [x] Run `./scripts/check.sh --full`.
- [x] Record outputs in `evidence.md`.

### Phase 8 — Attribution commit

- [x] Commit with `feat:` or `refactor:` and a subject containing `(goals/output-information-architecture/goal.md)`.
- [x] Run `sila goals --project=/home/jad/scripts/active/swapaudit`.
- [x] Do not push to a remote unless the user explicitly authorizes it.

## Tests to Add

| Test | File | What it proves |
| --- | --- | --- |
| `TestTruncateMiddleWidths` | `internal/term/term_test.go` | Width, suffix preservation |
| `TestTruncateMiddleANSI` | same | ANSI-aware width |
| `TestShortID` | `internal/model/model_test.go` | Deterministic ID |
| `TestNormalizeGroupKey` | `internal/triage/group_test.go` | Pattern normalization |
| `TestGroupItems` | same | Count/size/idle range |
| `TestExpandGroup` | same | Member expansion |
| `TestGroupCategoryTies` | same | Group sums tie to categories |
| `TestNoDuplicateRows` | `internal/app/display_test.go` | Disjoint tables |
| `TestVerdict` | same | Empty/one/two/unattributed |
| `TestThresholdLabels` | same | Thresholds in headers/legend |
| `TestPressureLegend` | same | 50/80 constants, NO_COLOR |
| `TestTableWidthInvariant` | `internal/term/term_test.go` | 80/100/120, ASCII/Unicode |
| `TestNoTrailingWhitespace` | same | Clean output |
| `TestWideMode` | `internal/app/display_test.go` | Full names |
| `TestNoGroupMode` | same | Expansion |
| `TestCleanupMiddleTruncation` | `internal/cleaner/cleaner_test.go` | Shared-prefix suffixes |
| `TestCleanupGroupSummary` | same | Grouped cleanup footer |

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -race -run 'Test(TruncateMiddle|ShortID|Group|Verdict|TableWidth|NoDuplicate|Threshold|Pressure|Wide)' ./internal/term ./internal/triage ./internal/app ./internal/cleaner
./swapaudit --no-color --ascii --limit 12 > /tmp/ui-100.txt
./swapaudit --no-color --ascii --no-group --limit 20 > /tmp/ui-nogroup.txt
./swapaudit --no-color --ascii --wide --limit 5 > /tmp/ui-wide.txt
grep -c "TestGoalsStatsFlagConflicts\*" /tmp/ui-100.txt || true
./scripts/check.sh --full
```

Width invariants at 80/100/120 are enforced by Go tests using `term.SetTermWidth`; do not rely on shell `COLUMNS`.

## Rollback

- Revert the implementation commit. No persistent state.
- If grouping is too disruptive, keep middle truncation, duplicate removal, verdict, and width contract; defer grouping.
- Do not restore right-truncated names or duplicate rows as the default.
