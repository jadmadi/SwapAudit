# Evidence: cache liveness classification

Verdict: PASS

Date: 2026-10-20 (implementation), baseline 2026-10-19.

## Before (goal facts, 2026-10-19)

- `active-caches | 434 | 3.0 GB | 0 B` actionable.
- Top rows: `go-build873559002 657.8 MB`, `go-build2547211510 627.0 MB`, `go-build1434250357 604.4 MB`; age == idle for the first two.
- Classification used only `lastActive > maxAge`; no `/proc/[pid]/fd`, `cwd`, or `maps` check.
- Deletion rechecked lstat/symlink/UID but not liveness.

## After (live host, 2026-10-20)

```text
$ ./swapaudit --no-color --ascii --limit 3 | sed -n '/Category Breakdown/,$p'
  Process swap (VmSwap) · dormant > 2h
    dormant-agents                        |        8 |     1.1 GB | [####......]  42.8% |     1.1 GB
    abandoned-agents                      |        0 |        0 B | [..........]   0.0% |        0 B
    desktop-daemons                       |       17 |   251.8 MB | [..........]   9.9% |        0 B
    active-apps                           |      177 |     1.2 GB | [####......]  47.3% |        0 B
    .. [unattributed]                     |          |     3.3 GB |                     |          -
  /tmp allocated (tmpfs) · stale > 12h · orphan-cache > 1h
    abandoned-runs                        |        0 |        0 B | [..........]   0.0% |        0 B
    orphaned-caches                       |        0 |        0 B | [..........]   0.0% |        0 B
    in-use-caches                         |      356 |     2.7 GB | [##########] 100.0% |        0 B
    .. [unattributed]                     |          |     1.8 GB |                     |          -
  Cache liveness: 356 in-use · 0 orphaned (0 B) · 0 recent · 356 evidence-incomplete · index 430 PIDs / 6104 FDs
```

JSON exposes the same scan evidence:

```text
$ ./swapaudit --json | jq .scan
{"scanned_caches":356,"in_use_caches":356,"orphaned_caches":0,"recent_caches":0,
 "evidence_incomplete":356,"open_index_truncated":false,"open_index_errors":16,
 "open_index_pids":430,"open_index_fds":6104,"fallback_count":0}
```

Counts move between runs as processes start and exit.

There is no `active-caches` string in Go source or rendered output. `recent-caches` is the honest recency label.

## Why every cache is `in-use` on this host

`BuildOpenIndex` failed closed because five same-UID processes have root-only fd tables:

```text
$ for p in /proc/[0-9]*; do ... done
unreadable same-uid: /proc/1570   (sd-pam)
unreadable same-uid: /proc/4239   (ssh-agent)
unreadable same-uid: /proc/4250   (kwin_wayland)
unreadable same-uid: /proc/4283   (fusermount3)
unreadable same-uid: /proc/4494   (polkit-kde-auth)
count=5
```

These are setuid-root processes whose real UID is 1000 but whose `fd` directories are root-only. The contract says a same-UID unreadable descriptor table marks the index incomplete and blocks orphaned-cache deletion. `--explain-caches` exposes the evidence per cache.

This is the expected fail-closed result, not a detector failure. On a host where all same-UID fd tables are readable, caches with no handle/cwd/mapping and idle beyond `--orphan-cache-age` classify as `orphaned-caches` (proven by fixture tests).

## Open-index benchmark

```text
$ go test -bench BenchmarkBuildOpenIndex -benchtime=1x -run '^$' ./internal/cache
goos: linux
goarch: amd64
pkg: github.com/jadmadi/swapaudit/internal/cache
cpu: Intel(R) Core(TM) i7-9700 CPU @ 3.00GHz
BenchmarkBuildOpenIndex-8   	       1	  41565228 ns/op
PASS
```

≈41.6 ms for the live host (439 PIDs, 200k FD cap not reached). The design target is ≤150 ms.

## Fixture tests

```text
=== RUN   TestBuildOpenIndexFakeProc       --- PASS  (fd/cwd/maps, ancestor expansion)
=== RUN   TestOpenIndexPermissionErrors    --- PASS  (same-UID unreadable → Errors > 0, incomplete)
=== RUN   TestOpenIndexDeletedSuffix       --- PASS  ((deleted) stripped; [heap] ignored)
=== RUN   TestOpenIndexComplete            --- PASS
=== RUN   TestClassifyCachePrecedence      --- PASS  (13 branches, including idle == max-age and idle == orphan-age boundaries)
=== RUN   TestClassifyCacheKind            --- PASS
=== RUN   TestScanOrphanedCacheOptIn       --- PASS  (orphaned by default; cleanable only on opt-in)
=== RUN   TestScanInUseAndEvidenceIncomplete --- PASS  (fail closed)
=== RUN   TestScanUsesAllocatedSize        --- PASS
```

## Cleanup gating

`--clean-all` does not include orphaned caches:

```text
$ ./swapaudit --dry-run --clean-all --no-color --ascii --limit 1 | grep -c 'Orphaned Cache Directories'
0
```

`--include-orphaned-caches` with `--clean-all` does:

```text
Group: Untouched Build Caches - Nothing cleanable found.
Group: Abandoned Test Runs & Workspaces - Nothing cleanable found.
Group: Orphaned Cache Directories - Nothing cleanable found.
Group: Dormant Agent Sessions (8 processes, 1.1 GB)
Cleanup summary (dry-run): 0 would prune, 0 would end, 8 skipped, (8 parent-alive)
```

`--clean-orphaned-caches` alone works:

```text
Group: Orphaned Cache Directories - Nothing cleanable found.
Cleanup summary (dry-run): 0 would prune, 0 would end, 0 skipped
```

Invalid thresholds fail before scanning:

```text
$ ./swapaudit --max-age 1h --orphan-cache-age 24h
exit=2
error: --orphan-cache-age (24h0m0s) must be <= --max-age (1h0m0s)
```

Deletion rechecks:

```text
=== RUN   TestPruneSkipsOpenHandleAndOpenedSinceScan --- PASS
=== RUN   TestPruneSkipsEvidenceIncomplete             --- PASS
=== RUN   TestDryRunNoRemove                           --- PASS
```

## Deviation

AC-12 asks for a live before/after reclassification of the three largest `go-build*` directories. On this host the live index is incomplete because of setuid-root same-UID fd tables, so every cache is `in-use-caches` and orphaned-cache cleanup is disabled. This is the specified fail-closed behavior (AC-09). The positive path is proven by the fixture tests above; the live decision is recorded as `evidence-incomplete`.
| # | Name | Category | Age | Idle | Allocated | Path |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | go-build873559002 | in-use-caches | 63169s | 63167s | 690561024 | /tmp/go-build873559002 |
| 2 | go-build2547211510 | in-use-caches | 51839s | 51835s | 658186240 | /tmp/go-build2547211510 |
| 3 | go-build1434250357 | in-use-caches | 50337s | 50328s | 634617856 | /tmp/go-build1434250357 |
| 4 | go-build3666691039 | in-use-caches | 59977s | 59830s | 205672448 | /tmp/go-build3666691039 |
| 5 | go-build2866978059 | in-use-caches | 42232s | 42095s | 148697088 | /tmp/go-build2866978059 |
| 6 | go-build2275922041 | in-use-caches | 35694s | 35549s | 85508096 | /tmp/go-build2275922041 |
| 7 | node-compile-cache | in-use-caches | 299810s | 32739s | 53673984 | /tmp/node-compile-cache |
| 8 | go-build905898776 | in-use-caches | 11324s | 11322s | 50880512 | /tmp/go-build905898776 |
| 9 | tsx-1000 | in-use-caches | 436s | 436s | 47546368 | /tmp/tsx-1000 |
| 10 | sila-audit-test-eval-bc3a542c2bc21dbf | in-use-caches | 50328s | 50328s | 34099200 | /tmp/sila-audit-test-eval-bc3a542c2bc21dbf |

```text
$ stat -c '%i %s %b %n' <top-10 paths>
10051993 8360 0 /tmp/go-build873559002
10250612 8260 0 /tmp/go-build2547211510
10435920 8180 0 /tmp/go-build1434250357
10119744 5840 0 /tmp/go-build3666691039
10774013 5580 0 /tmp/go-build2866978059
11191812 5580 0 /tmp/go-build2275922041
385 80 0 /tmp/node-compile-cache
11235289 4380 0 /tmp/go-build905898776
133624 48320 0 /tmp/tsx-1000
10459202 100 0 /tmp/sila-audit-test-eval-bc3a542c2bc21dbf
```

`lsof +D /tmp` cross-check (manual, not a product dependency) shows open descriptors under `/tmp` for X11/ICE/tmux/adb/Chrome/Telegram/opencode. None of the top-10 go-build directories appear as open paths in this snapshot. The product still fails closed because the `/proc` index has same-UID read errors; this cross-check is informational only.

```text
$ command -v lsof && lsof +D /tmp | head
COMMAND       PID USER  FD   TYPE ... NAME
kwin_wayl    4238  jad   8u  unix ... /tmp/.X11-unix/X1 type=STREAM (LISTEN)
Xwayland     4336  jad   8u  unix ... /tmp/.X11-unix/X1 type=STREAM (LISTEN)
ksmserver    4381  jad  16u  unix ... /tmp/.ICE-unix/4381 type=STREAM (LISTEN)
tmux         209573 jad  6u  unix ... /tmp/tmux-1000/default type=STREAM (LISTEN)
adb          415754 jad  1w   REG ... /tmp/adb.1000.log
chrome       942190 jad  16u  unix ... /tmp/com.google.Chrome.8ahsom/SingletonSocket
opencode     3319164 jad mem  REG ... /tmp/.bun-1000-89812e6e6a68aaeb.so
```

PID/FD caps are covered by `TestOpenIndexComplete` (truncated/errors make `Complete()` false). A literal 10,000-PID/200,000-FD cap test is not practical in the unit suite; the cap branches are exercised through the `Truncated` state and the live index reports its scanned PID/FD counts.
