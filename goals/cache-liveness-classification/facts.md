# Facts: Classify /tmp Caches by Liveness and Ownership, Not Just mtime

## Architectural Invariants & Constraints

- **Liveness beats age.** A cache with an open file descriptor, a process cwd inside it, or a file-backed mapping inside it is never cleanable, regardless of mtime.
- **Fail closed on incomplete evidence.** If the open index is truncated or a same-UID `/proc` read fails, orphaned caches are not cleanable.
- **mtime is a lower bound on activity.** `newestMtime` can be old while a process still holds the directory open; the liveness check is the stronger signal.
- **No external tools.** The open index uses only `/proc` reads and standard-library calls. No `lsof`, `fuser`, `find`, `cgo`, or new module dependency.
- **Current UID only.** The scanner continues to exclude other users' entries; the cleanup path continues to verify ownership.
- **No silent deletion.** Every skip reason is counted and printed; dry-run performs the same checks but never calls `RemoveAll`.
- **Opt-in for the new risk.** `orphaned-caches` is visible by default but cleanable only with an explicit flag.
- **One index per audit, one recheck per target for orphaned caches.** Performance and safety are explicit, not incidental.
- **Goal 1 owns allocated-size accounting and the report contract.** This goal consumes `AllocatedBytes`, `SkipReason`, and `AuditReport` rather than redefining them.

## Observed Facts (Repository)

| Fact | Evidence | Consequence |
| --- | --- | --- |
| `active-caches` is assigned whenever `lastActive <= maxAge` | `internal/cache/cache.go:140-150` | The label claims activity but tests only recency |
| `lastActive` is `now - newestMtime` | `internal/cache/cache.go:137-138` | A never-touched directory has `Age == Idle`; it is still labelled active |
| No open-handle check exists in the scan or cleaner | `grep -R "/fd\\|/maps\\|/cwd" internal/cache internal/cleaner` | A cache in active use can be classified by age alone |
| Scan uses current UID only | `internal/cache/cache.go:44`, `:83` | Ownership is known; liveness is not |
| Daemon prefixes are excluded before classification | `internal/cache/cache.go:51-70` | Their activity is invisible, but they are also not cleanup targets |
| Size is `info.Size()` | `internal/cache/cache.go:124` | Sparse files overstate reclaimable bytes |
| Delete path rechecks lstat/symlink/UID | `internal/cleaner/cleaner.go:148-171` | Good file guards, no liveness guard |
| No orphaned-cache category or flag exists | `internal/model/model.go:8-16`; `internal/app/app.go:160-180` | A young unused cache has no action |
| Dry-run and live share `PruneTmpTargets` | `internal/cleaner/cleaner.go:101-190` | A recheck can be added once and tested in both modes |
| `CleanupStats` already supports skip reasons | `internal/cleaner/cleaner.go:41-73` | Extend it rather than adding global counters |

## Observed Facts (Live Host, 2026-10-19)

| Observation | Command | Value |
| --- | --- | --- |
| Active-cache category | `./swapaudit --no-color --ascii` | `active-caches | 434 | 3.0 GB | 72.4% | 0 B` |
| Largest active cache | same | `go-build873559002 657.8 MB, Age 6h45m, Idle 6h45m` |
| Second/third | same | `go-build2547211510 627.0 MB, 3h37m/3h37m`; `go-build1434250357 604.4 MB, 3h12m/3h11m` |
| `/tmp` header | same | `5.2 GB / 15.6 GB (free 10.4 GB)` |
| Total cache entities | same | `434` active + `136` abandoned + `9` untouched |
| Shell/tmp noise | same | Many `sila_mcp_test_*` and `sila-cache-*` entries are `0 B` |
| `lsof` availability | `command -v lsof` | Unknown; do not depend on it |
| `fuser` availability | `command -v fuser` | Unknown; do not depend on it |
| `/proc/self/fd` readable | `readlink /proc/self/fd/*` | Yes |
| `hidepid` policy | `mount | grep proc` | Not verified; handle EACCES defensively |

## External Facts

- `/proc/[pid]/fd` entries are symlinks to open file descriptions. Reading the link yields the path, or a deleted-path suffix.
- `/proc/[pid]/cwd` is a symlink to the process working directory.
- `/proc/[pid]/maps` lists file-backed mappings; paths with spaces are possible and deleted mappings may end in ` (deleted)`.
- On Linux, unlinking an open file does not invalidate the open descriptor; the inode persists until the last descriptor is closed. A liveness check reduces the chance of surprising a running build, but cannot eliminate the race.
- `os.RemoveAll` follows the directory entries it traverses; the existing code rejects top-level symlinks and `WalkDir` does not follow symlinks. Do not weaken that.
- `syscall.Stat_t.Blocks` is in 512-byte units; Goal 1 uses it for allocated size.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| An open fd under a cache directory is the strongest available "in use" signal | It is cheap and kernel-authoritative | Fake `/proc` fixture with an fd symlink; live test with a child process holding a temp file |
| A process cwd inside a cache means it is in use | Build tools and shells can hold cwd | Fake `/proc` cwd fixture |
| A file-backed mapping means the cache is in use | Compilers map object files | Fake `/proc` maps fixture |
| `go-build*` and similar build caches are regenerable | Deleting them costs a rebuild, not data | Document; opt-in |
| A 1h orphan-cache threshold is safe enough to show, not delete by default | Balances opportunity and safety | Dry-run on the live host; user review |
| `/proc` read errors for other UIDs can be ignored | The scan only owns current-UID caches | Fake permission-error fixture; do not mark foreign caches cleanable |
| The open index is affordable on a developer workstation | The product promises sub-second audits | Benchmark; if >150 ms, cap and document |

## Unknowns

- How many of the 434 "active" caches are genuinely in use on this host. **Unknown until the index runs.** Do not claim a number before measuring.
- Whether `go-build` directories are held open by a Go process that is between builds. **Unknown.** If no fd/cwd/mapped evidence exists, they are classified orphaned; the opt-in flag is the protection.
- Whether `hidepid=2` or container `/proc` restrictions make the index incomplete. **Unknown.** Fail closed with `evidence-incomplete`.
- Whether a build can recreate a cache directory after it is removed. **Unknown.** Deleting a cache is safe for correctness (the tool rebuilds), but may slow the next build; document.
- Whether `lsof`/`fuser` would find handles that `/proc` does not. **Unknown.** They use `/proc` on Linux; the open index is the same source without the dependency.

## Evidence Commands

```bash
grep -n "CatActiveCache\|lastActive > maxAge\|newestMtime\|info.Size()" internal/cache/cache.go internal/model/model.go
grep -n "RemoveAll\|Lstat\|ModeSymlink\|stat_t.Uid" internal/cleaner/cleaner.go
./swapaudit --no-color --ascii | awk '/active-caches|go-build/'
stat -c '%i %s %b %n' /tmp/go-build873559002 /tmp/go-build2547211510 /tmp/go-build1434250357
readlink /proc/self/fd/* 2>/dev/null | head
cat /proc/self/maps | head
go test -race ./...
```

## File & Interface Contracts

- `internal/model/model.go`: `CatRecentCache`, `CatInUseCache`, `CatOrphanedCache`, `CacheKind`, `OpenHandles`, `CwdInside`, `MappedInside`, `Evidence`, `LastActiveKnown`.
- `internal/cache/liveness.go` (new): `OpenIndex`, `BuildOpenIndex`, `Lookup`, `Complete`.
- `internal/cache/cache.go`: `ClassifyCache`, `CacheMeta`, `CacheFlags`, `ScanStats` liveness counters; allocated size from Goal 1.
- `internal/cleaner/cleaner.go`: `OpenCheckMode`, `RecheckCache`, `buildOpenIndex` seam, new `CleanupStats` reasons, orphaned-cache group.
- `internal/app/app.go`: `--orphan-cache-age`, `--clean-orphaned-caches`, `--include-orphaned-caches`, `--allow-generic-caches`, `--explain-caches`; `ResolveCleanupFlags` fields.
- `internal/app/display.go`: new category rows, threshold header, liveness line, action text.
- `docs/cache-liveness.md` (new): the C1–C7 contract for operators.
- `README.md`, `CHANGELOG.md`: flag and migration documentation.
- `go.mod`: unchanged.
