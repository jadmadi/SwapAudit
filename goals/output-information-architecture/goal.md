# Goal: Redesign Audit Tables for Identity, Grouping, Reconciliation, and Layout

## Chain of Custody (Isnad)

- **Registered By**: UI review follow-up, 2026-10-19
- **Spec Audited By**: CommandCode, 2026-10-20 (acceptance criteria checked against tests and evidence)
- **Implemented By**: CommandCode, 2026-10-20 (see `evidence.md`)
- **Mystery Audited By**: Evidence certification, 2026-10-20 (see `certification.json`; independent mystery audit not run)

## Goal Description

The audit is now numerically honest after Goals 1–3, but the screen is still hard to act on:

1. Names are right-truncated to the column width, so the distinguishing suffix is lost (`sila-cli-test-p...`, `TestMCPServerPl...`). Repeated test names become indistinguishable.
2. The Actionable and Top Occupants tables repeat the same rows (agy, `go-build3514075730`, etc.), so the screen shows the same byte twice in two different rankings.
3. There is no single verdict line: the user must scan three tables and a category breakdown to decide what to do.
4. Thresholds live only in the quick-command footer, not in the table headers that display idle/age values.
5. The `PID` column is mostly `-` because cache rows have no PID; the column spends width on a value that is usually absent.
6. There is no grouping of repeated cache/test patterns (`TestGoalsStatsFlagConflicts*`, `go-build*`, `sila_mcp_test_*`).
7. There is no test that every panel uses the same visible width; the screenshot shows inconsistent right edges.

This goal makes identity stable, groups repeated patterns, removes duplicate rows, adds a one-line verdict, labels thresholds in-place, and enforces a single width contract. It consumes the accounting model from Goals 1–2 and the agent evidence from Goal 3; it does not change classification or cleanup eligibility.

## Business Context

- The UI review's top priority after correctness is actionability: the screen must answer "what can I safely reclaim now?" in one glance.
- The current output shows 150 cleanable candidates but only 12 rows; the 12 rows are dominated by 6 large items and 6 nearly identical test directories. Grouping and middle-truncation make the list scannable.
- Duplicate rows waste the terminal and make the "Top Occupants" ranking look like a second reclaim list.
- A verdict line is the product's answer; the README already frames the tool as an answer, not a data dump.
- The layout contract prevents regressions at 80/100/120 columns and with `--ascii`.
- This goal should land before `machine-readable-audit` because the JSON schema reuses the group/identity model.

## Verified Findings (2026-10-19, current tree)

### F1 — Right truncation loses identity

- `internal/term/term.go:296-356` appends the ellipsis at the end: `Truncate(s, w)` keeps the prefix and drops the suffix.
- `internal/app/display.go:135` and `:275` call `term.Truncate(name, targetWidth)`.
- Live output:
  ```text
  sila-cli-test-p...
  TestMCPServerPl...
  TestMCPListGoal...
  TestSilenceCryp...
  TestRecentActiv...
  ```
- The unique instance digits are at the end; the screen shows only the common prefix.
- `internal/cleaner/cleaner.go:173` and `:272` use the same truncation for cleanup targets, so the destructive preview has the same ambiguity.

### F2 — The two tables duplicate rows

- `PrintActionableSection` selects all `Cleanable` items and sorts by size (`internal/app/display.go:55-82`).
- `PrintMainTable` selects all items, filters small protected items, sorts by size/age, and limits (`internal/app/display.go:179-306`).
- A cleanable item above 10 MB appears in both tables.
- Live output: `agy 582.3 MB`, `go-build3514075730 452.5 MB`, `sila-cli-test-profile-... 344.4 MB`, `go-build879996547 329.4 MB` appear in both.
- This doubles the visual weight of the largest cleanable rows and makes the Top table's "hidden" count harder to interpret.

### F3 — No verdict line

- `PrintHeader` prints only the three meters (`internal/app/display.go:17-51`).
- The user must combine the meters, the Actionable header, the category table, and the quick commands mentally.
- The README's answer is "what can I safely reclaim now?"; the screen does not state it in one line.

### F4 — Thresholds are only in the footer

- The actionable table header says `Idle` (`internal/app/display.go:100`), not `Idle > 12h` or `Idle > 2h`.
- The main table header says `Age`/`Idle` (`:225-226`).
- The quick commands repeat `(> 12h untouched)` and `(> 2h idle)` (`:448-450`).
- A user reading only the table cannot know which threshold produced `dormant-agents` vs `untouched-caches`.

### F5 — PID column is mostly empty

- Cache items have `PID == 0` (`internal/cache/cache.go:152-164` does not set PID).
- The display prints `-` for PID 0 (`internal/app/display.go:129-133`, `:261-264`).
- Shell aggregation sets `PID = 0` (`internal/triage/triage.go:88`), also `-`.
- The column consumes 9 characters on every row while carrying information for only process rows.

### F6 — No pattern grouping

- `triage.AggregateShells` groups only small bash/sh/zsh processes (`internal/triage/triage.go:56-92`).
- Cache rows are never grouped. A host with 62 `TestGoalsStatsFlagConflicts*` directories shows 62 rows (or 12 + hidden).
- The cleanup output also lists each target (`internal/cleaner/cleaner.go:137-190`), with only a `... and N smaller targets` footer.

### F7 — Width contract is implicit

- `PrintBoxHeader` uses `term.TermWidth()` (`internal/term/term.go:509-540`).
- Tables use different offsets: `w-82`, `w-68`, `w-61` (`internal/app/display.go:91`, `:216`, `:333`).
- At 100 columns the current capture produces 100-column lines for all panels; the formulas happen to agree. There is no test that this holds at 80/100/120, with ANSI, or with `--ascii`.
- `RenderTable` pads the last column with spaces (`internal/term/term.go:494-504`), leaving trailing whitespace.
- The screenshot's differing right borders are not reproduced in the 100-column capture; the invariant test will tell us whether it is a width bug or a rendering artifact. Either way, the contract must be explicit.

### F8 — Pressure colors and legend are not explained

- `RenderMeterBar` uses green <50%, yellow <80%, red otherwise (`internal/term/term.go:400-416`).
- There is no legend; a color-blind or `NO_COLOR` user sees only the bar shape.
- The threshold constants are inline magic numbers.

## Dependencies & Execution Order

- **Mode**: Dependent (needs the final `AuditReport` fields, cache categories/evidence, and agent population)
- **Depends On**: `accounting-reconciliation`, `cache-liveness-classification`, `orphan-visibility-safety`
- **Sequence**: 4
- **Shape**: ship
- **Tier**: immediate
- **Blocks**: `machine-readable-audit`

## Files to Touch

- `internal/term/term.go` — `TruncateMiddle`, width helpers, pressure constants, table layout helpers
- `internal/term/term_test.go`
- `internal/model/model.go` — `GroupKey`, `GroupCount`, `ShortID`, `DisplayName` (or a view model)
- `internal/triage/group.go` (new) — pattern normalization and grouping
- `internal/triage/group_test.go` (new)
- `internal/app/display.go` — verdict, grouped tables, duplicate removal, thresholds, PID column, layout
- `internal/app/display_test.go`
- `internal/app/app.go` — pass a `ViewOptions` (group/expand/width) and report
- `internal/app/app_test.go`
- `internal/cleaner/cleaner.go` — middle-truncated paths in cleanup output
- `internal/cleaner/cleaner_test.go`
- `README.md`, `CHANGELOG.md`, `docs/metrics.md` (cross-reference)
- `goals/output-information-architecture/*` (this package)

## Information Architecture Contract (over-specified)

### I1 — Identity

```go
// internal/term
func TruncateMiddle(s string, w int) string
func TruncateMiddleANSI(s string, w int) string // if ANSI styling is present
```

Rules:

- If `StrWidth(s) <= w`, return `s` unchanged.
- Otherwise preserve the first `head` cells and the last `tail` cells, with a single ellipsis between. `head + ellipsisWidth + tail == w`; if `w <= ellipsisWidth`, return the ellipsis or a one-cell prefix.
- Prefer a suffix-preserving split when the name contains a digit/hash suffix:
  - `sila-cli-test-profile-1784434295` at width 24 → `sila-cli-t…1784434295` (suffix preserved).
  - `TestGoalsStatsFlagConflicts11102814295` at width 24 → `TestGoals…11102814295`.
- Never leave a trailing space before the ellipsis; strip trailing whitespace from the head.
- Never return a string with a dangling ellipsis if the original fits.
- ANSI-aware: preserve escape sequences, append `\x1b[0m` if any styling was opened, and do not count escapes as width.
- Property tests for widths 0..80, ASCII/Unicode/wide glyphs, ANSI, names with and without digits, and strings that are exactly the width.

Add a `ShortID` to each item for disambiguation when even the suffix is not unique:

```go
func ShortID(item model.SwapItem) string // first 8 hex of SHA-256(path or pid:startticks)
```

- Display the short ID in a narrow `ID` column only when `--wide` is false and the name was truncated; or append `[a1b2c3d4]` to the name in `--all`/`--explain` output.
- `--json` always carries the full name/path and the short ID (Goal 5).
- Add `--wide`/`--no-truncate` to show full names; the table may exceed the terminal width only with `--no-truncate` (documented).

### I2 — Grouping

```go
// internal/triage/group.go
type Group struct {
    Key         string
    Category    model.Category
    Count       int
    TotalBytes  int64
    MinIdle     time.Duration
    MaxIdle     time.Duration
    Representative model.SwapItem
    Members     []model.SwapItem
}

func NormalizeGroupKey(name string) string
func GroupItems(items []model.SwapItem, enabled bool) []Group
func ExpandGroup(g Group) []model.SwapItem
```

Normalization rules:

- Strip a trailing run of digits/hash/UUID: `-?\d{4,}$`, `-[0-9a-f]{8,}$`, `_[0-9]{4,}$`, `\d{6,}$`.
- Strip a trailing `-<pid>`/`.<pid>` when the prefix is non-empty.
- Preserve the category and pool in the key; never group across categories.
- Only group when `Count >= 2`. A single item is a group of one internally but renders as an individual row.
- `go-build123456` and `go-build987654` → key `go-build*`; `TestGoalsStatsFlagConflicts11102814295` → key `TestGoalsStatsFlagConflicts*`; `sila_mcp_test_426715724` → key `sila_mcp_test_*`.
- Do not group `test` with `testfoo` if the normalized prefixes differ.
- Do not group different kinds (`CacheBuild` vs `CacheRun` vs `CacheGeneric`).
- Group totals must equal the sum of members and must tie to the category summary (Goal 1). Add a test that `Σ groups == Σ categories` for every fixture.

Display:

- A group row shows: `Key ×N`, category, total size, idle range (e.g., `1h–11h`), and action (`Prune group`).
- The representative row is the largest member; the row is still actionable as a group.
- `--all` or `--no-group` expands groups into individual rows. `--expand <key>` can expand one group; `--collapse` restores.
- The actionable table should show groups first, then individual large items that do not belong to a group.
- The main/protected table uses the same grouping; a group of protected items is still protected.

### I3 — Table consolidation

Choose one of these two equivalent layouts and implement it consistently:

**Layout A (preferred):**
- `Actionable Reclaim Candidates`: only `Cleanable` items/groups, ranked by reclaimable bytes. Columns: `Name/Group`, `Category`, `Reclaim`, `Idle`, `Action`.
- `Protected Occupants`: only `!Cleanable` items/groups, ranked by size. Columns: `Name/Group`, `Category`, `Size`, `Age`, `Idle`.
- `Unattributed` rows are in the category summary (Goal 1), not repeated in both tables.

**Layout B:**
- One `Reclaim & Occupancy` table with a `Section` column (`ACTION`, `PROTECTED`) and a sort that puts actionable rows first. The `Action` column is empty for protected rows.

Whichever layout is chosen:

- No item or group appears in both tables/sections.
- A note under each table states how many cleanable rows were omitted if a cleanable item would otherwise be the largest occupant.
- The `Top Memory & Cache Occupants` title changes to `Protected Occupants` (Layout A) or `Reclaim & Occupancy` (Layout B).
- The current duplicate rows (agy, go-build3514075730, sila-cli-test-profile, go-build879996547) must appear exactly once in a live run.
- Add a test that concatenates all rendered data rows and asserts no duplicate (name, PID/path, category).

### I4 — Verdict

Add a `Verdict` function in `internal/app` or `internal/report`:

```go
func FormatVerdict(r report.AuditReport) string
```

Required shape (one line when the terminal is wide enough, wrapped with a hanging indent otherwise):

```text
Verdict: Swap 44.0% used · 1.9 GB reclaimable (1.1 GB tmpfs + 815 MB swap; upper bound) · biggest win: agy PID 1430533 (582 MB) · thresholds: cache 12h, agent 2h, orphan-cache 1h
```

- If no cleanable items: `Verdict: Swap 44.0% used · nothing safe to reclaim · 4.6 GB swap and 1.0 GB /tmp unattributed`.
- If only one pool is cleanable, omit the zero pool.
- If `Unattributed` is zero for a pool, omit the residual note for that pool.
- Color: green/amber/red for the swap percentage using the same constants as the meter bar; bold the biggest win; `NO_COLOR` removes styling but keeps the labels.
- The verdict is derived from `AuditReport`; no display-time recomputation.
- `--no-verdict` is not needed; a `--quiet` mode is out of scope.

### I5 — Threshold labels and pressure legend

- Table header labels include the threshold that produced the row:
  - `Idle > 12h` for cache stale rows.
  - `Idle > 2h` for dormant agents.
  - `Idle > 1h` for orphaned caches.
- If a table mixes categories with different thresholds, add a `Threshold` column or a legend line directly above the table:
  `Thresholds: untouched > 12h · dormant > 2h · orphan-cache > 1h`.
- Add a pressure legend under the header:
  `Pressure: green <50% · amber 50–79% · red ≥80%`.
- Export the constants:
  ```go
  const (
      PressureAmber = 50.0
      PressureRed   = 80.0
  )
  ```
- Tests at 49.9/50/79.9/80/100 and with `NO_COLOR`/`--ascii`.

### I6 — Layout contract

- Add `TableWidth(cols []model.Col) int` and `Layout` helpers; remove magic offsets `w-82`, `w-68`, `w-61` from `display.go`.
- Every rendered line must satisfy `term.StrWidth(line) == term.TermWidth()` for the header, verdict, table header, separator, data rows, and population lines, at widths 80, 100, and 120. Footers may be shorter but must not exceed the width.
- No trailing whitespace on any line. `RenderTable` must trim the final column's padding; if a right border is used, it must be the last visible character.
- Numeric columns right-aligned; text columns left-aligned; the last column may be left-aligned or trimmed.
- The PID column becomes `ID`:
  - Process rows: `PID` + short ID if truncated.
  - Cache rows: short ID only; no `-`.
  - Aggregated rows: `×N` in the name, ID blank or a group short ID.
  - If the terminal is narrow, drop the ID column before dropping the name.
- Add `--wide` (full names, no truncation) and `--no-group` (individual rows). Document that `--wide` may exceed the terminal width.
- Add golden tests for 80/100/120, Unicode/ASCII, `NO_COLOR`, and `--wide`.

### I7 — Cleanup output identity

- `PruneTmpTargets` and `KillProcessTargets` use `TruncateMiddle` for paths/names.
- The cleanup preview must preserve the unique suffix and, when a path is still ambiguous, append the short ID.
- The cleanup footer uses a group summary when the targets are grouped (e.g., `TestGoalsStatsFlagConflicts* ×62`), plus the existing per-target lines for the largest items.
- Add a test with 5 paths sharing a long prefix and distinct suffixes; assert all suffixes are visible.

## Requirements

### R1 — Middle truncation and short IDs

- Implement I1 in `term.go`.
- Add unit/property tests.
- Use `TruncateMiddle` in display and cleaner.
- Add `--wide`/`--no-truncate`.

### R2 — Grouping

- Implement I2 in `internal/triage/group.go`.
- Add normalization/grouping/expansion tests.
- Wire `GroupItems` into the actionable and protected tables.
- Add `--no-group`, `--expand`, `--collapse`.
- Ensure group totals tie to the category summary.

### R3 — Consolidate tables

- Implement Layout A or B consistently.
- Remove duplicate rows; add a duplicate assertion test.
- Update the table titles and the `Population` lines from Goal 1.
- Keep the actionable limit configurable and the `--all` expansion semantics.

### R4 — Verdict

- Implement `FormatVerdict`.
- Add golden tests for empty, one-pool, two-pool, unattributed, and narrow-terminal cases.
- Add the pressure legend and threshold labels.

### R5 — Layout contract

- Implement `TableWidth`/`Layout`.
- Remove hardcoded offsets.
- Trim trailing whitespace.
- Add width golden tests at 80/100/120.
- Ensure `--ascii` and Unicode both satisfy the width invariant.

### R6 — Cleanup identity

- Use `TruncateMiddle` and short IDs in cleanup output.
- Add a shared-prefix test.

### R7 — Documentation

- Update `README.md` with the verdict, grouping, `--wide`, `--no-group`, and table layout.
- Update `PrintUsage` and `PrintQuickCommands`.
- Update `CHANGELOG.md`.
- Add a `docs/metrics.md` cross-reference for the verdict's numbers.

## Non-Goals

- No new classification or cleanup eligibility; Goals 1–3 own that.
- No JSON (Goal 5); the group/identity model should be reusable.
- No GUI/TUI, no pagination, no interactive expand/collapse.
- No color palette redesign beyond the existing pressure thresholds and legend.
- No change to the terminal width clamp (80..120) in this goal.

## Acceptance Criteria

- **AC-01**: `go test -race ./...` and `go vet ./...` pass.
- **AC-02**: Every truncated name preserves its unique suffix; a shared-prefix fixture shows distinct suffixes for all 5 items.
- **AC-03**: No data row appears in both the actionable and protected tables; the live duplicate rows appear once.
- **AC-04**: `TestGoalsStatsFlagConflicts*` is rendered as one group row with count, total, idle range, and action; `--no-group` expands all members.
- **AC-05**: Group totals equal the sum of members and the category totals.
- **AC-06**: The verdict line is present, accurate, and matches the report fields; empty/one-pool/two-pool fixtures are golden-tested.
- **AC-07**: Threshold labels are visible in the table header or an adjacent legend; the pressure legend is present.
- **AC-08**: Every line is exactly `term.TermWidth()` visible columns (or a shorter footer) at 80/100/120, with `--ascii` and Unicode; no trailing whitespace.
- **AC-09**: The PID column no longer shows `-`; cache rows use an ID or omit the column.
- **AC-10**: `--wide` and `--no-truncate` show full names; `--no-group` shows individual rows.
- **AC-11**: Cleanup output preserves unique suffixes and groups repeated targets.
- **AC-12**: `scripts/check.sh --full` passes; `go.mod` unchanged.

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -race -run 'Test(TruncateMiddle|ShortID|Group|Verdict|TableWidth|NoDuplicate|Threshold|Pressure|Wide)' ./internal/term ./internal/triage ./internal/app ./internal/cleaner
./swapaudit --no-color --ascii --limit 12 > /tmp/ui-100.txt
COLUMNS=120 ./swapaudit --no-color --ascii --limit 12 > /tmp/ui-120.txt
./swapaudit --no-color --ascii --no-group --limit 20 > /tmp/ui-nogroup.txt
./swapaudit --no-color --ascii --wide --limit 5 > /tmp/ui-wide.txt
awk '{ if (length($0) > 100) print NR": "length($0) }' /tmp/ui-100.txt   # expect none for 100-col capture
grep -c "TestGoalsStatsFlagConflicts\*" /tmp/ui-100.txt                  # expect 1
grep -c "Top Memory & Cache Occupants" /tmp/ui-100.txt                   # expect 0 if Layout A title changed
./scripts/check.sh --full
```

Note: the shell cannot set the ioctl width by `COLUMNS` alone; use a PTY harness or `term.SetTermWidth` in tests for 80/100/120. The commands above are for manual text capture; the width invariants are enforced by Go tests.

## Test Matrix

| Test | File | Proves |
| --- | --- | --- |
| `TestTruncateMiddleWidths` | `internal/term/term_test.go` | Width, suffix preservation, no dangling ellipsis |
| `TestTruncateMiddleANSI` | same | ANSI styling and width |
| `TestShortID` | `internal/model/model_test.go` | Deterministic, path/PID based |
| `TestNormalizeGroupKey` | `internal/triage/group_test.go` | Pattern normalization |
| `TestGroupItems` | same | Count/size/idle range/ties |
| `TestExpandGroup` | same | All members returned |
| `TestGroupCategoryTies` | same | Sums equal category totals |
| `TestNoDuplicateRows` | `internal/app/display_test.go` | Actionable/protected disjoint |
| `TestVerdict` | same | Empty/one/two/unattributed |
| `TestThresholdLabels` | same | Thresholds in headers/legend |
| `TestPressureLegend` | same | 50/80 constants and NO_COLOR |
| `TestTableWidthInvariant` | `internal/term/term_test.go` | 80/100/120, ASCII/Unicode |
| `TestNoTrailingWhitespace` | same | Clean output |
| `TestWideMode` | `internal/app/display_test.go` | Full names |
| `TestNoGroupMode` | same | Expansion |
| `TestCleanupMiddleTruncation` | `internal/cleaner/cleaner_test.go` | Shared-prefix suffixes |
| `TestCleanupGroupSummary` | same | Grouped cleanup footer |

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Middle truncation is ambiguous when both prefix and suffix repeat | Add short ID; `--wide` for full names |
| Grouping hides a dangerous individual target | Group row shows the largest member and count; `--no-group` expands; the action applies to all |
| Grouping changes the meaning of "Actionable" totals | Test group totals against category totals; group rows carry the same `Reclaim` sum |
| Removing duplicate rows makes the Top table less useful | The protected table keeps all non-cleanable items; the verdict names the biggest win |
| Verdict line wraps on narrow terminals | Hanging indent/wrap; test at 80; keep the primary answer first |
| Width invariants fail with wide glyphs/ANSI | `StrWidth`-based tests; golden files for ASCII/Unicode |
| `--wide` can exceed the terminal | Document; only `--wide`/`--no-truncate` may exceed |
| Short IDs add visual noise | Show only when the name was truncated; `--wide` hides them |

## Rollback

- Revert the implementation commit. No persistent state.
- If grouping is too disruptive, keep middle truncation, duplicate removal, verdict, and width contract; defer grouping to a follow-up.
- Do not restore right-truncated names or duplicate rows as the default.

## References

- UI review screenshot and claim list, 2026-10-19.
- Verification report: `docs/reviews/2026-10-19-swapaudit-ui-review.md`.
- `internal/term/term.go:296-356`, `:400-416`, `:424-540`.
- `internal/app/display.go:84-174`, `:179-306`, `:316-458`.
- `internal/triage/triage.go:56-92`.
- `internal/cleaner/cleaner.go:137-190`, `:271-280`.
- Goal 1's `AuditReport`, Goal 2's cache categories, Goal 3's agent population.
