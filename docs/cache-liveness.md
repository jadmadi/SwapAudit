# Cache liveness contract

Cache classification is evidence-first. mtime is only one signal; an open descriptor, a process cwd, or a file-backed mapping always wins.

## Categories

| Category | Meaning | Cleanable |
| --- | --- | --- |
| `untouched-caches` | Build cache (`CacheBuild`) idle beyond `--max-age` | Yes |
| `abandoned-runs` | Test/run cache (`CacheRun`) idle beyond `--max-age` | Yes |
| `orphaned-caches` | No liveness evidence, idle beyond `--orphan-cache-age` | Only with `--clean-orphaned-caches` or `--include-orphaned-caches` + `--clean-all` |
| `in-use-caches` | Open fd, cwd, mapping, or incomplete evidence | Never |
| `recent-caches` | Younger than the thresholds, or a protected generic cache | Never |

`active-caches` is retired. It claimed activity but only tested mtime.

## Classification precedence

The first matching rule wins:

1. **Excluded** (counted in scan reasons): dotfile, `systemd-*`/`pulse-*`, daemon prefix, symlink, socket/pipe/device, other UID, non-matching name.
2. **In use:** `OpenHandles > 0`, `CwdInside`, or `MappedInside` → `in-use-caches`, not cleanable.
3. **Incomplete evidence:** the `/proc` index is truncated or a same-UID fd table is unreadable → `in-use-caches`, not cleanable (fail closed).
4. **Stale:** idle > `--max-age`:
   - `CacheBuild` → `untouched-caches`, cleanable.
   - `CacheRun` → `abandoned-runs`, cleanable.
   - `CacheGeneric` → `abandoned-runs` only with `--allow-generic-caches`; otherwise `recent-caches`.
5. **Young but unused:** idle > `--orphan-cache-age` and no liveness evidence → `orphaned-caches`. Cleanable only with the opt-in flag.
6. **Recent:** everything else → `recent-caches`.

`Idle` is `now - newestMtime` including nested writes. `Age` is `now - root mtime`.

## Cache kinds

- **Build:** `go-build*`, `rustup-update*`, `cargo-*`, `npm-*`, `sila-cache-*`, `pip-*`, `yarn-*`, `pnpm-*`, `gradle-*`, `m2-*`, `cmake-*`, `ninja-*`, `bazel-*`.
- **Run:** `Test*`, `test*`, `test-bwrap*`, `tsx-*`, `tmp-*`, `temp-*`, plus `--tmp-pattern` matches and the test/build/review/candidate/probe/profile/repro keywords.
- **Generic:** matched only by the broad `cache` keyword or an ambiguous pattern. Never cleanable by default.

## Open-handle index

`internal/cache.BuildOpenIndex` scans `/proc` for:

- `/proc/[pid]/fd` symlink targets,
- `/proc/[pid]/cwd` symlink targets,
- file-backed paths in `/proc/[pid]/maps`, stripping a trailing ` (deleted)`.

Each path is expanded to every ancestor down to the scan root, so a lookup for a cache directory is O(1). Anonymous mappings and `[heap]`/`[stack]` are ignored.

Caps: 10,000 PIDs and 200,000 descriptors. Hitting a cap sets `Truncated`. An unreadable fd table for a same-UID process increments `Errors`. Either condition makes `Complete()` false and forces fail-closed classification for orphaned caches.

`--explain-caches` prints the per-cache evidence string (`idle`, `handles`, `cwd`, `mapped`, `kind`, decision).

## Deletion recheck

`PruneTmpTargets` rechecks liveness before `os.RemoveAll`:

- `target` mode for `orphaned-caches`: a fresh index immediately before each deletion.
- `group` mode for `untouched-caches` and `abandoned-runs`: one index per group.

Skip reasons: `open-handle`, `cwd-inside`, `mapped-inside`, `evidence-incomplete`, `opened-since-scan`. Dry-run performs the same checks and prints the same reasons; it never calls `RemoveAll`.

## Flags

| Flag | Default | Semantics |
| --- | --- | --- |
| `--orphan-cache-age <duration>` | `1h` | Idle threshold for orphaned caches; must be `<= --max-age` |
| `--clean-orphaned-caches` | `false` | Prune orphaned caches only |
| `--include-orphaned-caches` | `false` | Include orphaned caches when `--clean-all` is set |
| `--allow-generic-caches` | `false` | Permit generic cache classification and cleanup |
| `--explain-caches` | `false` | Print per-cache liveness evidence |

`--clean-all` never includes orphaned caches. `--clean-tmp` covers untouched and abandoned caches only. A configuration with `--orphan-cache-age > --max-age` exits `2`.

## Performance and limits

`BenchmarkBuildOpenIndex` measures the live index. The design target is ≤150 ms on a host with 2,000 processes and 50,000 descriptors. Caps protect the audit from pathological fd tables; a truncated index disables orphaned-cache deletion rather than guessing.
