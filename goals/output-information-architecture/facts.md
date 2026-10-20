# Facts: Redesign Audit Tables for Identity, Grouping, Reconciliation, and Layout

## Architectural Invariants & Constraints

- **Identity survives truncation.** A truncated name must preserve the distinguishing prefix and suffix; when it cannot, a deterministic short ID disambiguates it.
- **No duplicate data rows.** An item/group appears in exactly one of the actionable or protected tables.
- **Group totals are additive.** The sum of group totals equals the category totals from Goal 1; grouping never changes the reclaim sum.
- **One verdict, one source.** The verdict is derived from `AuditReport`; display code does not recompute totals.
- **One width contract.** Every panel uses the same visible width; footers may be shorter but never wider; no trailing whitespace.
- **Thresholds are visible.** A row's idle/age value is accompanied by the threshold that produced its category.
- **Grouping is a view.** It does not change classification, cleanup eligibility, or the underlying item list.
- **No new dependency.** Use the standard library, `crypto/sha256`, `path/filepath`, and existing terminal helpers.
- **`--wide`/`--no-truncate` may exceed the terminal; the default may not.**

## Observed Facts (Repository)

| Fact | Evidence | Consequence |
| --- | --- | --- |
| `Truncate` keeps the prefix and appends the ellipsis | `internal/term/term.go:296-356` | Distinguishing suffix is lost |
| Display uses `term.Truncate` for names | `internal/app/display.go:135`, `:275` | Repeated test names become indistinguishable |
| Cleaner uses `term.Truncate` for paths | `internal/cleaner/cleaner.go:173`, `:272` | Destructive preview has the same ambiguity |
| Actionable selects all cleanable items | `internal/app/display.go:55-82` | Duplicates with the main table |
| Main table selects all items | `internal/app/display.go:179-306` | Same rows appear twice |
| No verdict function exists | `internal/app/display.go:17-51` | User must synthesize the answer |
| Thresholds are only in quick commands | `internal/app/display.go:448-450` | Table values lack context |
| PID 0 prints `-` | `internal/app/display.go:129-133`, `:261-264` | Mostly empty column |
| Shell aggregation is the only grouping | `internal/triage/triage.go:56-92` | Cache/test patterns are ungrouped |
| Table offsets differ by panel | `internal/app/display.go:91`, `:216`, `:333` | No explicit shared width contract |
| `RenderTable` pads the last column | `internal/term/term.go:494-504` | Trailing whitespace |
| Pressure thresholds are inline magic numbers | `internal/term/term.go:400-416` | No legend/constants |

## Observed Facts (Live Host, 2026-10-19)

| Observation | Command | Value |
| --- | --- | --- |
| Truncated names | `./swapaudit --no-color --ascii` | `sila-cli-test-p...`, `TestMCPServerPl...`, `TestMCPListGoal...`, `TestSilenceCryp...`, `TestRecentActiv...` |
| Duplicate rows | same | `agy 582.3 MB`, `go-build3514075730 452.5 MB`, `sila-cli-test-profile-... 344.4 MB`, `go-build879996547 329.4 MB` appear in both tables |
| Repeated pattern | same | Many `TestGoalsStatsFlagConflicts*` and `sila_mcp_test_*` entries |
| Table width | `awk '{print length($0)}'` | 100 visible columns at the 100-column capture; no current overflow |
| Trailing spaces | same | Rows are padded to 100; the last cell has trailing spaces |
| PID column | same | Cache rows show `-`; only process rows have a PID |
| No verdict | same | Header goes straight to the actionable table |
| Thresholds | same | Only the quick-command footer says `> 12h`/`> 2h` |

## External Facts

- ANSI escape sequences do not occupy visible cells; `term.StrWidth` already strips them.
- Wide glyphs/emoji can occupy two cells; `StrWidth` already handles the ranges it knows.
- A SHA-256 short ID is deterministic and collision-resistant enough for display; it is not a security identifier.
- Terminal width is clamped to 80..120 by `GetTerminalWidth`; tests can override it with `term.SetTermWidth`.
- `os.RemoveAll` semantics and the cleanup safety contract are unchanged by this goal.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| Suffix preservation is the right default for cache/test names | Instance IDs are at the end | Shared-prefix property test and live output |
| A short ID is acceptable when suffix preservation is insufficient | Some names share both prefix and suffix | `TestShortID`, duplicate-name fixture |
| Grouping by normalized prefix is semantically safe for display | The user can expand with `--no-group`/`--all` | Group tests; no classification changes |
| One of Layout A/B is sufficient; no interactive table is needed | The CLI is non-interactive | Golden tests |
| A one-line verdict can be wrapped rather than omitted on narrow terminals | The answer is the product's core value | 80-column golden test |
| Exported pressure constants do not change behavior | They formalize 50/80 | Boundary tests |

## Unknowns

- Whether the screenshot's differing right borders are a real width bug or a rendering artifact. **Unknown.** The width invariant test will settle it; the current 100-column capture is consistent.
- Which grouping depth users prefer (pattern only vs pattern + category). **Unknown.** Default to pattern + category; `--no-group` is the escape hatch.
- Whether short IDs are more useful than full paths for cache rows. **Unknown.** Show short IDs only when truncated; `--wide` shows full paths.
- Whether a one-line verdict is too wide for 80 columns. **Design choice.** Wrap with a hanging indent; keep the primary reclaim number first.
- Whether group rows should be actionable as a unit or only expandable. **Design choice.** The goal chooses actionable groups with `--no-group` expansion; evidence can revisit.

## Evidence Commands

```bash
grep -n "func Truncate\|func RenderTable\|func RenderMeterBar\|func PrintBoxHeader" internal/term/term.go
grep -n "term.Truncate\|targetWidth\|limit := 12\|Top Memory" internal/app/display.go internal/cleaner/cleaner.go
grep -n "AggregateShells" internal/triage/triage.go
./swapaudit --no-color --ascii > /tmp/ui-before.txt
awk '{print length($0)}' /tmp/ui-before.txt | sort | uniq -c
awk 'NF && /^  [> ]/ {print}' /tmp/ui-before.txt | sed -n '1,20p'
go test -race ./...
```

## File & Interface Contracts

- `internal/term`: `TruncateMiddle`, `TruncateMiddleANSI`, `TableWidth`, `PressureAmber`, `PressureRed`; `RenderTable` trims trailing whitespace.
- `internal/model`: `ShortID(item)`; optional `GroupKey`/`DisplayName` view fields.
- `internal/triage`: `Group`, `NormalizeGroupKey`, `GroupItems`, `ExpandGroup`.
- `internal/app/display.go`: `FormatVerdict`, grouped tables, disjoint actionable/protected tables, threshold labels, ID/PID column, width layout.
- `internal/app/app.go`: `ViewOptions{Group, Expand, Wide, NoTruncate}`; flags `--no-group`, `--expand`, `--collapse`, `--wide`, `--no-truncate`.
- `internal/cleaner`: middle-truncated paths, short IDs, grouped cleanup summaries.
- `docs/output-layout.md` (new) or `docs/metrics.md` section: the I1–I7 contract.
- `go.mod`: unchanged.
