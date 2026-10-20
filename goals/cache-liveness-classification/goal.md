# Goal: Classify /tmp Caches by Liveness and Ownership, Not Just mtime

## Chain of Custody (Isnad)

- **Registered By**: UI review follow-up, 2026-10-19
- **Spec Audited By**: CommandCode, 2026-10-20 (acceptance criteria checked against tests and evidence)
- **Implemented By**: CommandCode, 2026-10-20 (see `evidence.md`)
- **Mystery Audited By**: Evidence certification, 2026-10-20 (see `certification.json`; independent mystery audit not run)

## Goal Description

The current `/tmp` classifier has only two states: "stale enough to delete" and "active". "Active" means only that the newest mtime is younger than `--max-age` (default 12h). It does not mean a process has the cache open, that a build is running, or that the cache is in use. On the reviewed host, 434 entities and 3.0 GB were labelled `active-caches` with `0 B` actionable; several had `Age == Idle`, proving they had never been touched after creation.

This goal replaces the mtime-only label with evidence-based categories:

- `recent-caches` — younger than `--max-age`, with no stronger evidence (the honest rename of `active-caches`).
- `in-use-caches` — an open file descriptor, mapped file, or process cwd points inside the cache; protected unconditionally.
- `orphaned-caches` — no open handle, no cwd, no mapped file, and idle longer than `--orphan-cache-age` (default 1h); visible and opt-in cleanable.
- `untouched-caches` / `abandoned-runs` — existing stale categories, now allocated-size based and re-checked before deletion.

The goal also builds an open-handle index from `/proc` without `lsof`, `fuser`, or any external dependency, and adds a fail-closed recheck immediately before deletion.

## Business Context

- This is the second priority in the UI review: "The largest bucket (4.0 GB of hours-idle caches) is labelled safe and unexamined."
- The current classifier can both overstate safety (a cache in use is "active" and protected, which is safe but invisible) and understate opportunity (a 6-hour-old orphaned `go-build` directory is "active" and never offered for cleanup).
- The reviewed host has a 3.0 GB `active-caches` bucket with 434 entities; three `go-build*` directories alone account for ~1.9 GB and have `Age == Idle`, so they were never touched after creation.
- The product's promise is "safe cleanup". A category named `active` that is determined only by mtime is a trust defect, even if no deletion occurs.
- This goal must land before the v0.1.0 release; it is the largest reclaim opportunity and the largest classification honesty gap.

## Verified Findings (2026-10-19, current tree)

### F1 — "active" is an mtime bucket, not liveness

- `internal/cache/cache.go:137-150`:
  ```go
  age := now.Sub(fi.ModTime())
  lastActive := now.Sub(newestMtime)
  cleanable := lastActive > maxAge
  if cleanable { ... } else { cat = model.CatActiveCache }
  ```
- There is no `open(2)`, `/proc/[pid]/fd`, `/proc/[pid]/maps`, `/proc/[pid]/cwd`, or process-ownership check.
- `internal/model/model.go:13` defines `CatActiveCache Category = "active-caches"`.
- Live output:
  ```text
  active-caches | 434 | 3.0 GB | [#######...] 72.4% | 0 B
  go-build873559002 | active-caches | 657.8 MB | 6h45m | 6h45m
  go-build2547211510 | active-caches | 627.0 MB | 3h37m | 3h37m
  go-build1434250357 | active-caches | 604.4 MB | 3h12m | 3h11m
  ```
  Age equals Idle for the first two, so they were never modified after creation; they are recent, not active.

### F2 — The scan has ownership but not liveness

- `internal/cache/cache.go:44` captures `currentUid`.
- `internal/cache/cache.go:82-89` checks the top-level `Stat_t.Uid` and rejects symlinks/sockets/pipes.
- Ownership proves who owns the directory; it does not prove whether the owner is using it.
- `internal/cache/cache.go:51-70` skips known daemon prefixes; those directories never appear in the table, so their activity cannot be inspected.

### F3 — Size is apparent, not allocated

- `internal/cache/cache.go:120-135` sums `info.Size()` in `WalkDir`.
- A sparse file can inflate the displayed reclaimable size relative to the tmpfs blocks that `statfs` reports.
- Goal 1 adds `AllocatedBytes` and makes `SizeBytes` allocated; this goal consumes that field for classification and deletion.

### F4 — Deletion does not re-check liveness

- `internal/cleaner/cleaner.go:148-171` re-checks lstat, symlink, UID, and then calls `os.RemoveAll`.
- There is no open-handle check, no cwd check, and no "opened since scan" reason.
- A build that starts after the scan can have its cache deleted underneath it.

### F5 — The category list has no honest place for "young but unused"

- `internal/app/display.go:391-395` renders only `untouched-caches`, `abandoned-runs`, and `active-caches`.
- There is no row or action for a cache that is too young for `--max-age` but has no user.
- `internal/app/display.go:448-458` quick commands do not mention caches by liveness.

### F6 — The same problem exists in cleanup output

- `internal/cleaner/cleaner.go:101-190` prints only `pruned`/`would prune`; it does not say whether the target was open at deletion time, because it never checks.

## Dependencies & Execution Order

- **Mode**: Dependent (consumes the `AllocatedBytes`, `SkipReason`, and `AuditReport` contracts from Goal 1)
- **Depends On**: `accounting-reconciliation`
- **Sequence**: 2
- **Shape**: ship
- **Tier**: immediate
- **Blocks**: `orphan-visibility-safety`, `output-information-architecture`

## Files to Touch

- `internal/model/model.go` — new categories, `CacheKind`, `OpenHandles`, `CwdInside`, `MappedInside`, `Evidence`, `LastActiveKnown`
- `internal/model/model_test.go`
- `internal/cache/cache.go` — liveness index integration, classification precedence, evidence
- `internal/cache/liveness.go` (new) — `/proc` open-handle index
- `internal/cache/liveness_test.go` (new)
- `internal/cache/cache_test.go`
- `internal/cleaner/cleaner.go` — recheck seam, new skip reasons, orphaned-cache group
- `internal/cleaner/cleaner_test.go`
- `internal/app/app.go` — new flags, `ResolveCleanupFlags` fields
- `internal/app/app_test.go`
- `internal/app/display.go` — category rows, action text, threshold header, evidence
- `internal/app/display_test.go`
- `internal/triage/triage.go` — no change unless cache grouping needs a helper
- `README.md`, `CHANGELOG.md`, `docs/cache-liveness.md` (new), `docs/metrics.md` (cross-reference)
- `goals/cache-liveness-classification/*` (this package)

## Data and Classification Contract (over-specified)

### C1 — Categories

```go
const (
    CatUntouchedCache  Category = "untouched-caches"
    CatAbandonedRun    Category = "abandoned-runs"
    CatRecentCache     Category = "recent-caches"    // replaces active-caches
    CatInUseCache      Category = "in-use-caches"
    CatOrphanedCache   Category = "orphaned-caches"
    // Keep CatActiveCache as a deprecated alias only if a compile-time caller needs it.
    // Its string value must not be "active-caches"; use CatRecentCache's value.
)
```

- `CatActiveCache` may remain as a Go alias (`CatActiveCache = CatRecentCache`) for one release, but the rendered/JSON string is `recent-caches`. No output may contain `active-caches` except the CHANGELOG migration note.
- `orphaned-caches` is a single category with a `Kind` subfield:
  ```go
  type CacheKind string
  const (
      CacheBuild   CacheKind = "build"
      CacheRun     CacheKind = "run"
      CacheGeneric CacheKind = "generic"
  )
  ```
- `CacheBuild`: `go-build*`, `cargo-*`, `rustup-update*`, `cargo-install*`, `npm-*`, `sila-cache-*`, and the existing build-cache detection after refinement. `CacheRun`: `Test*`, `test*`, `test-bwrap*`, `tsx-*`, `tmp-*`, `temp-*`, plus `--tmp-pattern`. `CacheGeneric`: a directory that matched only the broad `cache` keyword or a user-supplied pattern that is not clearly build/run.

### C2 — Classification precedence

Apply exactly in this order; the first match wins:

1. **Special/ineligible** (skip entirely, counted in `SkipReason`): dotfile, `systemd-*`, `pulse-*`, daemon prefix, symlink, socket/pipe/device, other UID, non-matching name.
2. **In use**: `OpenHandles > 0 || CwdInside || MappedInside` → `CatInUseCache`, `Cleanable=false`. Even if stale beyond `--max-age`, a cache with an open handle is never cleanable.
3. **Unknown evidence**: `EvidenceIncomplete` (open-index truncated or a same-UID `/proc` read failed) → `CatInUseCache` or a dedicated `evidence-unknown` status, `Cleanable=false`. Fail closed.
4. **Stale**:
   - `Idle > MaxAge` and `Kind == CacheBuild` → `CatUntouchedCache`, `Cleanable=true`.
   - `Idle > MaxAge` and `Kind == CacheRun` → `CatAbandonedRun`, `Cleanable=true`.
   - `Idle > MaxAge` and `Kind == CacheGeneric` → `CatAbandonedRun` only if `--allow-generic-caches`; otherwise `CatRecentCache` with a note. (Generic caches are the riskiest; the goal can also keep them in `recent-caches` and never offer them. Pick one and test it. Default: generic remains `recent-caches` and is never cleanable in this goal.)
5. **Young but unused**: `Idle > OrphanCacheAge` and `Kind != CacheGeneric` (or generic allowed by flag) and no in-use evidence → `CatOrphanedCache`, `Cleanable` only when `--clean-orphaned-caches`/`--include-orphaned-caches` is set; otherwise `Cleanable=false` with `Evidence: "orphaned; use --clean-orphaned-caches"`.
6. **Recent**: everything else → `CatRecentCache`, `Cleanable=false`.

Additional rules:

- `Idle` is `now - newestMtime` (including nested writes), not `now - root mtime`. `Age` remains `now - root mtime` for context.
- A directory with `SizeBytes == 0` can still be `orphaned-caches`; removing empty dirs is safe but yields no memory. Show `0 B` and allow cleanup.
- A top-level file (not a directory) is classified by the same rules; `newestMtime` = file mtime.
- `MappedInside`: a file-backed mapping under the cache path, read from `/proc/[pid]/maps`. Anonymous mappings are irrelevant.
- `CwdInside`: `/proc/[pid]/cwd` resolves to the cache dir or a descendant.
- `OpenHandles`: count of distinct file descriptors whose resolved path is the cache dir or a descendant. The top-level directory itself being open counts.
- `Evidence`: a short deterministic string used by the UI/JSON, for example `idle=6h45m handles=0 cwd=no kind=build`.
- If `--max-age <= --orphan-cache-age`, `orphaned` is unreachable; reject the configuration with exit 2 or clamp and warn. Define: `--orphan-cache-age` must be <= `--max-age`; otherwise exit 2 with a clear error. Add a test.

### C3 — Open-handle index

```go
// internal/cache/liveness.go
type OpenIndex struct {
    Paths       map[string]int // cleaned absolute directory/file path -> handle count
    Cwds        map[string]int
    Mapped      map[string]int
    Truncated   bool
    Errors      int
    ScannedPIDs int
    ScannedFDs  int
}

// BuildOpenIndex scans /proc for open descriptors, cwds, and file-backed mappings
// whose path is under one of roots. It never follows symlinks outside /proc.
func BuildOpenIndex(procRoot string, roots []string) OpenIndex

func (idx OpenIndex) Lookup(path string) (handles, cwds, mapped int)
func (idx OpenIndex) Complete() bool
```

- `roots` defaults to `[]string{"/tmp"}`, plus `os.TempDir()` if different, plus `/var/tmp` only if the scanner is extended; this goal keeps `/tmp`.
- For each open path, add the cleaned path and every ancestor up to (but not including) the root to `Paths`. This makes a directory lookup O(1) and correctly marks a cache when a nested file is open.
- `/proc/[pid]/fd` readlink errors: `EACCES`, `ENOENT`, `ESRCH` are counted; do not abort the whole scan. If the PID belongs to the current UID and its fd table is unreadable, set `Errors++` and treat evidence as incomplete.
- `/proc/[pid]/cwd` and `/proc/[pid]/maps` are scanned with the same rules. `/proc/[pid]/maps` path parsing must handle ` (deleted)` suffixes and paths containing spaces; use the last ` `-delimited path field and strip ` (deleted)`.
- Cap the scan at `maxScannedPIDs = 10000` and `maxScannedFDs = 200000`; set `Truncated=true` if either cap is hit. A truncated index is incomplete; orphaned caches are not cleanable.
- Do not use `lsof`, `fuser`, `find`, or external commands. Do not follow `/proc/[pid]/root` or `/proc/[pid]/cwd` outside the roots for the index (only the path string matters).
- The function accepts `procRoot` so tests can build a fake `/proc` tree with symlinks. Production passes `/proc`.
- Performance target: <=150 ms on a host with 2,000 processes and 50,000 fds, with no allocation explosion. Add a benchmark; record the result, not a hard CI gate unless stable.

### C4 — Deletion recheck

```go
// internal/cleaner/cleaner.go
var buildOpenIndex = cache.BuildOpenIndex

type CacheRecheck struct {
    Complete bool
    Handles  int
    Cwds     int
    Mapped   int
    Err      error
}

func RecheckCache(path string) CacheRecheck
```

- `PruneTmpTargets` accepts recheck behavior through `CleanupOptions`:
  ```go
  type CleanupOptions struct {
      DryRun    bool
      Yes       bool
      AllUsers  bool
      PathWidth int
      Prefix    string
      OpenCheck OpenCheckMode // "group" (default for untouched/abandoned) or "target" (default for orphaned)
  }
  ```
  For orphaned caches, always use `target` (a fresh index immediately before `RemoveAll`). For untouched/abandoned, `group` is allowed; document the race.
- Skip reasons:
  - `open-handle` — handles > 0.
  - `cwd-inside` — cwd > 0.
  - `mapped-inside` — mapped > 0.
  - `evidence-incomplete` — index truncated or same-UID read error.
  - `opened-since-scan` — a target-level recheck finds a handle that the group scan did not.
- In dry-run, perform the same recheck (with the same mode) and print `[dry-run] SKIP ... reason`. Dry-run must not call `RemoveAll`.
- The cleanup summary counts each reason exactly once per target. Add to `CleanupStats`.

### C5 — Flags

| Flag | Default | Semantics |
| --- | --- | --- |
| `--orphan-cache-age <duration>` | `1h` | Idle threshold for `orphaned-caches`; must be <= `--max-age` |
| `--clean-orphaned-caches` | `false` | Prune `orphaned-caches` only; never implies agents |
| `--include-orphaned-caches` | `false` | Include orphaned-cache pruning when `--clean-all` is set |
| `--allow-generic-caches` | `false` | Permit `CacheGeneric` classification/cleanup; default false |

- `--clean-all` continues to mean untouched + abandoned + dormant. It does **not** include orphaned caches or orphaned agents.
- `--include-orphaned-caches` without `--clean-all` or `--clean-orphaned-caches` is a warning/no-op.
- `--clean-orphaned-caches` alone works and asks for confirmation unless `--yes`/`--dry-run`.
- `--clean-tmp` remains untouched + abandoned; it does not include orphaned caches.
- `ResolveCleanupFlags` gets `OrphanedCaches` and `IncludeOrphanedCaches` fields; table tests cover all relevant combinations.

### C6 — Display

- The `/tmp` group rows become:
  `untouched-caches`, `abandoned-runs`, `orphaned-caches`, `in-use-caches`, `recent-caches`.
- `orphaned-caches` is always rendered, including `0` and `0 B`.
- `in-use-caches` is always rendered if any exist; it shows `0 B` actionable and a `Protected` note.
- The group header shows thresholds: `/tmp allocated (tmpfs) · stale > 12h · orphan-cache > 1h`.
- The action column:
  - `untouched-caches` → `Prune untouched cache`
  - `abandoned-runs` → `Prune abandoned run`
  - `orphaned-caches` → `Prune orphaned cache` when cleanable; `Orphaned (opt-in)` when not
  - `in-use-caches` → `In use`
  - `recent-caches` → `Recent`
- Add a compact evidence line under the table:
  `Cache liveness: 12 in-use · 84 orphaned (1.8 GB) · 338 recent · 0 evidence-incomplete`.
- `--explain-caches` prints per-cache evidence for up to `--limit` items (or all with `--all`), including handles/cwd/mapped counts.
- The main table's category column uses the new strings; no `active-caches` may remain.

### C7 — Scanner metadata

- Extend `cache.ScanStats` (from Goal 1) with:
  - `Scanned int`, `InUse int`, `Orphaned int`, `Recent int`, `EvidenceIncomplete int`, `OpenIndexTruncated bool`, `OpenIndexErrors int`, `OpenIndexPIDs int`, `OpenIndexFDs int`.
- The report's `Population` includes these. Goal 1's `AuditReport` can be extended without a schema break; Goal 5 serializes them.

## Requirements

### R1 — Add categories and precedence

- Implement C1/C2 in `cache.go`.
- Refactor the current broad `isBuildCache` check (`strings.HasPrefix(name, "go-build") || strings.Contains(lower, "cache")`) into `CacheKind` helpers; keep `DefaultTmpPatterns`.
- Ensure the selection of top-level entries is unchanged except for liveness metadata.
- Tests for every precedence branch and every `CacheKind`.

### R2 — Build the open-handle index

- Implement C3 in `internal/cache/liveness.go`.
- Add a fake `/proc` fixture builder used by tests: PIDs with `fd/0` symlinks, `cwd`, and `maps` files.
- Test ancestors, nested files, cwd, maps, deleted-suffix paths, permission errors, caps, and `Complete()`.
- Benchmark `BuildOpenIndex` on the live host; record the result in evidence.

### R3 — Wire classification into the scan

- Build one `OpenIndex` per audit before scanning `/tmp`, pass it to `ScanTmpDirectoriesIn`, and use `Lookup`.
- Add `--orphan-cache-age` validation.
- Add `--allow-generic-caches` behavior.
- Ensure `orphaned-caches` is not cleanable unless the flag is set; the `Cleanable` field must reflect the effective decision, not just the category.

### R4 — Add cleanup group and recheck

- Add the orphaned-cache group to `ExecuteCleanup`.
- Add `RecheckCache` and the `OpenCheckMode` seam.
- Recheck before `RemoveAll`; skip with the C4 reasons.
- Dry-run performs the same recheck but does not delete.
- Add `CleanupStats` reasons and summary tests.

### R5 — Display and help

- Implement C6.
- Add the threshold header and liveness line.
- Add `--explain-caches`.
- Update `PrintQuickCommands`, `PrintUsage`, README, and `docs/cache-liveness.md`.
- Add a docs table that explains "recent" vs "in use" vs "orphaned" vs "stale".

### R6 — Tests and safety

- Tests must not delete a real user cache. Use temp dirs under `t.TempDir()` and a fake `/proc` root.
- Tests must not depend on the live `/proc` for deterministic classification; use the seam.
- A live-host verification may be manual and read-only (`--dry-run`); record it in evidence.
- Add a benchmark for the open index; a regression >2× the baseline requires a note, not a hard failure.
- No external dependency; no cgo.

## Non-Goals

- No JSON output (Goal 5).
- No scan of `/var/tmp` in this goal; document it as unscanned/unattributed.
- No `lsof`/`fuser` integration.
- No deletion of caches owned by another UID; the current-UID rule remains.
- No automatic deletion of `in-use-caches` or `recent-caches`.
- No agent-classification changes (Goal 3).
- No table grouping/truncation redesign (Goal 4).

## Acceptance Criteria

- **AC-01**: `go test -race ./...` and `go vet ./...` pass.
- **AC-02**: `grep -R "active-caches" internal` returns no matches; the rendered label is `recent-caches`.
- **AC-03**: A fixture with a 6h-old `go-build` dir, no handles, `--orphan-cache-age=1h` is `orphaned-caches`; with an open fd it is `in-use-caches` and not cleanable.
- **AC-04**: A fixture with a 13h-old `go-build` dir, no handles is `untouched-caches` and cleanable (existing behavior preserved).
- **AC-05**: A fixture with an open fd in a 13h-old dir is `in-use-caches` and not cleanable (stronger than the old behavior).
- **AC-06**: `orphaned-caches` is not cleanable by default; it becomes cleanable only with `--clean-orphaned-caches` or `--include-orphaned-caches` + `--clean-all`.
- **AC-07**: `--clean-all` does not prune orphaned caches; `--include-orphaned-caches` does.
- **AC-08**: Deletion rechecks open handles; a handle that appears after the group scan causes `opened-since-scan` and no deletion.
- **AC-09**: A truncated/error index causes `evidence-incomplete` and no orphaned-cache deletion.
- **AC-10**: Dry-run never calls `os.RemoveAll`; it prints the same skip reasons.
- **AC-11**: `--orphan-cache-age > --max-age` exits 2 with a clear error.
- **AC-12**: The live dry-run reclassifies at least the three largest `go-build*` dirs as `orphaned-caches` if they have no handles and idle >1h; record the before/after counts. If a handle exists, record `in-use-caches` instead.
- **AC-13**: Open-index benchmark completes without timeout and records PIDs/fds scanned; no external command is invoked.
- **AC-14**: `scripts/check.sh --full` passes; `go.mod` unchanged.

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -race -run 'Test(BuildOpenIndex|Classify.*Cache|OrphanedCache|InUseCache|EvidenceIncomplete|OpenedSinceScan|ResolveCleanupFlags)' ./internal/cache ./internal/cleaner ./internal/app
go test -bench 'BenchmarkBuildOpenIndex' -benchtime=1x ./internal/cache
./swapaudit --no-color --ascii --dry-run --clean-orphaned-caches > /tmp/orphan-cache-dryrun.txt
./swapaudit --no-color --ascii --dry-run --clean-all > /tmp/clean-all-dryrun.txt
grep -c "Orphaned Cache" /tmp/clean-all-dryrun.txt    # expect 0 without include flag
grep -c "Orphaned Cache" /tmp/orphan-cache-dryrun.txt # expect 1
grep -R "active-caches" internal --include='*.go' || true
./scripts/check.sh --full
```

## Test Matrix

| Test | File | Proves |
| --- | --- | --- |
| `TestBuildOpenIndexFakeProc` | `internal/cache/liveness_test.go` | fd/cwd/maps indexing and ancestors |
| `TestOpenIndexCaps` | same | `Truncated` and `Complete()` |
| `TestOpenIndexPermissionErrors` | same | Errors counted, no panic |
| `TestOpenIndexDeletedSuffix` | same | `(deleted)` paths parsed |
| `TestClassifyCachePrecedence` | `internal/cache/cache_test.go` | Every C2 branch |
| `TestClassifyCacheKind` | same | build/run/generic |
| `TestOrphanedCacheOptIn` | same | Cleanable only with flags |
| `TestInUseCacheProtected` | same | Open handle/cwd/mapped always protected |
| `TestEvidenceIncompleteProtected` | same | Fail closed |
| `TestOrphanAgeValidation` | `internal/app/app_test.go` | `--orphan-cache-age > --max-age` exits 2 |
| `TestResolveCleanupFlagsOrphanedCaches` | same | Flag combinations |
| `TestPruneSkipsOpenHandle` | `internal/cleaner/cleaner_test.go` | `open-handle` reason |
| `TestPruneSkipsOpenedSinceScan` | same | Target-level recheck |
| `TestPruneSkipsEvidenceIncomplete` | same | Fail closed |
| `TestDryRunNoRemove` | same | No mutation |
| `TestCacheLivenessDisplay` | `internal/app/display_test.go` | New rows, zero row, threshold header |
| `TestNoActiveCacheLabel` | same | Banned string absent |
| `BenchmarkBuildOpenIndex` | `internal/cache/liveness_test.go` | Performance evidence |

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Reclassifying 3 GB of caches as `orphaned` could delete a cache a build is about to reuse | Opt-in flag; open-handle/cwd/mapped checks; target-level recheck; dry-run first |
| `/proc` scanning is slow or races with process exit | Caps + `Truncated`; ignore ENOENT/ESRCH; fail closed on incomplete same-UID evidence; benchmark |
| A file can be opened after the final check | Linux keeps open inodes alive after unlink; the cache directory disappears but the open file survives. Use target-level recheck for orphaned caches; document the residual race |
| `go-build*` directories are safely regenerable, but a generic cache may not be | `CacheGeneric` is never cleanable by default; `--allow-generic-caches` is explicit |
| The broader `strings.Contains(name, "cache")` rule classifies unrelated dirs | Refine to a known build/run list; keep generic in the protected category |
| `MappedInside` produces false positives from deleted mappings | Strip ` (deleted)` and only match roots; document |
| The new category breaks existing tests/docs | Update tests; keep `CatActiveCache` as a Go alias only; changelog migration note |
| Open-index caps hide a real handle | `Truncated` protects orphaned caches; do not delete under incomplete evidence |

## Rollback

- Revert the implementation commit. No persistent state.
- If the open-index is too expensive, keep the `recent-caches` rename and the `orphaned-caches` display, but leave orphaned caches non-cleanable; remove the opt-in flags until performance is acceptable.
- Do not restore the `active-caches` label without an explicit liveness check.

## References

- UI review screenshot and claim list, 2026-10-19.
- Verification report: `docs/reviews/2026-10-19-swapaudit-ui-review.md`.
- `internal/cache/cache.go:44`, `:51-70`, `:82-89`, `:91-150`.
- `internal/model/model.go:8-16`.
- `internal/app/display.go:391-395`, `:448-458`.
- `internal/cleaner/cleaner.go:101-190`.
- `proc(5)`: `/proc/[pid]/fd`, `/proc/[pid]/cwd`, `/proc/[pid]/maps`.
- `open(2)`: an unlinked file with an open descriptor remains accessible until the descriptor is closed.
- Goal 1: `internal/report` `AllocatedBytes`, `SkipReason`, `AuditReport`.
