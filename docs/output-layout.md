# Output layout contract

The default output is one answer, then evidence, then actions. All tables are formula-driven; no hardcoded `w-82` offsets.

## Identity

- `TruncateMiddle` keeps both the beginning and the distinguishing suffix. `sila-cli-test-profile-1784434295` at width 24 keeps `1784434295`.
- ANSI escape sequences are preserved and a reset is appended when styling was opened.
- Every item has a deterministic eight-hex `ShortID`: SHA-256 of the path, or `pid:startticks` for processes.
- The `ID` column shows the PID for process rows and the short ID for cache rows. Grouped rows leave it blank.
- `--wide`/`--no-truncate` shows full names and may exceed the terminal width. It is the only mode allowed to do so.

## Grouping

`NormalizeGroupKey` strips trailing instance numbers, hashes, and PIDs:

- `go-build3514075730` → `go-build*`
- `TestGoalsStatsFlagConflicts11102814295` → `TestGoalsStatsFlagConflicts*`
- `sila_mcp_test_426715724` → `sila_mcp_test*`
- `agy`, `chrome-devtools`, and names without an instance suffix are not rewritten.

Groups never span categories or cache kinds. A group renders as `key ×N` with the total size, the idle range, and the action. `--no-group` expands everything; `--expand <key>` expands one group; `--collapse` restores the default.

Group totals equal the sum of their members and tie to the category totals.

## Tables

Layout A is implemented:

- **Actionable Reclaim Candidates** contains only cleanable items/groups. Columns: ID, name, category, idle, action, reclaim.
- **Protected Occupants** contains only non-cleanable items/groups. Columns: ID, name, category, age, idle, size.
- No item or group appears in both tables.
- `Top Memory & Cache Occupants` is retired.

## Verdict

One line (wrapped with a hanging indent when needed):

```text
Verdict: Swap 48.5% used · 1.9 GB reclaimable (1.1 GB tmpfs + 815.0 MB swap; upper bound; pools may overlap) · biggest win: agy PID 2603976 (582.3 MB) · thresholds: cache 12h, agent 2h, orphan-cache 1h
```

If nothing is cleanable: `Verdict: Swap X% used · nothing safe to reclaim · A swap and B /tmp unattributed`.

## Thresholds and legend

Thresholds are labelled in the table headers or the group header:

- `/tmp allocated (tmpfs) · stale > 12h · orphan-cache > 1h`
- `Process swap (VmSwap) · dormant > 2h`

The header includes `Pressure: green <50% · amber 50–79% · red ≥80%`. The constants are exported as `term.PressureAmber` and `term.PressureRed`.

## Width and whitespace

- `FitColumns` sizes the flexible column so `TableWidth(cols) == TermWidth()` at 80, 100, and 120 columns.
- Table headers, separators, and data rows are exactly the terminal width.
- The final column is right-aligned, so no table line carries trailing whitespace.
- Prose lines (verdict, reconciliation, population, footers) wrap to at most the terminal width; they may be shorter.
- `--ascii` and Unicode both satisfy the same contract.

## Population lines

Every count explains its population:

```text
Actionable population: 8 cleanable · 8 shown (2 rows) · 0 hidden (0 B) · limit 12 rows
Population: 549 raw · 478 after shell aggregation · 14 shown · 429 hidden below 10 MB · 35 beyond --limit
Population: 557 entities · 8 cleanable · 549 protected · 3.3 GB swap unattributed · 1.9 GB /tmp unattributed
```

`--all` disables shell aggregation, grouping, and the 10 MB protected-item filter; its default limit is `0` unless `--limit` is explicit.

## Cleanup output

Cleanup targets use `TruncateMiddle`, so shared prefixes keep distinct suffixes. Repeated patterns add a `Groups: key ×N` line. Every process action prints `SIGTERM` (`would-signal` in dry-run, `signaled` live). The cleanup summary uses the same `Reclaim` split as the audit: `X tmp` and `Y swap`, never a single primary total.
