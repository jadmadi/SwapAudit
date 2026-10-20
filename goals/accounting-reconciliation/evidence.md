# Evidence: accounting reconciliation

Verdict: PASS

Date: 2026-10-20 (implementation), baseline 2026-10-19.

## Before (goal facts, 2026-10-19)

- Header swap `7.5 GB / 17.1 GB`; category rows `814.7 MB + 263.3 MB + 1.8 GB ≈ 2.85 GB`; no residual row.
- Header `/tmp 5.2 GB`; scanned cache categories `782.2 MB + 373.8 MB + 3.0 GB ≈ 4.15 GB`; no residual row.
- `Cleanable: 1.9 GB` added tmpfs and swap into one primary number.
- Actionable: 150 cleanable, 12 shown, 138 hidden, 23.3 MB hidden; limit hardcoded to 12.
- `--all --limit 0` still printed `... and 594 more hidden`.
- Cache size used `info.Size()` (apparent), not allocated blocks.
- Prune footer used `len(targets)-shown`, so skipped targets inflated the hidden count.

## After (live host, 2026-10-20)

Header sources:

```text
$ grep -E '^(SwapTotal|SwapFree|MemTotal|MemAvailable):' /proc/meminfo
MemTotal:       32762868 kB
MemAvailable:   22811276 kB
SwapTotal:      17892336 kB
SwapFree:       11803544 kB

$ df -B1 /tmp
Filesystem       1B-blocks       Used   Available Use% Mounted on
tmpfs          16774586368 4748091392 12026494976  29% /tmp

$ awk '/^VmSwap:/{s+=$2} END{printf "%d kB\n", s}' /proc/[0-9]*/status
2596340 kB
```

Rendered reconciliation:

```text
  Swap:  2.5 GB attributed + 3.3 GB unattributed = 5.8 GB used
  /tmp:  2.7 GB attributed + 1.8 GB unattributed = 4.4 GB used
  unattributed reasons: kernel-or-shmem 3.3 GB · non-matching 1.7 GB · dotfile 20.7 MB ·
                        daemon-prefix 1.7 MB
  Reconciles: OK
```

Exact JSON arithmetic (no rounding):

```text
swap 2658652160 + 3576270848 = 6234923008 True
tmpfs 2845933568 + 1902166016 = 4748099584 True
schema 1 cleanup None items 557
```

`SwapUsed` is `17892336 kB - 11803544 kB = 6088792 kB = 6,234,923,008` bytes, which equals the JSON sum. `TmpfsUsed` is `4,748,091,392` bytes from `statfs`, which equals the JSON sum.

## `--all` population

```text
$ ./swapaudit --no-color --ascii --all --limit 0 > /tmp/all.txt
$ grep -c 'more hidden' /tmp/all.txt
0
$ grep -c 'more groups' /tmp/all.txt
0
$ wc -l /tmp/all.txt
631
```

A previously hidden small cache is present:

```text
$ python3 ... # first cache under 10 MB from the JSON report
sample=go-build3656852606 (4538368 bytes)
$ grep -F go-build3656852606 /tmp/all.txt
  51c485e6 | go-build3656852606          | in-use-caches    |     17h52m |       17h52m |     4.3 MB
```

The population line for a normal run ties out:

```text
Population: 549 raw · 478 after shell aggregation · 15 shown · 428 hidden below 10 MB · 35 beyond --limit
```

428 + 35 + 15 = 478. The actionable line is `8 cleanable · 8 shown (2 rows) · 0 hidden (0 B) · limit 3 rows`.

## Tests

```text
=== RUN   TestBuildReconcilesSwap        --- PASS
=== RUN   TestBuildReconcilesTmpfs       --- PASS
=== RUN   TestBuildOverlap               --- PASS
=== RUN   TestBuildNoHybridItems         --- PASS
=== RUN   TestBuildCategoriesDeterministic --- PASS
=== RUN   TestBuildBiggestWin            --- PASS
=== RUN   TestRandomFixturesNoNegativeResidual --- PASS  (100 randomized fixtures)
=== RUN   TestAllocatedBytesSparseFile   --- PASS  (1 GiB apparent, ≤64 KiB allocated)
=== RUN   TestSkipReasons                 --- PASS
=== RUN   TestPruneFooterWithSkips        --- PASS  (1 hidden non-skipped target, 16 B; skipped target excluded)
=== RUN   TestTableWidthAndNoTrailingWhitespace --- PASS  (80/100/120)
=== RUN   TestReconciliationFailureVisible --- PASS  (WARN/FAIL without hiding the screen)
=== RUN   TestAllModeWithExplicitLimit  --- PASS  (--all --limit 3 shows 3, hides 2)
=== RUN   TestPopulationExactCounts      --- PASS  (20 raw · 18 aggregated · 4 shown · 5 size-hidden · 9 limit-hidden)
ok  github.com/jadmadi/swapaudit/internal/report
ok  github.com/jadmadi/swapaudit/internal/cache
ok  github.com/jadmadi/swapaudit/internal/app
```

Verification gate:

```text
$ gofmt -l .            # empty
$ go vet ./...          # pass
$ go test -race ./...   # pass
$ ./scripts/check.sh --full
✔ All checks passed.
```

`go.mod` is unchanged:

```text
module github.com/jadmadi/swapaudit

go 1.23.0
```

## Deviation

The live `--all` output is 631 lines and 557 JSON items, not the 805 items in the 2026-10-19 baseline. The scan now uses allocated blocks, liveness classification, full agent visibility, and shell aggregation, so the raw population differs. The invariants are checked against the new snapshot, not the old one.

## Sparse-file proof

`TestAllocatedBytesSparseFile` creates a 1 GiB sparse file, then asserts `ApparentBytes >= 1 GiB` and `AllocatedBytes <= 64 KiB` with no fallback. `SizeBytes` equals `AllocatedBytes` for cache items (`TestScanUsesAllocatedSize`), proving the display/reclaim path never uses apparent size.
