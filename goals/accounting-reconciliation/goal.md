# Goal: Make Swap and tmpfs Accounting Reconcile with Every Header Total

## Chain of Custody (Isnad)

- **Registered By**: UI review follow-up, 2026-10-19
- **Spec Audited By**: CommandCode, 2026-10-20 (acceptance criteria checked against tests and evidence)
- **Implemented By**: CommandCode, 2026-10-20 (see `evidence.md`)
- **Mystery Audited By**: Evidence certification, 2026-10-20 (see `certification.json`; independent mystery audit not run)

## Goal Description

Define one accounting model that makes every displayed byte traceable to a kernel total or to an explicit residual. Fix the three defects that make the current screen impossible to reconcile:

1. The header reports system swap `/tmp` usage, but the category table reports only a subset of attributed items and silently drops the difference.
2. `Process Swap (RAM)` is not RAM, not RSS, and not PSS; it is the sum of `/proc/[pid]/status:VmSwap` for processes that have swapped pages. The header `RAM` value is `MemTotal - MemAvailable`, a different metric.
3. `Cleanable: 1.9 GB` adds tmpfs bytes and swapped-process bytes into one number, although the pools overlap physically (tmpfs pages may themselves be swapped out) and have different reclaim semantics.

The goal is not to add PSS/RSS collection, a new scanner, or machine-readable output. It is to make the existing screen truthful, add residual rows, split cleanable totals by pool, and make every count explain its population.

## Business Context

- This is the first item in the UI review priority: "The screen doesn't reconcile with system totals."
- A user cannot trust a cleanup decision if the screen says 7.5 GB of swap is used and the categories account for 2.9 GB with no explanation.
- The `/tmp` scanner intentionally filters by UID, name pattern, daemon prefix, dotfile prefix, and top-level entry. Those filters are defensible, but the excluded bytes must be visible as `unattributed`, not implied to be zero.
- The existing dry-run already prints `Would reclaim 1.1 GB tmp, 814.7 MB swap`; the audit header does not. This inconsistency is the immediate trigger.
- This goal should land before `standalone-v0-1-0-release`; it is a reporting/trust fix, not a new feature.

## Verified Findings (2026-10-19, current tree)

### F1 — Swap header does not tie to process categories

- `internal/proc/proc.go:54` computes `SwapUsed = SwapTotal - SwapFree`.
- `internal/app/display.go:316-328` sums `SwapBytes` by category.
- Live capture on this host:
  - Header: `Swap 7.5 GB / 17.1 GB (free 9.6 GB)`.
  - Category rows: `dormant-agents 814.7 MB + desktop-daemons 263.3 MB + active-apps 1.8 GB ≈ 2.85 GB`.
  - Independent `awk '/^VmSwap:/{s+=$2} END{...}' /proc/[0-9]*/status` returned `2999900 kB ≈ 2.86 GB`.
- The gap (≈4.65 GB) is real and expected: kernel swap, swapped tmpfs/shmem pages, inaccessible processes, and shared swap entries are not represented by any single process's `VmSwap`.

### F2 — "Process Swap (RAM)" is an undefined metric

- `internal/app/display.go:366` prints the literal label `Process Swap (RAM)`.
- The values in that group are `swapSums[c]`, populated from `it.SwapBytes` (`internal/app/display.go:321`, `:372`).
- `internal/app/app.go:63-65` includes a process only when `swapBytes > 0`.
- `internal/proc/proc.go:55` defines `MemUsed = MemTotal - MemAvail`; the header `RAM` row (`internal/app/display.go:33-40`) prints that value. It is not the sum of process memory and cannot be compared with `Σ VmSwap`.

### F3 — /tmp header does not tie to scanned cache categories

- `internal/proc/proc.go:68-73` computes `TmpfsUsed = Blocks*Bsize - Bfree*Bsize`.
- `internal/cache/cache.go:44` limits the scan to the current UID.
- `internal/cache/cache.go:51-70` skips known daemon prefixes.
- `internal/cache/cache.go:58` skips dotfiles and `systemd-*`/`pulse-*`.
- `internal/cache/cache.go:91-114` includes only entries matching `DefaultTmpPatterns` or a keyword list.
- `internal/cache/cache.go:124` sums `info.Size()` (apparent size), not allocated blocks.
- Live capture: header `/tmp 5.2 GB`; scanned categories `782.2 MB + 373.8 MB + 3.0 GB ≈ 4.15 GB`; residual ≈1.05 GB.
- `internal/cache/cache.go:83` also excludes other UIDs from the scan, but those bytes remain in `statfs`.

### F4 — "Cleanable" adds two pools

- `internal/app/display.go:84` builds the title from `totalReclaimTmp + totalReclaimSwap`.
- `internal/app/display.go:59-65` accumulates `SizeBytes` and `SwapBytes` separately, then immediately adds them for display.
- The pools are not interchangeable: deleting a tmpfs file frees tmpfs space and may free swap if its pages were swapped; sending `SIGTERM` to a process frees the process's swapped pages but may free no tmpfs bytes. The two numbers may overlap.
- `internal/cleaner/cleaner.go:358-370` already prints them separately for cleanup (`Would reclaim X tmp, Y swap`); the audit header should match that honesty.

### F5 — Counts use different populations

- `internal/app/display.go:117` hardcodes the actionable limit to 12.
- `internal/app/display.go:122-127` counts every cleanable item for the hidden count; for the current output, 150 cleanable − 12 shown = 138 hidden, which is arithmetically correct, and 23.3 MB hidden ties to the displayed 1.9 GB.
- `internal/app/display.go:302-304` reports `717 more hidden` for the main table, but the summary reports 804 raw entities (`counts[Category] += it.Count` at `internal/app/display.go:322`). The main-table population is post-`AggregateShells` and post-`sz < 10 MB && !Cleanable`; the 717 is correct for that population but is not explained.
- `--all` does not show all: `internal/app/display.go:251-258` still hides small protected items. Live capture with `--all --limit 0` printed `... and 594 more hidden`.
- `internal/cleaner/cleaner.go:187-189` prints `len(targets)-shown` as the hidden target count even when targets were skipped; `hiddenBytes` only accumulates non-shown, non-skipped targets. The count and the bytes can disagree.

**Review correction:** the review's "65 should be 66 (77 actionable minus 11 shown)" is based on a miscount of the screenshot. The current output shows 12 rows, and 150 − 12 = 138. The real defect is not an off-by-one in the actionable footer; it is the unreconciled populations and the misleading main-table footer.

### F6 — Apparent size vs allocated size

- `internal/cache/cache.go:124` uses `info.Size()`.
- Sparse files can report gigabytes of apparent size while consuming kilobytes of tmpfs blocks. The header `statfs` value counts allocated blocks, so the scanner can overstate what deletion will free.
- Directory metadata is also not counted, so a fully faithful reconciliation needs an explicit `metadata/other` residual rather than pretending the file-size sum is exact.

### F7 — Layout/count observations relevant to this goal

- All tables are formula-driven (`w-82`, `w-68`, `w-61`); at a 100-column capture every line is 100 visible columns. The screenshot's differing right edges are not reproduced in the current 100-column capture, but there is no invariant test.
- The `PID` column is mostly `-` because cache rows have `PID == 0`; this is a display concern that Goal 4 owns.

## Dependencies & Execution Order

- **Mode**: Independent (this goal owns the accounting contract)
- **Depends On**: none
- **Sequence**: 1
- **Shape**: ship
- **Tier**: immediate
- **Blocks**: `cache-liveness-classification`, `output-information-architecture`, `machine-readable-audit`

## Files to Touch

- `internal/report/accounting.go` (new) — pure report model and reconciliation
- `internal/report/accounting_test.go` (new)
- `internal/model/model.go` — add `AllocatedBytes`, `ApparentBytes`, `ID`, `Kind`, `Evidence`, `OpenHandles`; keep `SizeBytes` as the display/reclaim size
- `internal/model/model_test.go`
- `internal/cache/cache.go` — allocated-size walk, skip-reason accounting
- `internal/cache/cache_test.go`
- `internal/proc/proc.go` — add `ReadTmpfsUsage` if needed; expose exact byte totals; no behavior change
- `internal/app/app.go` — build and pass an `AuditReport`
- `internal/app/display.go` — header, verdict/reconciliation, category rows, population
- `internal/app/display_test.go` (new or extend `app_test.go`)
- `internal/term/term.go` — add `StrWidth`-safe helpers only if needed; no new rendering feature
- `docs/metrics.md` (new)
- `README.md`
- `CHANGELOG.md`
- `goals/accounting-reconciliation/*` (this package)

## Accounting Contract (over-specified)

### A1 — Canonical byte definitions

| Name | Definition | Source | Additive? |
| --- | --- | --- | --- |
| `SwapTotal` | `SwapTotal` from `/proc/meminfo` | `proc.ParseMeminfo` | no |
| `SwapFree` | `SwapFree` from `/proc/meminfo` | `proc.ParseMeminfo` | no |
| `SwapUsed` | `SwapTotal - SwapFree` | `proc.ParseMeminfo` | no |
| `ProcessSwapAttributed` | `Σ item.SwapBytes` over scanned processes with `VmSwap > 0` | `/proc/[pid]/status:VmSwap` | yes, within the scan |
| `SwapUnattributed` | `max(0, SwapUsed - ProcessSwapAttributed)` | derived | no |
| `SwapOverlap` | `max(0, ProcessSwapAttributed - SwapUsed)` when shared accounting exceeds the system total | derived | no |
| `TmpfsTotal` | `Blocks * Bsize` for `/tmp` | `statfs` | no |
| `TmpfsFree` | `Bfree * Bsize`; report `Bavail * Bsize` separately if it differs | `statfs` | no |
| `TmpfsUsed` | `TmpfsTotal - TmpfsFree` | `statfs` | no |
| `TmpfsAttributed` | `Σ item.AllocatedBytes` over scanned entries | walk of `/tmp` | yes, within the scan |
| `TmpfsUnattributed` | `max(0, TmpfsUsed - TmpfsAttributed)` | derived | no |
| `TmpfsOverlap` | `max(0, TmpfsAttributed - TmpfsUsed)` (scan/statfs race) | derived | no |
| `ReclaimTmpfs` | `Σ cleanable cache AllocatedBytes` | scan | upper bound |
| `ReclaimSwap` | `Σ cleanable process SwapBytes` | scan | upper bound |
| `MemTotal` | `MemTotal` from `/proc/meminfo` | `proc.ParseMeminfo` | no |
| `MemAvailable` | `MemAvailable` from `/proc/meminfo` | `proc.ParseMeminfo` | no |
| `MemUsed` | `MemTotal - MemAvailable`; label it `used = total - available`, never "process RAM" | `proc.ParseMeminfo` | no |

### A2 — Invariants

1. `SwapUnattributed + ProcessSwapAttributed == SwapUsed` exactly in `int64` bytes when neither is clamped.
2. `TmpfsUnattributed + TmpfsAttributed == TmpfsUsed` exactly when neither is clamped.
3. If a residual is negative, report `Overlap` instead of a negative `Unattributed`; never print a negative number.
4. `ReclaimTmpfs` and `ReclaimSwap` are never added into a `Total` field. They may be displayed side by side; a combined number is allowed only with the literal label `upper bound; pools may overlap` and must not be used for the verdict's primary number.
5. Every displayed byte comes from exactly one of: a kernel total, a category sum, or an `Unattributed`/`Overlap` row. No display function recomputes a sum that the report can provide.
6. Formatting may round to one decimal; the invariant is checked in exact bytes. The UI must state `± rounding` only if the formatted rows do not visibly add up.
7. `.. [residual]` rows are always rendered, including `0 B`, for both pools.
8. The share denominator is explicit: `share_attributed = category / attributed` and `share_system = category / system_total`. The default column label must say which one it uses.

### A3 — Report types

```go
// internal/report/accounting.go
package report

type Pool string

const (
    PoolProcessSwap Pool = "process-swap"
    PoolTmpfs       Pool = "tmpfs"
)

type ReasonBytes struct {
    Reason string
    Bytes  int64
    Count  int
}

type Attribution struct {
    TotalBytes        int64
    AttributedBytes   int64
    UnattributedBytes int64
    OverlapBytes      int64
    Reasons           []ReasonBytes // sorted by Bytes desc, then Reason
}

type Reclaim struct {
    TmpfsBytes   int64
    SwapBytes    int64
    CacheCount   int
    ProcessCount int
}

type Population struct {
    RawItems            int
    AggregatedShells    int
    AggregatedRows      int
    Displayed           int
    HiddenByLimit       int
    HiddenBySizeFilter  int
    Cleanable           int
    Protected           int
}

type CategoryTotal struct {
    Category          model.Category
    Pool              Pool
    Entities          int
    AttributedBytes   int64
    ReclaimableBytes  int64
    ShareOfAttributed float64
    ShareOfSystem     float64
}

type Thresholds struct {
    MaxAge          time.Duration
    IdleThreshold   time.Duration
    OrphanCacheAge  time.Duration // supplied by Goal 2; zero until then
}

type AuditReport struct {
    GeneratedAt time.Time
    System      model.SystemSwap
    Swap        Attribution
    Tmpfs       Attribution
    Reclaim     Reclaim
    Categories  []CategoryTotal
    Population  Population
    Thresholds  Thresholds
    Items       []model.SwapItem
}

func Build(now time.Time, system model.SystemSwap, items []model.SwapItem, thresholds Thresholds) AuditReport
func (r AuditReport) Reconciles() error
func (r AuditReport) BiggestWin() (model.SwapItem, bool)
func (r AuditReport) CleanableCount() int
```

- `Build` is pure: no I/O, no globals, no terminal state. It sorts categories in a deterministic order.
- `Build` must not mutate `items`.
- `Reconciles` returns a descriptive error if any invariant fails; tests use it.
- `BiggestWin` returns the largest `ItemSize` cleanable item; ties break by PID then name/path.

### A4 — Scanner skip reasons

Add a `SkipReason` accumulator to the cache scan and the process scan. At minimum:

- `other-uid` — top-level entry owned by another UID.
- `daemon-prefix` — known daemon prefix (`tmux-`, `codex-daemon-`, `plasma-`, etc.).
- `dotfile` — starts with `.`.
- `systemd-or-pulse` — `systemd-*` or `pulse-*`.
- `symlink` — top-level symlink.
- `special-file` — socket, named pipe, device.
- `non-matching` — does not match a cache/test pattern or keyword.
- `inaccessible` — `ReadDir`/`Lstat`/`WalkDir` error.
- `metadata` — allocated directory blocks not attributable to a file (computed only as a residual; never guessed per entry).

For processes:

- `zero-swap` — process scanned but `VmSwap == 0`; it is excluded from the swap pool but counted in the population (Goal 3 uses this).
- `inaccessible` — `/proc/[pid]/status` unreadable.
- `vanished` — process disappeared mid-scan.
- `own-process` — the auditor itself.

The summary must print only nonzero reasons, sorted by bytes descending.

### A5 — Allocated-size accounting

- Add `AllocatedBytes int64` and keep `ApparentBytes int64` to `model.SwapItem`.
- For caches, `SizeBytes` becomes `AllocatedBytes` (the value used for display and reclaim).
- `ApparentBytes` is kept for `--json` and for a future `--apparent` display; it is never used in reconciliation.
- On Linux, read `syscall.Stat_t.Blocks * 512`. If the stat is unavailable, fall back to `info.Size()` and increment `fallback-count`.
- For directories, include the directory inode's allocated blocks in the containing entry's total; do not count the scan root itself.
- Sparse file fixture: 1 GiB apparent, 4 KiB allocated; assert `ApparentBytes >= 1 GiB` and `AllocatedBytes <= 64 KiB` (filesystem block rounding).
- Hard links: the walk counts each path once; the residual can exceed the sum if hard links are counted twice. Document this as `hardlink-overlap` and do not attempt inode deduplication in this goal.

### A6 — Display contract

The header and summary must render:

```text
Verdict: Swap 44.0% used · 1.9 GB reclaimable (1.1 GB tmpfs + 815 MB swap; upper bound) · biggest win: agy PID 1430533 (582 MB)

Swap:  2.9 GB attributed + 4.6 GB unattributed = 7.5 GB used
/tmp:  4.2 GB attributed + 1.0 GB unattributed = 5.2 GB used
       unattributed reasons: other-uid 800 MB · non-matching 150 MB · dotfile 60 MB · metadata 40 MB
```

- The verdict line is owned by Goal 4 for styling, but the data comes from this report. If Goal 4 lands first, this goal may render a plain version.
- The category table group label becomes `Process swap (VmSwap)`; the literal `Process Swap (RAM)` must not appear anywhere.
- The `/tmp` group label becomes `/tmp allocated (tmpfs)`.
- The `Share` column header becomes `Share (attributed)`; add a second line or an optional `--share=attributed|system` flag if width allows. Default: attributed, with the total row showing the system denominator.
- The `Cleanable` header is replaced by `Reclaimable: 1.1 GB tmpfs · 815 MB swap`; no combined total in the primary position.
- The dry-run/summary line must use the same `Reclaim` fields.

### A7 — Population and count contract

- `PrintMainTable` must print one line:
  `Population: 804 raw · 739 after shell aggregation · 12 shown · 55 hidden below 10 MB · 717 beyond --limit`
  (exact numbers; omit zero components; use `raw`/`aggregated`/`shown`/`hidden` labels).
- `PrintActionableSection` must print:
  `Actionable population: 150 cleanable · 12 shown · 138 hidden (23.3 MB) · limit 12`.
- `--all` must disable shell aggregation, disable the 10 MB protected-item filter, and default `limit` to 0. An explicit `--limit N` still caps the output. `--all` with `--limit 0` must not print a `more hidden` footer.
- If `--all` is false, the main-table footer must say both reasons:
  `... and N more hidden (A below 10 MB, B beyond --limit). Run with --all...`
- `PruneTmpTargets` must count `hidden` separately from `skipped`; the footer is `... and H smaller targets (B bytes)` where `H` is the number of non-shown, non-skipped targets. Skipped targets are reported in the cleanup summary, not the hidden footer.
- Add a `Population` line to the category summary:
  `Population: 804 entities · 150 cleanable · 654 protected · 4.6 GB unattributed`.

## Requirements

### R1 — Add the report package and pure builder

- Implement the types and functions in A3.
- `Build` groups by category and pool; process categories go to `PoolProcessSwap`, cache categories to `PoolTmpfs`.
- `Build` computes residuals and reasons from the scanner metadata.
- `Build` rejects an item with both `SwapBytes > 0` and `AllocatedBytes > 0` unless `Kind` says it is a hybrid; current data has no hybrids. Return an error in that case to prevent double counting.
- `Build` copies slices; callers can mutate their input without affecting the report.

### R2 — Allocated-size scan

- Implement A5 in `cache.go`.
- Add `SkipReason` counters to `ScanTmpDirectoriesIn`; return them via a new struct or a second return value. Preserve the existing function for callers, adding a `ScanTmpDirectoriesDetailed` seam.
- Do not change which top-level entries are selected in this goal; only change how their size is measured and how exclusions are counted.

### R3 — Wire the report through the app

- `CollectSwapItems` returns items plus a `ScanStats` struct (or `app.ScanResult`).
- `Run` calls `report.Build` once and passes the report to `PrintHeader`, `PrintActionableSection`, `PrintMainTable`, `PrintCategorySummary`, and `PrintQuickCommands`.
- Do not let display functions recompute totals with ad-hoc loops; they may only read report fields.
- Keep the existing function signatures where possible; add overloads or new functions rather than breaking tests unnecessarily. Existing tests may be updated.

### R4 — Unattributed and reconciliation rendering

- Implement A6.
- Always render the `unattributed`/`overlap` row for swap and /tmp.
- Add the reason breakdown line; if all reasons are zero, print `unattributed reasons: none`.
- Add a `reconciles` indicator: `OK` when `Reconciles() == nil`; on failure, print `FAIL: <error>` and a `WARN` badge, but do not hide the screen.
- The indicator must be testable without a live kernel: inject a `Reconciles` error via a seam or fixture.

### R5 — Cleanable split

- Implement A6's `Reclaimable: X tmpfs · Y swap` title.
- Remove the combined total from `PrintActionableSection`; if a combined number is still desired, put it in a secondary `upper bound` note.
- The dry-run summary and the audit header must use the same `Reclaim` values; add a test that asserts equality.

### R6 — Metric labels and documentation

- Rename strings exactly:
  - `Process Swap (RAM)` → `Process swap (VmSwap)`.
  - `/tmp Caches (tmpfs)` → `/tmp allocated (tmpfs)`.
  - `Size` in the process group → `Swap`; `Size` in the cache group → `Allocated`.
  - Header `RAM` → `RAM used` with a parenthetical `(total - available)` in help/docs; the header line may stay `RAM` if the docs define it, but the summary must not imply it is the process sum.
- Create `docs/metrics.md` with the A1 table, exact sources, units, additive flags, and caveats (shared swap, shmem, tmpfs swap, other UIDs, daemon prefixes, dotfiles, non-matching names, metadata, hard links).
- Add a `README.md` "What the numbers mean" section linking to it.
- Update `PrintUsage` to define `--all`/`--limit` interactions.

### R7 — Count and footer fixes

- Implement A7.
- Add a `Population` struct to the report and use it everywhere.
- Fix `PruneTmpTargets` hidden counting.
- Add `--all` semantics: disables aggregation and size filter; `limit=0` unless explicitly set.
- Add tests with synthetic items:
  - 20 items, 3 shells, 5 below 10 MB protected, limit 4: assert exact counts.
  - 10 targets where 3 are skipped: assert footer hidden count/bytes.
  - `--all --limit 0`: assert no hidden footer.
  - `--all --limit 3`: assert 3 shown and a footer with the correct remainder.

### R8 — Verification and evidence

- Add `goals/accounting-reconciliation/evidence.md` with the exact commands and outputs from the live host and fixtures.
- The evidence must include:
  - The before/after reconciliation line.
  - The exact byte arithmetic for swap and /tmp.
  - The `--all` before/after hidden counts.
  - Test/race/vet outputs.
  - One sparse-file size test.

## Non-Goals

- No PSS/RSS collection, no `/proc/[pid]/smaps_rollup`, no per-process RAM attribution. Those are separate metrics with different costs and semantics.
- No new `/var/tmp` scan (document it as unattributed until a separate goal).
- No JSON output (Goal 5).
- No change to process-kill eligibility, cache liveness classification, or table grouping (Goals 2–4).
- No removal of the `statfs`-based header or the `meminfo`-based header.
- No new external dependency; `go.mod` must remain dependency-free.

## Acceptance Criteria

- **AC-01**: `go test -race ./...` passes.
- **AC-02**: `go vet ./...` passes.
- **AC-03**: On a live run, `SwapUnattributed + ProcessSwapAttributed == SwapUsed` and `TmpfsUnattributed + TmpfsAttributed == TmpfsUsed` using exact `int64` bytes; the rendered line shows both terms and the sum.
- **AC-04**: No screen or output contains a negative byte value; `Overlap` is shown instead.
- **AC-05**: The `Cleanable` title contains `tmpfs` and `swap` and does not contain a single combined total without the `upper bound` note.
- **AC-06**: `grep -R "Process Swap (RAM)" .` returns no matches outside `goals/` and `CHANGELOG.md`.
- **AC-07**: `docs/metrics.md` defines every number in A1 and is linked from `README.md`.
- **AC-08**: `swapaudit --no-color --ascii --all --limit 0` prints no `more hidden` footer and includes at least one item below 10 MB that was previously hidden.
- **AC-09**: `PruneTmpTargets` footer count equals the number of non-shown, non-skipped targets for a fixture with skips; the byte total equals the sum of those targets.
- **AC-10**: All rendered lines are exactly `term.TermWidth()` visible columns at widths 80, 100, and 120 for the header, verdict, reconciliation line, tables, and population lines.
- **AC-11**: A 100-fixture randomized property test proves the residual invariant; no fixture produces an unclamped negative.
- **AC-12**: The sparse-file test proves `AllocatedBytes` is used for display/reclaim while `ApparentBytes` retains the apparent size.
- **AC-13**: `scripts/check.sh --full` passes.
- **AC-14**: `go.mod` has no new `require` entries.

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -run 'Test(Report|Reconcile|Population|Allocated|CleanableSplit)' -v ./internal/report ./internal/cache ./internal/app
./swapaudit --no-color --ascii --limit 12 > /tmp/swapaudit-before.txt
./swapaudit --no-color --ascii --all --limit 0 > /tmp/swapaudit-all.txt
grep -c "more hidden" /tmp/swapaudit-all.txt            # expect 0
grep -R "Process Swap (RAM)" . --exclude-dir=.git --exclude-dir=goals || true
./scripts/check.sh --full
```

Add a small `scripts/check-accounting.sh` only if it can parse the report without brittle formatted-output parsing. Prefer Go tests for exact-byte invariants; use shell only for the `--all`/label assertions.

## Test Matrix

| Test | File | Proves |
| --- | --- | --- |
| `TestBuildReconcilesSwap` | `internal/report/accounting_test.go` | Exact swap residual, reasons, no negative |
| `TestBuildReconcilesTmpfs` | same | Exact tmpfs residual, reasons, metadata residual |
| `TestBuildOverlap` | same | Attributed > total yields `Overlap`, not negative |
| `TestBuildNoHybridItems` | same | A hybrid item returns an error; no double count |
| `TestBuildCategoriesDeterministic` | same | Stable order and shares |
| `TestBuildBiggestWin` | same | Largest cleanable, deterministic tie-break |
| `TestAllocatedBytesSparseFile` | `internal/cache/cache_test.go` | `AllocatedBytes` vs `ApparentBytes` |
| `TestSkipReasons` | `internal/cache/cache_test.go` | UID/daemon/dotfile/symlink/non-matching counters |
| `TestAllModeShowsSmallItems` | `internal/app/display_test.go` | `--all` bypasses the 10 MB filter |
| `TestAllModeNoHiddenFooter` | same | `--all --limit 0` footer absent |
| `TestPopulationLine` | same | Raw/aggregated/shown/hidden counts |
| `TestPruneFooterWithSkips` | `internal/cleaner/cleaner_test.go` | Hidden count excludes skipped; bytes match |
| `TestCleanableSplit` | `internal/app/display_test.go` | Title has both pools; no combined total |
| `TestReconciliationLine` | same | Exact rendered arithmetic |
| `TestNoProcessSwapRAMLabel` | same | Banned label absent |
| `TestTableWidthInvariant` | `internal/term/term_test.go` | Every line exactly term width |
| `TestRandomFixturesNoNegativeResidual` | `internal/report/accounting_test.go` | 100 randomized fixtures |

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| `VmSwap` sums can double-count shared swapped pages | Report `ProcessSwapAttributed` as "attributed, may include shared pages"; use `Overlap` when it exceeds the system total; never claim it is exact physical memory |
| `statfs` and the walk race (files created/deleted during the scan) | Compute residuals from one snapshot; show `Overlap` or a small `scan-race` reason; do not fail the audit |
| Allocated blocks are not portable across filesystems | Linux-only goal; use `Stat_t.Blocks * 512`; fallback to apparent size and count `fallback` |
| Directory metadata and hard links make the residual nonzero forever | Document `metadata/hardlink` in the reason breakdown; do not chase exact inode accounting in this goal |
| The report package duplicates display logic | `Build` is the only sum; display reads fields; add a grep/test that forbids `+= it.SwapBytes` outside `internal/report` |
| Tests become brittle against formatted output | Test exact fields via `report.Build`; only a few golden tests assert the rendered line |
| `--all` changes output volume dramatically | Keep `--limit` explicit; add a population line; document that `--all` means all items, not all columns |

## Rollback

- Revert the implementation commit. No data migration, no persistent state.
- If the report package causes churn, keep the `AuditReport` types but revert the display wiring; the accounting tests remain useful.
- Do not reintroduce a combined `Cleanable` total as the primary number.

## References

- UI review screenshot and claim list, 2026-10-19 (this session).
- Verification report: `docs/reviews/2026-10-19-swapaudit-ui-review.md`.
- `internal/proc/proc.go:54-55`, `:68-73` — memory and tmpfs headers.
- `internal/cache/cache.go:44`, `:51-70`, `:83`, `:91-150` — scan population and size.
- `internal/app/display.go:84`, `:117-174`, `:251-304`, `:316-428` — sums, shares, labels, footers.
- `internal/app/app.go:63-77` — swap-only process inclusion.
- `internal/cleaner/cleaner.go:187-189`, `:358-370` — hidden count and split cleanup summary.
- `internal/model/model.go:19-60` — item and system types.
- Live capture: `$COMMANDCODE_SCRATCHPAD/audit.txt` (this session) and independent `/proc` sums.
