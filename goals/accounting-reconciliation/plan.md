# Plan: Make Swap and tmpfs Accounting Reconcile with Every Header Total

## Execution Steps

### Phase 1 — Capture the baseline and freeze the contract

- [x] Run `./swapaudit --no-color --ascii --limit 12` and save the exact output to `goals/accounting-reconciliation/evidence.md`.
- [x] Capture the independent totals:
  - `grep -E '^(SwapTotal|SwapFree|MemTotal|MemAvailable):' /proc/meminfo`
  - `df -B1 /tmp`
  - `awk '/^VmSwap:/{s+=$2} END{printf "%d kB\n", s}' /proc/[0-9]*/status`
- [x] Compute the exact residual for swap and /tmp in a scratch file; record it before any code change.
- [x] Capture the `--all --limit 0` output and record how many items are still hidden.
- [x] Run `go test -race ./...` and `go vet ./...`; record the baseline.
- [x] Add the accounting contract (A1–A7 from `goal.md`) to `docs/metrics.md` before implementation so reviewers can challenge definitions first.

### Phase 2 — Add report types and the pure builder

- [x] Create `internal/report/accounting.go` with the types from A3.
- [x] Implement `Build(now, system, items, thresholds) AuditReport` as a pure function.
- [x] Implement `Reconciles()`, `BiggestWin()`, `CleanableCount()`.
- [x] Add `ReasonBytes` sorting and deterministic category ordering.
- [x] Reject hybrid items (`SwapBytes > 0 && AllocatedBytes > 0`) unless `Kind == "hybrid"`; return an error.
- [x] Write `TestBuildReconcilesSwap`, `TestBuildReconcilesTmpfs`, `TestBuildOverlap`, `TestBuildNoHybridItems`, `TestBuildCategoriesDeterministic`, `TestBuildBiggestWin`, `TestRandomFixturesNoNegativeResidual`.
- [x] Run `go test -race ./internal/report`.

### Phase 3 — Allocated-size cache accounting

- [x] Add `AllocatedBytes`, `ApparentBytes`, `Kind`, `ID`, `Evidence`, `OpenHandles` to `model.SwapItem`.
- [x] Set `SizeBytes = AllocatedBytes` for cache items.
- [x] Implement `allocatedSize(path)` using `syscall.Stat_t.Blocks * 512`; fall back to `info.Size()` and increment a `fallback` counter.
- [x] Include directory inode allocated blocks in the entry total; exclude the scan root.
- [x] Add `ScanStats`/`SkipReason` counters in `cache.ScanTmpDirectoriesIn`.
- [x] Add a `ScanTmpDirectoriesDetailed` seam that returns items plus stats; keep `ScanTmpDirectories` as a wrapper for existing callers.
- [x] Tests:
  - `TestAllocatedBytesSparseFile` (1 GiB apparent, ≤64 KiB allocated).
  - `TestSkipReasons` (other UID, daemon prefix, dotfile, symlink, non-matching).
  - `TestDirectoryMetadataCounted` if feasible with a temp dir.
- [x] Run `go test -race ./internal/cache`.

### Phase 4 — Wire the report through app and display

- [x] Change `CollectSwapItems` to return `(items []model.SwapItem, stats app.ScanStats)` or a `ScanResult`.
- [x] Call `report.Build` once in `Run`.
- [x] Pass `AuditReport` to `PrintHeader`, `PrintActionableSection`, `PrintMainTable`, `PrintCategorySummary`, and the verdict renderer.
- [x] Remove ad-hoc sums from display; display reads `report` fields only.
- [x] Add a `report.Reconciles()` indicator to the header/verdict.
- [x] Update all tests and call sites compile-first.
- [x] Run `go test -race ./internal/app`.

### Phase 5 — Render residuals, split reclaim, and population

- [x] Add `Swap: X attributed + Y unattributed = Z used` and `/tmp: ...` lines.
- [x] Add the `unattributed reasons:` line with only nonzero reasons.
- [x] Always render the residual rows in the category table, including 0 B.
- [x] Replace `Process Swap (RAM)` with `Process swap (VmSwap)`.
- [x] Replace `/tmp Caches (tmpfs)` with `/tmp allocated (tmpfs)`.
- [x] Replace the `Cleanable` title with `Reclaimable: X tmpfs · Y swap (upper bound)`.
- [x] Add `Share (attributed)` and a system-share secondary value.
- [x] Add the `Population:` lines to the actionable and main tables.
- [x] Fix the main-table footer to break out `below 10 MB` vs `beyond --limit`.
- [x] Implement `--all` semantics: no aggregation, no 10 MB filter, `limit=0` unless explicit.
- [x] Fix `PruneTmpTargets` hidden counting (count only non-shown, non-skipped; bytes match).
- [x] Add golden tests for a clean host, an unattributed host, an overlap host, and a zero-cleanable host.

### Phase 6 — Documentation and CLI text

- [x] Write `docs/metrics.md` with A1 and the caveats.
- [x] Add `README.md` "What the numbers mean".
- [x] Update `PrintUsage` for `--all`/`--limit` semantics and the metric labels.
- [x] Update `CHANGELOG.md` under `Changed` and `Fixed`.
- [x] Add a grep sentry to `scripts/check.sh` for `Process Swap (RAM)` outside allowed paths.

### Phase 7 — Full verification and evidence

- [x] Run `gofmt -l .` (empty).
- [x] Run `go vet ./...`.
- [x] Run `go test -race ./...`.
- [x] Run `go test -run 'Test(Report|Reconcile|Population|Allocated|CleanableSplit)' -v ./internal/report ./internal/cache ./internal/app`.
- [x] Run `./scripts/check.sh --full`.
- [x] Run a live audit and paste the before/after reconciliation lines into `evidence.md`.
- [x] Run `./swapaudit --no-color --ascii --all --limit 0` and confirm no `more hidden` footer and at least one previously hidden small item appears.
- [x] Record `go.mod` diff (must be empty).
- [x] Record the exact residual arithmetic and the fixture tests.

### Phase 8 — Attribution commit

- [x] Commit with a subject containing `(goals/accounting-reconciliation/goal.md)` or `Goal: accounting-reconciliation`, using `fix:` or `feat:`.
- [x] Run `sila goals --project=/home/jad/scripts/active/swapaudit`.
- [x] Do not push to a remote unless the user explicitly authorizes it.

## Tests to Add

| Test | File | What it proves |
| --- | --- | --- |
| `TestBuildReconcilesSwap` | `internal/report/accounting_test.go` | Exact swap residual and reasons |
| `TestBuildReconcilesTmpfs` | same | Exact tmpfs residual and reasons |
| `TestBuildOverlap` | same | Attributed > total yields overlap, never negative |
| `TestBuildNoHybridItems` | same | No double counting |
| `TestBuildCategoriesDeterministic` | same | Stable ordering |
| `TestBuildBiggestWin` | same | Deterministic largest cleanable |
| `TestRandomFixturesNoNegativeResidual` | same | 100 randomized fixtures |
| `TestAllocatedBytesSparseFile` | `internal/cache/cache_test.go` | Allocated vs apparent size |
| `TestSkipReasons` | same | Every scan exclusion is counted |
| `TestAllModeShowsSmallItems` | `internal/app/display_test.go` | `--all` bypasses the 10 MB filter |
| `TestAllModeNoHiddenFooter` | same | `--all --limit 0` footer absent |
| `TestPopulationLine` | same | Exact population arithmetic |
| `TestPruneFooterWithSkips` | `internal/cleaner/cleaner_test.go` | Hidden count excludes skipped |
| `TestCleanableSplit` | `internal/app/display_test.go` | Both pools, no misleading total |
| `TestReconciliationLine` | same | Exact rendered arithmetic |
| `TestNoProcessSwapRAMLabel` | same | Banned label absent |
| `TestTableWidthInvariant` | `internal/term/term_test.go` | All lines exactly term width |
| `TestShareDenominator` | `internal/app/display_test.go` | Attributed vs system share labels |

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -run 'Test(Report|Reconcile|Population|Allocated|CleanableSplit)' -v ./internal/report ./internal/cache ./internal/app
./swapaudit --no-color --ascii --limit 12 > /tmp/swapaudit-after.txt
./swapaudit --no-color --ascii --all --limit 0 > /tmp/swapaudit-all-after.txt
grep -c "more hidden" /tmp/swapaudit-all-after.txt || true   # expect 0
grep -R "Process Swap (RAM)" . --exclude-dir=.git --exclude-dir=goals || true
./scripts/check.sh --full
git diff -- go.mod   # expect empty
```

## Rollback

- Revert the implementation commit. No persistent state or migration.
- If the report package is too large, keep the `AuditReport` types and the pure tests, and revert only the display wiring.
- Do not restore a combined `Cleanable` total as the primary headline.

## Completion Evidence

`goals/accounting-reconciliation/evidence.md` must contain:

1. Baseline and after screenshots/text with exact byte arithmetic.
2. The independent `/proc` and `statfs` commands and their outputs.
3. The `--all` before/after hidden counts.
4. The sparse-file test output.
5. `go test -race`, `go vet`, and `scripts/check.sh --full` outputs.
6. Any deviation from this plan and why.
