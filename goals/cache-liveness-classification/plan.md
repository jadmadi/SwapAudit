# Plan: Classify /tmp Caches by Liveness and Ownership, Not Just mtime

## Execution Steps

### Phase 1 — Baseline and contract freeze

- [x] Capture the current `active-caches` category row and the top 10 `active-caches` rows in `goals/cache-liveness-classification/evidence.md`.
- [x] For the top 10, record `Age`, `Idle`, `SizeBytes`, `Path`, and `stat -c '%i %s %b %n'` (inode, apparent size, blocks) so the before/after classification is auditable.
- [x] Run `lsof +D /tmp` only as a manual cross-check if available; do not make it a product dependency. Record the output separately.
- [x] Run `go test -race ./...` and `go vet ./...`; record the baseline.
- [x] Add the C1–C7 contract from `goal.md` to `docs/cache-liveness.md` before implementation.

### Phase 2 — Categories and classification precedence

- [x] Add `CatRecentCache`, `CatInUseCache`, `CatOrphanedCache`, and `CacheKind` to `internal/model/model.go`.
- [x] Keep `CatActiveCache` as a compile-time alias only; set its string to `recent-caches` or remove it after updating callers.
- [x] Refactor `cache.go` classification into a pure helper:
  ```go
  func ClassifyCache(meta CacheMeta, threshold Thresholds, flags CacheFlags) (model.Category, bool)
  ```
- [x] Implement the C2 precedence exactly; return an `Evidence` string for each result.
- [x] Refactor `isBuildCache` into `CacheKind` detection with explicit prefixes/patterns; keep the generic fallback protected.
- [x] Add table tests for every branch, including `Idle == MaxAge`, `Idle == OrphanCacheAge`, zero size, and generic caches.
- [x] Run `go test -race ./internal/model ./internal/cache`.

### Phase 3 — Open-handle index

- [x] Create `internal/cache/liveness.go` with `OpenIndex`, `BuildOpenIndex`, `Lookup`, `Complete`.
- [x] Implement ancestor expansion for `/proc/[pid]/fd`, `/proc/[pid]/cwd`, and `/proc/[pid]/maps`.
- [x] Parse maps paths with spaces and ` (deleted)`.
- [x] Count EACCES/ENOENT/ESRCH; mark same-UID unreadable evidence incomplete.
- [x] Add PID/FD caps and `Truncated`.
- [x] Build a fake `/proc` fixture helper in `liveness_test.go`; test fd/cwd/maps, ancestors, deleted suffix, caps, and errors.
- [x] Add `BenchmarkBuildOpenIndex`; record PIDs/fds and duration in evidence.
- [x] Run `go test -race ./internal/cache`.

### Phase 4 — Wire liveness into the scan

- [x] Build one `OpenIndex` per audit before `ScanTmpDirectories`.
- [x] Pass the index and `--orphan-cache-age`/`--allow-generic-caches` into the scanner.
- [x] Set `OpenHandles`, `CwdInside`, `MappedInside`, `Evidence`, `LastActiveKnown`, and `CacheKind` on each item.
- [x] Make `Cleanable` reflect the opt-in flag; keep the category independent of the flag.
- [x] Extend `ScanStats` with the C7 counters.
- [x] Validate `--orphan-cache-age <= --max-age` in `Run`; exit 2 with a clear message.
- [x] Run `go test -race ./internal/app ./internal/cache`.

### Phase 5 — Cleanup group and recheck

- [x] Add `OrphanedCaches` and `IncludeOrphanedCaches` to `CleanupFlags`.
- [x] Extend `ResolveCleanupFlags` and add table tests.
- [x] Add `OpenCheckMode` to `CleanupOptions`; default `target` for orphaned caches and `group` for untouched/abandoned.
- [x] Implement `RecheckCache` with the `buildOpenIndex` seam.
- [x] Add the `Orphaned Cache Directories` group to `ExecuteCleanup`.
- [x] In `PruneTmpTargets`, run the recheck before `RemoveAll`; skip with `open-handle`, `cwd-inside`, `mapped-inside`, `evidence-incomplete`, or `opened-since-scan`.
- [x] Ensure dry-run performs the same checks and never calls `RemoveAll`.
- [x] Add `CleanupStats` reasons and summary tests.
- [x] Run `go test -race ./internal/cleaner ./internal/app`.

### Phase 6 — Display and documentation

- [x] Add `orphaned-caches` and `in-use-caches` rows to the summary; always render the orphaned row.
- [x] Rename `active-caches` to `recent-caches` in all output.
- [x] Add the threshold header and liveness line.
- [x] Add action text for each category.
- [x] Add `--explain-caches` and a per-item evidence table.
- [x] Update `PrintQuickCommands`, `PrintUsage`, README, and `docs/cache-liveness.md`.
- [x] Update `CHANGELOG.md` with the migration note.
- [x] Run `grep -R "active-caches" internal --include='*.go'` and confirm no output.

### Phase 7 — Verification and evidence

- [x] Run `gofmt -l .` (empty).
- [x] Run `go vet ./...`.
- [x] Run `go test -race ./...`.
- [x] Run the targeted tests and the benchmark.
- [x] Run `./swapaudit --no-color --ascii --dry-run --clean-orphaned-caches` and capture the before/after category counts.
- [x] Run `./swapaudit --no-color --ascii --dry-run --clean-all` and confirm no orphaned-cache group without the include flag.
- [x] Run `./swapaudit --no-color --ascii --dry-run --clean-all --include-orphaned-caches` and confirm the group appears without deletion.
- [x] Run `./scripts/check.sh --full`.
- [x] Record all outputs in `evidence.md`.

### Phase 8 — Attribution commit

- [x] Commit with `feat:` or `fix:` and a subject containing `(goals/cache-liveness-classification/goal.md)`.
- [x] Run `sila goals --project=/home/jad/scripts/active/swapaudit`.
- [x] Do not push to a remote unless the user explicitly authorizes it.

## Tests to Add

| Test | File | What it proves |
| --- | --- | --- |
| `TestBuildOpenIndexFakeProc` | `internal/cache/liveness_test.go` | fd/cwd/maps indexing and ancestors |
| `TestOpenIndexCaps` | same | Truncation and `Complete()` |
| `TestOpenIndexPermissionErrors` | same | Error counting, no panic |
| `TestOpenIndexDeletedSuffix` | same | Deleted mappings parsed |
| `TestClassifyCachePrecedence` | `internal/cache/cache_test.go` | Every C2 branch |
| `TestClassifyCacheKind` | same | Build/run/generic |
| `TestOrphanedCacheOptIn` | same | Cleanable only with flags |
| `TestInUseCacheProtected` | same | Open handle/cwd/mapped always protected |
| `TestEvidenceIncompleteProtected` | same | Fail closed |
| `TestOrphanAgeValidation` | `internal/app/app_test.go` | Invalid threshold exits 2 |
| `TestResolveCleanupFlagsOrphanedCaches` | same | Flag combinations |
| `TestPruneSkipsOpenHandle` | `internal/cleaner/cleaner_test.go` | `open-handle` reason |
| `TestPruneSkipsOpenedSinceScan` | same | Target-level recheck |
| `TestPruneSkipsEvidenceIncomplete` | same | Fail closed |
| `TestDryRunNoRemove` | same | No mutation |
| `TestCacheLivenessDisplay` | `internal/app/display_test.go` | New rows, zero row, threshold header |
| `TestNoActiveCacheLabel` | same | Banned string absent |
| `BenchmarkBuildOpenIndex` | `internal/cache/liveness_test.go` | Performance evidence |

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -race -run 'Test(BuildOpenIndex|Classify.*Cache|OrphanedCache|InUseCache|EvidenceIncomplete|OpenedSinceScan|ResolveCleanupFlags)' ./internal/cache ./internal/cleaner ./internal/app
go test -bench 'BenchmarkBuildOpenIndex' -benchtime=1x ./internal/cache
./swapaudit --no-color --ascii --dry-run --clean-orphaned-caches > /tmp/orphan-cache-dryrun.txt
./swapaudit --no-color --ascii --dry-run --clean-all > /tmp/clean-all-dryrun.txt
grep -c "Orphaned Cache" /tmp/clean-all-dryrun.txt || true    # expect 0
grep -c "Orphaned Cache" /tmp/orphan-cache-dryrun.txt || true # expect 1
grep -R "active-caches" internal --include='*.go' || true
./scripts/check.sh --full
```

## Rollback

- Revert the implementation commit. No persistent state.
- If the open index is too slow, keep the `recent-caches` rename and the `orphaned-caches` display but leave orphaned caches non-cleanable; remove the opt-in flags until performance is acceptable.
- Do not restore `active-caches` without a real liveness check.

## Completion Evidence

`goals/cache-liveness-classification/evidence.md` must contain:

1. Before/after category counts and the top 10 cache rows with `Age`, `Idle`, size, inode, apparent size, and blocks.
2. The open-index benchmark output (PIDs, FDs, duration).
3. The fake-`/proc` test outputs.
4. Dry-run outputs for `--clean-orphaned-caches`, `--clean-all`, and `--include-orphaned-caches`.
5. `go test -race`, `go vet`, and `scripts/check.sh --full` outputs.
6. Any deviation from this plan and why.
