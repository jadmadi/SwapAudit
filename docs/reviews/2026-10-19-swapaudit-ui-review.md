# UI Review Verification — swapaudit — 2026-10-19

## Review metadata

| Field | Value |
| --- | --- |
| Subject | Screenshot review: swap and in-memory cache auditor |
| Reviewed artifact | `swapaudit` TUI screenshot and claim list supplied in the session |
| Current tree | `/home/jad/scripts/active/swapaudit` (untracked standalone directory inside the parent `scripts` worktree) |
| Current build | `swapaudit dev (commit: none, built: unknown)` from `go build ./cmd/swapaudit` |
| Host | Linux, 17.1 GiB swap, 31.2 GiB RAM, 15.6 GiB `/tmp` tmpfs |
| Date | 2026-10-19 |
| Method | Source inspection plus a live read-only audit and a dry-run cleanup on the current tree |
| Goal packages | `goals/accounting-reconciliation`, `cache-liveness-classification`, `orphan-visibility-safety`, `output-information-architecture`, `machine-readable-audit` |

## Verdict

The review is **directionally correct on the two priority defects**: the screen does not reconcile with system totals, and the largest `/tmp` bucket is classified only by mtime. Six of the eight bug claims are confirmed in the current tree. One is **partially outdated** because the destructive safety work has already landed. One contains **incorrect arithmetic** (the actionable count and hidden bytes are internally correct in the current output), although the underlying population-reconciliation problem is real.

The goals are split so the two P0 defects can be fixed first, followed by visibility/safety and information architecture, with JSON deferred:

| Priority | Goal | Fixes |
| --- | --- | --- |
| P0 | `accounting-reconciliation` | Review bugs #1, #2, #4, #3 (populations/footers) |
| P0 | `cache-liveness-classification` | Review bug #5; adds open-handle evidence and opt-in orphaned caches |
| P1 | `orphan-visibility-safety` | Review bug #7 (zero-swap/zero-row/evidence), parent recheck, signal text, non-TTY guard |
| P1 | `output-information-architecture` | Review bugs #6, #8; grouping, duplicate removal, verdict, width contract |
| P2 (roadmap) | `machine-readable-audit` | Review improvements `--json`/`--yes`, schema, exit codes |

## Claim-by-claim verification

| # | Claim | Status | Evidence | Goal |
| --- | --- | --- | --- | --- |
| 1 | Header totals do not tie to category sums; no unattributed row | **Confirmed** | Header `Swap 7.5 GB`; categories `814.7 MB + 263.3 MB + 1.8 GB ≈ 2.85 GB`; independent `Σ VmSwap ≈ 2.86 GB`; residual ≈4.65 GB. Header `/tmp 5.2 GB`; categories `≈4.15 GB`; residual ≈1.05 GB. Sources: `proc.go:54-55`, `:68-73`; `display.go:316-328`; `cache.go:44`, `:51-70`, `:83`, `:91-135`. | 1 |
| 2 | "Process Swap [retired RAM label]" is undefined; Size is not swap/RSS/PSS | **Confirmed** | The label is literal at `display.go:366`; values are `it.SwapBytes` (`display.go:321`, `:372`) from `/proc/[pid]/status:VmSwap`. Processes with `VmSwap == 0` are excluded (`app.go:63-65`). Header RAM is `MemTotal - MemAvailable` (`proc.go:55`), a different metric. | 1 |
| 3 | Counts do not reconcile: "65 smaller candidates" should be 66; "702 hidden" does not match 785 entities | **Partially confirmed / arithmetic incorrect** | Current output: 150 cleanable, 12 shown, 138 hidden, 23.3 MB hidden; the visible 12 sum to 1.9 GB − 23.3 MB. The actionable count is internally correct. The review's 11-row count is a miscount of the screenshot (12 rows are shown), and the `~2.8 MB` figure does not match the current 23.3 MB population. The real defect: the main table's `717 more hidden` uses a post-shell-aggregation, post-10 MB-filter population, while the summary reports 804 raw `Count` entities; the difference is not explained. Sources: `display.go:117-174`, `:251-304`, `:322`; `triage.go:56-92`. | 1 |
| 4 | "Cleanable: 1.2 GB" adds two different pools | **Confirmed** | `display.go:84` sums `totalReclaimTmp + totalReclaimSwap`; the dry-run already splits them (`cleaner.go:358-370`). The pools overlap: tmpfs pages can be swapped. | 1 |
| 5 | "Active-caches" is a misnomer; classification is mtime-only | **Confirmed** | `cache.go:137-150`: `cleanable := lastActive > maxAge`; else `CatActiveCache`. No open-fd/cwd/maps check. Current output: 434 entities, 3.0 GB, 0 B actionable; top rows have `Age == Idle` (`go-build873559002 6h45m/6h45m`, `go-build2547211510 3h37m/3h37m`). | 2 |
| 6 | Truncation destroys the identifier | **Confirmed** | `term.go:296-356` right-truncates; current output shows `sila-cli-test-p...`, `TestMCPServerPl...`, `TestMCPListGoal...`, `TestSilenceCryp...`, `TestRecentActiv...`. Cleanup output uses the same `term.Truncate` (`cleaner.go:173`, `:272`). The "detached ellipsis" is a rendering artifact; the identity loss is real. | 4 |
| 7 | `orphaned` is promised but never shown | **Partially outdated** | Current tree already has `ClassifyOrphan` (`proc.go:207-253`), `CatAbandonedAgent`, an opt-in `--include-orphaned`, UID checks (`cleaner.go:234-250`), start-ticks recheck (`:252-268`), and the false "conversations preserved" claim is gone. However: `CollectSwapItems` drops `VmSwap == 0` (`app.go:63-65`), so zero-swap orphans are invisible; the `abandoned-agents` row is omitted when zero (`display.go:368-369`); evidence is discarded (`app.go:75`); no signal text. | 3 |
| 8 | Layout: table widths/right borders differ; PID column mostly `-` | **Partially confirmed** | At the current 100-column capture, every header, separator, and row is exactly 100 visible columns; the screenshot's right-edge difference is not reproduced. There is no width-invariant test, and `RenderTable` pads the last column with trailing spaces. The PID column is mostly `-` because cache rows have `PID == 0` and aggregated shells set `PID = 0` (`display.go:129-133`, `:261-264`; `triage.go:88`). | 4 |

## Additional defects found during verification

These were not in the review list but are directly related and are folded into the goals:

1. **`--all` does not show all.** `display.go:251-258` still hides items below 10 MB that are not cleanable. Live `--all --limit 0` printed `... and 594 more hidden`, despite the help saying "Display all processes individually" (`display.go:476`). → Goal 1.
2. **`PruneTmpTargets` footer count can disagree with its bytes.** `cleaner.go:187-189` uses `len(targets)-shown`, which includes skipped targets, while `hiddenBytes` only sums non-shown, non-skipped targets. → Goal 1.
3. **Cache size uses apparent size, not allocated blocks.** `cache.go:124` sums `info.Size()`, so sparse files overstate reclaimable tmpfs. → Goal 1.
4. **`--limit` does not apply to the actionable table.** `display.go:117` hardcodes 12; the `--limit` flag only affects the main table. The help says "main table", so this is a documentation/semantics gap, not a crash. → Goal 1/4.
5. **Zero categories and agent evidence are omitted.** `display.go:368-369`; `app.go:75`. → Goal 3.
6. **Idle-unknown is silent.** `proc.go:331-336` returns `-1`; `triage.go:47-50` treats it as "not dormant". → Goal 3.
7. **Non-TTY cleanup silently skips and exits 0.** `cleaner.go:81-89`, `:123-131`, `:216-224`. → Goal 3.
8. **Signal is not printed.** `cleaner.go:264-280` sends `SIGTERM` but prints only `ended`/`would end`. → Goal 3.
9. **No JSON.** No user-facing `--json`; `--yes` exists; `--dry-run` already behaves as a modifier. → Goal 5.
10. **No verdict line or threshold labels.** `display.go:17-51`, `:448-450`. → Goal 4.

## What is already fixed in the current tree (do not re-fix)

The screenshot predates the current safety work. The following review suggestions are already implemented or superseded:

- `--clean-all` does **not** include orphaned cleanup; `--include-orphaned` is required (`app.go:119-137`, `:229-234`).
- Process kills check the target UID in dry-run and live mode (`cleaner.go:234-250`).
- Start ticks are re-read immediately before `SIGTERM` (`cleaner.go:252-268`).
- The orphan heuristic is evidence-gated (`.service`/`.scope`, tty, parent state) and fails closed on ambiguous evidence (`proc.go:207-253`).
- The unverified "Conversations stay preserved on disk" prompt is gone; the dormant prompt warns about unsaved state (`cleaner.go:353`).
- `--dry-run` is already a modifier: `--clean-all --dry-run` works and never mutates.
- `--yes` already exists for scripting.

## Live evidence (2026-10-19)

### Captured audit

```text
Swap  [#######.........]  44.0%   7.5 GB /  17.1 GB (free 9.6 GB)
RAM   [########........]  53.3%  16.7 GB /  31.2 GB (avail 14.6 GB)
/tmp  [#####...........]  33.4%   5.2 GB /  15.6 GB (free 10.4 GB)

Actionable Reclaim Candidates (Cleanable: 1.9 GB)
... 150 cleanable, 12 shown, 138 hidden (23.3 MB)

Process Swap [retired RAM label]
  dormant-agents  5   814.7 MB  actionable 814.7 MB
  desktop-daemons 17  263.3 MB  actionable 0 B
  active-apps     203 1.8 GB    actionable 0 B

/tmp Caches (tmpfs)
  untouched-caches 9   782.2 MB  actionable 782.2 MB
  abandoned-runs   136 373.8 MB  actionable 373.8 MB
  active-caches    434 3.0 GB    actionable 0 B
```

### Independent totals

```text
SwapTotal: 17892336 kB
SwapFree:  10015744 kB
SwapUsed:   7876592 kB  (~7.5 GiB)
Σ VmSwap:   2999900 kB  (~2.86 GiB)
Residual:  ~4.65 GiB
/tmp used: 5596966912 bytes (~5.2 GiB)
Scanned categories: ~4.15 GiB
Residual: ~1.05 GiB
```

### `--all` behavior

```text
./swapaudit --no-color --ascii --all --limit 0
... and 594 more hidden. Run with --all to inspect full list.
```

`--all` does not disable the `sz < 10 MB && !Cleanable` filter.

### Test baseline

```text
go test -race ./...   # all packages pass
go vet ./...          # clean
```

## Goal execution order

1. **`accounting-reconciliation`** (P0, independent) — exact-byte residuals, unattributed rows, split cleanable, allocated size, population/footer fixes, `--all` semantics.
2. **`cache-liveness-classification`** (P0, depends on 1) — `/proc` open-handle index, `recent`/`in-use`/`orphaned` categories, opt-in orphaned-cache cleanup, deletion recheck.
3. **`orphan-visibility-safety`** (P1, depends on 2) — zero-swap agents, always-on zero rows, evidence display, parent/orphan/idle rechecks, `SIGTERM` label, non-TTY guard.
4. **`output-information-architecture`** (P1, depends on 1–3) — middle truncation, short IDs, grouping, duplicate removal, verdict, thresholds, width contract.
5. **`machine-readable-audit`** (P2/roadmap, depends on 1 and 4) — versioned `--json`, cleanup result, exit-code matrix, golden schema tests.

## Recommendation

Execute Goals 1 and 2 before any release or cleanup claim. They fix the two priority defects and produce a report that every later goal can trust. Goals 3 and 4 should follow before `--clean-all` is recommended to users. Goal 5 is valuable for automation but is not a blocker for the correctness fix.

The existing `standalone-v0-1-0-release` goal currently lists only the three original safety prerequisites. If the project wants these UI/accounting fixes to gate v0.1.0, add the P0/P1 goals as dependencies:

```bash
sila goals depend add standalone-v0-1-0-release accounting-reconciliation cache-liveness-classification orphan-visibility-safety
```

That dependency change was not made automatically; it is a roadmap decision for the maintainer.
