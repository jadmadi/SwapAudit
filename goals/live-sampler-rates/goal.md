# Goal: Add a Headless Sampler with Rates, Attribution, and a Gated ETA

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit (personas 1, 5, 32)
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: Pending

## Goal Description

A one-shot report can say what is present; it cannot say what is moving. This
goal adds the measurement engine that answers three questions no snapshot can:
how fast is swap or tmpfs filling, who is responsible for the change, and how
long until the resource is exhausted. It is deliberately headless — no TUI, no
key handling, no terminal state. It emits a stable JSON snapshot and an optional
NDJSON stream, which makes it usable from a statusline, a script, CI, or a future
interactive view without rework.

The hard part is not sampling. It is trusting the rate. Swap is bursty, the
kernel can reclaim pages mid-window, and a recycled PID can fabricate a
multi-gigabyte mover. This goal therefore treats the estimator as a tested
component: it is gated on sample count, window span, fit quality, and monotonic
behavior; it returns a nullable ETA rather than a confident lie; and it ships
with recorded traces that exercise idle, ramp, spike, recovery, process exit,
and PID reuse before any ETA is exposed.

### Desired outcome

- `swapaudit sample --once` emits one JSON object: the goal-2 snapshot plus a
  `window` block with deltas, rates, movers, and a nullable ETA.
- `swapaudit sample --stream` emits one compact NDJSON line per tick and a final
  `stopped` line.
- A process that exits is reported as `exited_bytes`, never as a negative mover.
- A reused PID produces a new entity key; the old key is retired, so no phantom
  delta appears.
- When the fit is poor, the trend is negative, or the window is undersampled,
  `eta_seconds` is `null` and `eta_basis` says why.
- Every estimate is reproducible from a recorded trace and asserted in tests.
- A 60-second run costs less than 15% of one core on this workstation.

### Non-goals

- No TUI, alternate screen, raw mode, termios, key handling, or colors. The
  interactive view is a separate, deferred goal.
- No historical database, retention policy, or query language.
- No alerting, webhooks, or daemons.
- No changes to the one-shot human report or the action picker.
- No new data sources: `/proc`, `statfs`, and the existing `/tmp` scanner only.
- No estimation of RAM exhaustion; only swap and tmpfs are modeled, because
  `MemAvailable` is reclaimable and not a monotone reservoir.

## Scope & Boundaries

### In scope

- `internal/live/sampler.go` (new): lanes, lifecycle, sampling.
- `internal/live/ring.go` (new): bounded window and entity keys.
- `internal/live/analyze.go` (new): deltas, attribution, OLS, gating.
- `internal/live/record.go` (new): raw NDJSON recorder and replay helper.
- `internal/live/analyze_test.go`, `sampler_test.go`, `ring_test.go`,
  `record_test.go`.
- `internal/live/testdata/*.ndjson` fixtures.
- `internal/app/app.go`: `sample` subcommand and flags.
- `internal/app/display.go`: usage text.
- `README.md`, `CHANGELOG.md`, `docs/sampling.md` (new).
- `goals/live-sampler-rates/evidence.md`.

### Out of scope

- `internal/picker` and the post-report menu.
- Any terminal input or rendering.
- Changes to `cache`, `proc`, `triage`, `cleaner`, or `model` semantics.
- A second copy of the JSON snapshot schema; this goal imports the shared
  package created by `json-snapshot-output`.

## Requirements

### Functional requirements

**FR-1 — Command and flags.** Add a `sample` subcommand:

```text
swapaudit sample [--once | --stream] [--interval=1s] [--window=60s]
                 [--tmp-interval=30s] [--duration=0] [--record=<path>]
                 [--no-command]
```

- `--once` (default when neither is given) emits one JSON document and exits 0.
- `--stream` emits one NDJSON object per fast tick until a signal or
  `--duration`.
- `--once` and `--stream` are mutually exclusive; exit 2.
- `sample` is always machine-readable; `--json` is accepted as a no-op for
  symmetry and does not change output.
- `--interval` must be within `100ms..60s`; `--window` within `1s..1h`;
  `--tmp-interval` within `1s..10m`. Out-of-range values exit 2 with a message.
- `--window / --interval` must not exceed 600 samples; otherwise exit 2 with
  `error: window/interval exceeds 600 samples; increase --interval`.
- `--duration` is a Go duration; `0` means until a signal. It is ignored by
  `--once`.
- `--record` without `--once` or `--stream` implies `--stream`; with `--once` it
  records exactly one raw sample.
- `sample` implies `--no-update-check`.

**FR-2 — Lanes and cadence.**

| Lane | Cadence | Work |
| :--- | :--- | :--- |
| Fast | `--interval` (1s) | `/proc/meminfo`, `statfs("/tmp")`, `/proc/<pid>/status` (`VmSwap`, `PPid`), `/proc/<pid>/stat` (`starttime`), `/proc/<pid>/comm` |
| Slow | `--tmp-interval` (30s) | `cache.ScanTmpDirectories` in a goroutine; result cached under a mutex, including cleanable byte and item totals |
| On-demand | when VmSwap changes or first seen | `/proc/<pid>/sched` (`se.exec_start`) for idle time |

- The fast lane must never run the recursive `/tmp` walk inline.
- The slow lane must not start a new scan while the previous one is running; it
  skips the cycle and increments `slow_lane_skips`.
- If a fast scan exceeds the interval, skip the next tick and increment
  `skipped_ticks`; never queue ticks.
- The fast lane reuses the cached tmp result and records
  `tmpfs_sampled_age_seconds`.
- The slow lane sums `model.ItemSize` for items with `Cleanable=true` into
  `tmpfs_cleanable_bytes` and counts them into `tmpfs_cleanable_items`. Both are
  exposed in `window` so the dependent notification layer can state the remedy
  without running a second scan or re-deriving categories.

**FR-3 — Entity keys and the ring.**

- Process identity is the tuple `(pid, start_ticks)`. A PID alone is forbidden.
- Cache identity is the absolute path.
- The ring capacity is `ceil(window / interval)`, capped at 600. Each slot stores
  the wall-clock timestamp, monotonic offset, system counters, a
  `map[processKey]int64` of swap bytes, and a `map[string]int64` of cache sizes.
- Only entities with `VmSwap > 0` and cache items matching the existing patterns
  are tracked, mirroring `CollectSwapItems`.
- Ring eviction is FIFO; the oldest slot is dropped when capacity is reached.
- The ring must be safe for one writer (fast lane) and one reader (analyzer);
  use a mutex or a snapshot copy, not an unbounded channel.

**FR-4 — Deltas and attribution.**

For a window of samples `S0..Sn`:

- `system_delta = swap_used(Sn) - swap_used(S0)`.
- `attributed = sum(current - oldest)` for process keys present at both ends.
- `started_bytes = sum(current)` for keys absent at `S0`.
- `exited_bytes = sum(oldest)` for keys absent at `Sn`.
- `unattributed_bytes = system_delta - attributed - started_bytes + exited_bytes`.
- `tmpfs_delta` and `tmpfs_attributed` are computed the same way over cache
  paths.
- `unattributed_bytes` may be negative. It includes shared pages, kernel
  structures, page migration, and sampling skew; never claim exact
  reconciliation.
- Exited entities are never emitted as negative movers; they contribute only to
  `exited_bytes`.
- Movers are ranked by absolute delta descending; ties break by name then key.
- `movers` contains at most 10 process entries; `tmpfs_movers` at most 10 cache
  entries. Both arrays are always present.

**FR-5 — Rate estimator and gating.**

Use ordinary least squares over the window samples for swap used and tmpfs used:

- `slope` in bytes per second; `intercept`; `r2 = 1 - SS_res/SS_tot`.
- `non_negative_ratio` is the fraction of consecutive deltas `>= 0`.
- `trend` is one of `filling`, `recovering`, `flat`, `insufficient`.
- `eta_basis` is one of `ols`, `insufficient_samples`, `short_span`,
  `noisy_fit`, `non_monotone`, `recovering`, `flat`.

Gating rules, applied in order:

1. `samples < 5` → `insufficient`, ETA `null`, basis `insufficient_samples`.
2. `span < 0.8 * window` → `insufficient`, ETA `null`, basis `short_span`.
3. `slope < 0` → `recovering`, ETA `null`, basis `recovering`.
4. `|slope| < 1 KiB/s` → `flat`, ETA `null`, basis `flat`.
5. `r2 < 0.50` → `insufficient`, ETA `null`, basis `noisy_fit`.
6. `non_negative_ratio < 0.70` → `insufficient`, ETA `null`, basis `non_monotone`.
7. Otherwise `filling`; `eta_seconds = free_bytes / slope`, rounded down,
   basis `ols`.

- `free_bytes` is `swap_total - swap_used` at `Sn` for swap and
  `tmpfs_total - tmpfs_used` for tmpfs.
- ETA is `null` for tmpfs when the tmpfs scanner reports a stale sample older
  than `2 * tmp_interval`.
- All statistics are computed from the same window; do not mix a 5-minute rate
  with a 60-second mover table.
- `fit_r2`, `non_negative_ratio`, `samples`, and `span_seconds` are always
  emitted so consumers can judge the estimate.

**FR-6 — Raw trace recording.** `--record=<path>` writes one raw sample per fast
tick:

```json
{"schema_version":1,"type":"raw_sample","sampled_at":"2026-10-21T10:00:00+03:00",
 "monotonic_seconds":12.34,
 "system":{"swap_total_bytes":0,"swap_used_bytes":0,"swap_free_bytes":0,
           "mem_total_bytes":0,"mem_used_bytes":0,"mem_available_bytes":0,
           "tmpfs_total_bytes":0,"tmpfs_used_bytes":0},
 "processes":[{"pid":1420081,"start_ticks":123456,"name":"gopls","swap_bytes":2040109465}],
 "caches":[{"path":"/tmp/go-build998","size_bytes":2576980377,"age_seconds":4}]}
```

- File mode `0600`; flush after each line.
- Recording is independent of `--stream`; both may run together.
- No command lines are recorded, only names and paths.
- `--record` failure exits 1 with a stderr diagnostic.

**FR-7 — Output shapes.**

`--once` (abbreviated but syntactically valid; `snapshot` carries the complete
goal-2 document):

```json
{
  "type": "once",
  "sampled_at": "2026-10-21T10:00:00+03:00",
  "snapshot": {
    "schema_version": 1,
    "tool": {"name": "swapaudit", "version": "dev", "commit": "none", "built": "unknown"},
    "generated_at": "2026-10-21T10:00:00+03:00",
    "uptime_seconds": 123456.78,
    "thresholds": {"max_age_seconds": 43200, "idle_threshold_seconds": 7200},
    "system": {
      "swap_total_bytes": 18321752064, "swap_used_bytes": 16486000000, "swap_free_bytes": 1835752064,
      "mem_total_bytes": 33549176832, "mem_used_bytes": 29585000000, "mem_available_bytes": 3964176832,
      "tmpfs_total_bytes": 16777216000, "tmpfs_used_bytes": 15258000000
    },
    "totals": {
      "items": 0, "process_swap_bytes": 0, "tmpfs_cache_bytes": 0,
      "cleanable_bytes": 0, "cleanable_process_swap_bytes": 0, "cleanable_tmpfs_bytes": 0
    },
    "summary": [],
    "items": [],
    "warnings": []
  },
  "window": {"seconds": 60, "samples": 60, "complete": true}
}
```

`--stream` line:

```json
{"type":"sample","sampled_at":"2026-10-21T10:00:01+03:00","monotonic_seconds":12.34,
 "system":{"swap_total_bytes":18321752064,"swap_used_bytes":16486000000,"swap_free_bytes":1835752064,
           "mem_total_bytes":33549176832,"mem_used_bytes":29585000000,"mem_available_bytes":3964176832,
           "tmpfs_total_bytes":16777216000,"tmpfs_used_bytes":15258000000},
 "window":{"seconds":60,"samples":42,"complete":false,
   "swap":{"trend":"filling","rate_bytes_per_sec":2097152.0,"eta_seconds":3635,
           "eta_basis":"ols","fit_r2":0.97,"non_negative_ratio":1.0},
   "tmpfs":{"trend":"flat","rate_bytes_per_sec":0.0,"eta_seconds":null,
            "eta_basis":"flat","fit_r2":0.0,"non_negative_ratio":1.0},
   "movers":[],"tmpfs_movers":[],
   "unattributed_bytes":0,"exited_bytes":0,"started_bytes":0,
   "tmpfs_sampled_age_seconds":8,"skipped_ticks":0,"slow_lane_skips":0,
   "tmpfs_cleanable_bytes":605617408,"tmpfs_cleanable_items":30}}
```

Final line on shutdown: `{"type":"stopped","reason":"signal","sampled_at":"..."}`.

- `--stream` flushes each line (`bufio.Writer.Flush`).
- No full item array is emitted per stream tick; that is what `--once` is for.
- `type` is always present and is the discriminator.

**FR-8 — Lifecycle.**

- Use `signal.NotifyContext(context.Background(), os.Interrupt,
  syscall.SIGTERM)`; `defer cancel()` immediately after creation.
- Use `time.NewTicker`; `defer ticker.Stop()`.
- The slow lane runs in one goroutine tracked by a `sync.WaitGroup`; the
  sampler waits for it before returning.
- No goroutine may outlive `Run`.
- Clock jumps: use monotonic durations for intervals and span; use wall-clock
  time only for `sampled_at`. Drop a sample whose monotonic delta is `<= 0`.
- On signal, emit the `stopped` line (stream mode), close the recorder, and
  exit 0.

**FR-9 — Cost and memory.**

- Fast lane target: under 100 ms per tick on a 500-process box; measured and
  recorded in evidence.
- Slow lane: at most one `/tmp` walk per `--tmp-interval`; the walk is the
  existing 60-90 ms recursive scan on this workstation.
- Total CPU target for a 60 s run at 1 s cadence: under 15% of one core.
- Ring memory must be bounded by capacity × tracked entities; a test with 1,000
  synthetic entities × 60 slots must stay under 50 MB (assert with
  `runtime.ReadMemStats` in a non-flaky, generous bound, or document the
  calculation instead).

**FR-10 — Shared schema.** Reuse the `Snapshot`, `Item`, and `System` types from
the `json-snapshot-output` package. Do not duplicate or fork the schema. If the
package names differ, the implementation may move the types into
`internal/snapshot` as part of that goal; `internal/live` must import them, and
`internal/app` must not create an import cycle.

**FR-11 — Trace validation harness.** Ship fixtures and table-driven tests:

| Fixture | Shape | Expected |
| :--- | :--- | :--- |
| `idle.ndjson` | 60 samples, swap unchanged | `trend=flat`, `eta=null`, basis `flat` |
| `ramp.ndjson` | 60 samples, +2 MiB/s linear | `trend=filling`, ETA within 10% of analytic |
| `spike.ndjson` | flat then one 500 MiB jump then flat | `eta=null`, basis `noisy_fit` |
| `recovering.ndjson` | 60 samples, -1 MiB/s | `trend=recovering`, `eta=null` |
| `noise.ndjson` | alternating ±20 MiB | `eta=null`, basis `non_monotone` or `noisy_fit` |
| `short.ndjson` | 3 samples | `eta=null`, basis `insufficient_samples` |
| `pid-reuse.ndjson` | same PID, different start_ticks | no phantom delta; old key in `exited_bytes`; new key in `started_bytes` |
| `exit.ndjson` | a process disappears | `exited_bytes > 0`; no negative mover |
| `stale-tmp.ndjson` | tmp sample older than 2× interval | tmpfs `eta=null`, basis `short_span` or stale flag |

The analytic ETA for `ramp.ndjson` is computed in the test from the fixture's
free bytes and slope, not hard-coded.

**FR-12 — Documentation.** Add `docs/sampling.md` covering the lanes, the entity
key, the estimator and its gating, the attribution caveats, the trace format,
and three recipes (statusline, CI threshold, incident capture). Add a README
section and a CHANGELOG entry.

**FR-13 — Exit codes.** `0` normal completion, signal shutdown, and `--once`
success; `2` usage/range/mutual-exclusion errors; `1` recorder or encoder
failure.

**FR-14 — No TUI.** The `sample` subcommand must not import `internal/picker`,
`term` rendering, or any terminal-state package. It may use `term.FormatBytes`
only in documentation examples, not in output.

### Non-functional requirements

- **NFR-1 — Dependency floor.** Standard library only; Linux build tag; Go 1.23;
  no `golang.org/x/sys`.
- **NFR-2 — Size ceiling.** `internal/live` production code at most 500 lines;
  tests at most 550; no file approaches 1,000 lines.
- **NFR-3 — Determinism.** `Analyze` is pure and deterministic for a given
  sample slice. `--once` with an injected clock and source is byte-identical
  across runs.
- **NFR-4 — No leaks.** A goroutine-count test asserts no growth after
  cancellation.
- **NFR-5 — Bounded memory.** The ring capacity and entity tracking are capped;
  no unbounded slices or maps.
- **NFR-6 — Privacy.** Raw traces contain process names and cache paths, not
  command lines. Record files are `0600`. Document that traces may still reveal
  workflow.
- **NFR-7 — Portability.** All sampling code is Linux-only; the JSON schema
  contains no syscall types.
- **NFR-8 — Operability.** Errors go to stderr; stdout carries only JSON.

## Acceptance Criteria

1. `go build ./...`, `go vet ./...`, `go test -race ./...` pass.
2. `./scripts/check.sh --full` passes.
3. `swapaudit sample --once` emits valid JSON with `type: "once"`, a goal-2
   `snapshot`, and a `window` block including `tmpfs_cleanable_bytes` and
   `tmpfs_cleanable_items`.
4. `swapaudit sample --stream --interval=100ms --duration=1s` emits at least
   three valid JSON lines and one `stopped` line.
5. Every fixture in FR-11 passes its expected trend/basis/ETA assertion.
6. `ramp.ndjson` ETA is within 10% of the analytic value.
7. PID reuse produces no phantom mover and reports the retired key in
   `exited_bytes`.
8. A process exit is never rendered as a negative mover.
9. `--window/--interval > 600` exits 2 with the exact message.
10. `--once --stream` exits 2; out-of-range interval/window exits 2.
11. A 60-second stream run uses under 15% of one core, measured and recorded.
12. Cancellation closes the recorder, emits `stopped`, and leaves no goroutine
    growth.
13. `--record` writes `0600` NDJSON that the replay helper can feed back into
    `Analyze` with identical results.
14. `internal/live` stays within the NFR-2 ceiling.
15. `docs/sampling.md`, README, and `PrintUsage` document the command, schema,
    gating, and recipes.
16. `sample` emits no ANSI escapes and no human prose.

## Evidence Requirements

`goals/live-sampler-rates/evidence.md` with a standalone `Verdict: PASS` and:

- `go test -race ./...` and `./scripts/check.sh --full` output;
- a fixture-by-fixture table of `trend`, `eta_seconds`, and `eta_basis`;
- the analytic-versus-measured ETA calculation for `ramp.ndjson`;
- a PID-reuse trace excerpt and the resulting `exited_bytes`/`started_bytes`;
- a 60-second `--stream` capture with line count and validity check;
- a CPU measurement for the 60-second run (`time` builtin or `/proc/self/stat`
  deltas);
- a goroutine-count test result;
- a `--record` file with `stat -c '%a'` showing `600`, plus a replay test;
- `git diff --stat` and line-count checks;
- confirmation that no TUI or terminal-state code was added.

## Dependencies & Execution Order

- **Mode**: Dependent (requires prerequisite goals)
- **Depends On**: json-snapshot-output
- **Sequence**: 3
- **Shape**: ship
- **Tier**: roadmap

## Files to Touch

- internal/live/sampler.go
- internal/live/ring.go
- internal/live/analyze.go
- internal/live/record.go
- internal/live/analyze_test.go
- internal/live/sampler_test.go
- internal/live/ring_test.go
- internal/live/record_test.go
- internal/live/testdata/idle.ndjson
- internal/live/testdata/ramp.ndjson
- internal/live/testdata/spike.ndjson
- internal/live/testdata/recovering.ndjson
- internal/live/testdata/noise.ndjson
- internal/live/testdata/short.ndjson
- internal/live/testdata/pid-reuse.ndjson
- internal/live/testdata/exit.ndjson
- internal/live/testdata/stale-tmp.ndjson
- internal/app/app.go
- internal/app/display.go
- docs/sampling.md
- README.md
- CHANGELOG.md
- goals/live-sampler-rates/evidence.md

## Persona & Audit Plan

- **Persona 1 — agent-readiness-architect:** machine consumer ergonomics, stable
  keys, stream framing, and the cost of each tick for agent statuslines.
- **Persona 5 — senior-fullstack-engineer:** goroutine lifecycle, ring safety,
  estimator correctness, error paths, and import boundaries.
- **Persona 32 — mystery-auditor:** try to fabricate a mover with a reused PID,
  produce a confident ETA from noise, leak a goroutine on SIGTERM, overflow the
  ring, emit a negative mover on exit, and write a world-readable trace.
- **Commands:** `sila persona get 1`, `sila persona get 5`,
  `sila persona get 32`, `sila hire assess 1,5,32`.
- **Artifacts:** `goals/live-sampler-rates/evidence.md` (or a sibling
  `persona-review.md`).

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation | Test |
| :--- | :---: | :---: | :--- | :--- |
| ETA looks authoritative but is noise | High | High | Gating on samples, span, R², monotonicity; nullable ETA | Fixture matrix |
| PID reuse fabricates a mover | Medium | High | Key `(pid, start_ticks)`; retire old keys | `pid-reuse.ndjson` |
| Negative movers on exit | Medium | Medium | Separate `exited_bytes`; never emit negative movers | `exit.ndjson` |
| Attribution claimed as exact | Medium | Medium | Emit `unattributed_bytes`; document shared pages | Attribution test |
| Slow `/tmp` walk blocks ticks | Medium | High | Separate goroutine, cached result, skip overlap | Timing test |
| Tick backlog under load | Medium | Medium | Skip ticks, increment `skipped_ticks` | Synthetic slow source test |
| Goroutine leak on signal | Low | High | `WaitGroup`, `NotifyContext`, `defer cancel()` | Leak test |
| Ring memory blowup | Low | Medium | Capacity cap 600, track only swap>0 items | Bounded-memory test |
| Clock jump on suspend | Medium | Medium | Monotonic duration for span; drop non-positive deltas | Clock-jump test |
| Trace leaks workflow details | Medium | Medium | Names/paths only, `0600`, documented | Mode check |

## Open Questions / Deferred Decisions

- Should `--stream` include a periodic full snapshot every N ticks? Deferred;
  consumers can call `--once` on demand.
- Should the estimator use Theil-Sen instead of OLS for spike resistance?
  Deferred; OLS plus R² gating passes the fixture matrix. Revisit if field
  traces show false positives.
- Should `unattributed_bytes` be split into shared/kernel/exited buckets?
  Deferred until a trace can attribute them defensibly.
- Should the sampler write a history file? Deferred; recording is opt-in and
  bounded.
- Should tmpfs movers be merged with process movers in one ranked list?
  Deferred; mixing RAM-backed cache bytes with swap bytes is misleading.

## References

- **Shared Understanding & Fact Sheet**:
  [`goals/live-sampler-rates/facts.md`](facts.md)
- **Execution Plan**: [`goals/live-sampler-rates/plan.md`](plan.md)
- `internal/proc/proc.go` — `ReadSystemSwap`, `ReadProcessLastActive`,
  `ReadStartTicks`.
- `internal/cache/cache.go` — the recursive `/tmp` scanner and patterns.
- `internal/app/app.go` — `CollectSwapItems` and the scan inputs.
- `goals/json-snapshot-output/goal.md` — shared snapshot schema.
- `goals/one-shot-action-picker/goal.md` — the separate interactive surface.
- `/proc/meminfo`, `/proc/[pid]/status`, `/proc/[pid]/stat`,
  `/proc/[pid]/sched`, `statfs(2)`.

## Done Condition

1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the
   `implemented` transition.
2. Implementation is committed with `(goals/live-sampler-rates/goal.md)` or
   `[goal: live-sampler-rates]` attribution under a `feat:`, `fix:`,
   `refactor:`, `perf:`, or `chore:` prefix.
3. Goal is certified (`goals/live-sampler-rates/certification.json` with
   `verified: true` or a signed `audit-receipt.json`) and any markdown evidence
   contains an explicit standalone `Verdict: PASS` declaration.
4. `sila goals --json` reports `status: "implemented"`,
   `verification_status: "verified"`, and `unverified_count: 0` for delivered
   goals, with a clean `git status`.
5. `go test -race ./...`, `./scripts/check.sh --full`, `sila facts verify`, and
   `sila gate` pass.
