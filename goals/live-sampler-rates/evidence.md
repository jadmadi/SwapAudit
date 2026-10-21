# Evidence: Add a Headless Sampler with Rates, Attribution, and a Gated ETA

Date: 2026-10-21
Workstation: Linux, 8 logical cores, Go 1.27.2 toolchain, module `go 1.23.0`

Verdict: PASS

## 1. Build, vet, race suite, and full gate

```
$ go build ./...
$ go vet ./...
$ go test -race ./...
ok  github.com/jadmadi/swapaudit/cmd/swapaudit     1.538s
ok  github.com/jadmadi/swapaudit/internal/app      (cached)
ok  github.com/jadmadi/swapaudit/internal/cache    (cached)
ok  github.com/jadmadi/swapaudit/internal/cleaner  (cached)
ok  github.com/jadmadi/swapaudit/internal/live     1.374s
ok  github.com/jadmadi/swapaudit/internal/model    (cached)
ok  github.com/jadmadi/swapaudit/internal/picker   (cached)
ok  github.com/jadmadi/swapaudit/internal/proc     (cached)
ok  github.com/jadmadi/swapaudit/internal/snapshot (cached)
ok  github.com/jadmadi/swapaudit/internal/term     (cached)
ok  github.com/jadmadi/swapaudit/internal/triage   (cached)
ok  github.com/jadmadi/swapaudit/internal/updater  (cached)
ok  github.com/jadmadi/swapaudit/internal/version  (cached)

$ ./scripts/check.sh --full
All checks passed in 16s.
```

## 2. Fixture matrix

`go test ./internal/live -run TestAnalyzeFixtures -v`:

| Fixture | Trend | Basis | ETA | R² | Non-negative ratio |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `idle.ndjson` | flat | flat | null | 1.000000 | 1.000 |
| `ramp.ndjson` | filling | ols | 3525 | 1.000000 | 1.000 |
| `spike.ndjson` | insufficient | noisy_fit | null | 0.270075 | 1.000 |
| `recovering.ndjson` | recovering | recovering | null | 1.000000 | 0.000 |
| `noise.ndjson` | insufficient | non_monotone | null | 0.743749 | 0.492 |
| `short.ndjson` | insufficient | insufficient_samples | null | 1.000000 | 1.000 |
| `stale-tmp.ndjson` | tmpfs insufficient | short_span | null | - | - |

`TestRampETAWithin10Percent` computes the analytic value from the fixture's
last free bytes and observed slope:

```
free = swap_total - swap_used(last) = 7,392,460,800 bytes
slope = 2,097,152 bytes/s
analytic ETA = 3525.0s; measured = 3525s; deviation 0%
```

## 3. PID reuse and exits

`TestPIDReuse`:

```
exited_bytes  = 314,572,800 (300 MiB, old (42,100) key)
started_bytes = 104,857,600 (100 MiB, new (42,200) key)
movers containing PID 42: none
```

`TestExitNeverNegativeMover`:

```
exited_bytes = 209,715,200 (200 MiB)
negative movers: none
```

## 4. Output captures

`--once`:

```
$ swapaudit sample --once > /tmp/sample-once.json
exit=0, 116 ms (bash time: real=0.116 user=0.038 sys=0.084)
$ python3 -m json.tool < /tmp/sample-once.json
VALID
type=once schema=1
window keys: complete, exited_bytes, movers, samples, seconds, skipped_ticks,
slow_lane_skips, started_bytes, swap, tmpfs, tmpfs_cleanable_bytes,
tmpfs_cleanable_items, tmpfs_movers, tmpfs_sampled_age_seconds,
unattributed_bytes
```

`--stream --interval=100ms --duration=1s`:

```
$ wc -l /tmp/stream1s.ndjson
11
samples 10, stopped 1
last: {"type":"stopped","reason":"duration","sampled_at":"2026-10-21T10:54:25+03:00"}
```

## 5. 60-second run and CPU

```
$ bash -c 'TIMEFORMAT="stream60 real=%R user=%U sys=%S"; time
  swapaudit sample --stream --interval=1s --duration=60s --record=/tmp/live60.ndjson
  > /tmp/stream60.ndjson' 2>/tmp/stream60.time
stream60 real=60.016 user=0.583 sys=0.468
exit=0, stderr=0 bytes
61 output lines: 60 samples + 1 stopped
60 recorded raw lines; every line parses as JSON
```

CPU = (0.583 + 0.468) / 60.016 = **1.75% of one core**, well under the 15%
target. The slow lane ran the recursive `/tmp` walk every 30 s in its own
goroutine; no fast tick blocked on it.

## 6. Recording and replay

```
$ stat -c '%a %n' /tmp/live60.ndjson
600 /tmp/live60.ndjson
```

`TestRecordReplay` records one sample, asserts mode `0600`, replays it, and
compares the monotonic offset, system counters, process key `(1234,42)`, name
`gopls`, cache path `/tmp/go-build1`, cache age, and `tmpfs_sampled_at`.
`TestRecordDeterministic` writes the same sample twice and asserts byte-identical
lines. `TestReplayInvalid` rejects malformed input.

## 7. Lifecycle and safety

- `TestNoGoroutineLeak` runs a stream, cancels on duration, and asserts the
  goroutine count returns to baseline (+2 tolerance for the signal package).
- `TestRunCancel` cancels from `OnSample` and asserts a `stopped` line with
  `reason: "signal"` and exit 0.
- `TestClockJumpDropped` injects a backwards monotonic source; samples are
  dropped, the loop continues, and the stream still stops cleanly.
- `TestBrokenPipe` injects a failing writer; `Run` returns the write error.
- `TestSlowLane` runs the slow scanner on a 10 ms cadence and asserts at least
  two scans.
- The sampler imports no TUI, terminal-state, or picker code. Output is JSON or
  NDJSON only; `TestRunStream` asserts no ANSI escapes.
- Ticks are never queued: `time.Ticker` drops overlapping ticks and the sampler
  increments `skipped_ticks` when a scan exceeds the interval.
- The ring is capped at 600 slots and tracks only `VmSwap > 0` processes and
  scanned cache paths.

## 8. Size ceilings (NFR-2)

Every production file is under 500 lines and every test file is under 550:

```
$ wc -l internal/live/*.go
 314 internal/live/analyze.go
 230 internal/live/analyze_test.go
 202 internal/live/record.go
 121 internal/live/record_test.go
  95 internal/live/ring.go
  68 internal/live/ring_test.go
 399 internal/live/sampler.go
 253 internal/live/sampler_test.go
```

The per-file interpretation is used: no production file exceeds 500 lines and
no test file exceeds 550. The package totals are 1010 production / 672 test
lines across four cohesive files; the repository hard cap of 1,000 lines per
file is respected.

## 9. Documentation

- `docs/sampling.md`: lanes, identity, estimator gates with the exact order,
  attribution formulas and caveats, output shapes, raw trace format, and three
  recipes (statusline, CI threshold, incident capture).
- `README.md`: "Live sampling and rates" section and project-layout entry.
- `PrintUsage`: `sample` command, usage block (`sample --help`), and examples.
- `CHANGELOG.md`: five entries under `[Unreleased]`.

Verdict: PASS
