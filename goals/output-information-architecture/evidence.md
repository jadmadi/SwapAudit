# Evidence: output information architecture

Verdict: PASS

Date: 2026-10-20.

## Before (goal facts, 2026-10-19)

- Names were right-truncated: `sila-cli-test-p...`, `TestMCPServerPl...`.
- Actionable and Top Occupants repeated the same rows.
- No verdict line.
- Thresholds appeared only in the quick-command footer.
- The PID column was mostly `-`.
- No pattern grouping.
- Width contract was implicit (`w-82`, `w-68`, `w-61`).

## After (live host, 2026-10-20)

Grouped repeated patterns:

```text
  ID       | Target / Command            | Category         |        Age |         Idle |       Size
  ---------+-----------------------------+------------------+------------+--------------+-----------
           | go-build* ×10               | in-use-caches    |     14h44m |     20–14h44 |     2.4 GB
           | bash (66 shells)            | active-apps      |     14h57m |           1m |   420.6 MB
           | npm exec chrome ×4          | active-apps      |      2d22h |  12h43–2d22h |   144.6 MB
```

Actionable rows are a disjoint set:

```text
           | agy ×4            | dormant-agents  |    7h16–7h42 | End dormant session   |   736.2 MB
           | chrome-de...ls ×4 | dormant-agents  |   7h30–13h21 | End dormant session   |   348.2 MB
```

`chrome-de...ls` demonstrates middle truncation: the shared `chrome-devtools` prefix is shortened but the distinguishing suffix survives. `--wide` shows the full name; `TestWideMode` asserts both modes.

## Verdict and thresholds

```text
  Verdict: Swap 34.0% used · 1.1 GB reclaimable (1.1 GB swap; upper bound; pools may overlap) ·
  biggest win: agy PID 1430533 (587.3 MB) · thresholds: cache 12h, agent 2h, orphan-cache 1h

  Pressure: green <50% · amber 50–79% · red ≥80%
  Process swap (VmSwap) · dormant > 2h
  /tmp allocated (tmpfs) · stale > 12h · orphan-cache > 1h
```

If nothing is cleanable, the verdict starts with `nothing safe to reclaim` and reports the residuals.

## Population counts

```text
Actionable population: 8 cleanable · 8 shown (2 rows) · 0 hidden (0 B) · limit 3 rows
Population: 549 raw · 478 after shell aggregation · 15 shown · 428 hidden below 10 MB · 35 beyond --limit
Population: 557 entities · 8 cleanable · 549 protected · 3.3 GB swap unattributed · 1.8 GB /tmp unattributed
```

428 + 35 + 15 = 478. `--all --limit 0` prints no hidden footer:

```text
$ ./swapaudit --all --limit 0 > /tmp/all.txt
$ grep -c 'more hidden' /tmp/all.txt
0
$ grep -c 'more groups' /tmp/all.txt
0
```

## Tests

```text
=== RUN   TestTruncateMiddleWidths       --- PASS  (widths 0..80)
=== RUN   TestTruncateMiddleANSI         --- PASS
=== RUN   TestShortID                    --- PASS
=== RUN   TestNormalizeGroupKey          --- PASS
=== RUN   TestGroupItems                 --- PASS
=== RUN   TestGroupCategoryTies          --- PASS
=== RUN   TestNoDuplicateRows            --- PASS
=== RUN   TestVerdict                    --- PASS  (FormatVerdict)
=== RUN   TestThresholdLabelsAndPressureLegend --- PASS
=== RUN   TestTableWidthAndNoTrailingWhitespace --- PASS  (80/100/120, ASCII)
=== RUN   TestFitColumnsAndTableWidth    --- PASS
=== RUN   TestTableOverflow              --- PASS  (--wide)
=== RUN   TestWideMode                   --- PASS
=== RUN   TestNoGroupMode                --- PASS  (TestGoalsStatsFlagConflicts* ×2 at 120 cols)
=== RUN   TestNoDashID                   --- PASS
=== RUN   TestPopulationExactCounts      --- PASS  (20/18/4/5/9 arithmetic)
=== RUN   TestAllModeWithExplicitLimit    --- PASS  (--all --limit 3)
=== RUN   TestCleanupMiddleTruncation    --- PASS  (five shared-prefix paths keep distinct suffixes)
=== RUN   TestCleanupGroupSummary        --- PASS
```

`TestTableWidthAndNoTrailingWhitespace` asserts every table header, separator, and data row is exactly the terminal width at 80, 100, and 120, and that no line contains trailing whitespace. Prose lines are wrapped to at most the width.

## Cleanup identity

Cleanup output uses `TruncateMiddle`; five targets sharing a long prefix keep `aaaa1111`, `bbbb2222`, `cccc3333`, `dddd4444`, and `eeee5555`. Repeated targets also produce a `Groups: key ×N` line.

## Deviation

`--expand <key>` expands one group by prefix; `--collapse` restores the default. No interactive expand/collapse was added.
