# Headless sampling, rates, and ETA

`swapaudit sample` is the measurement engine behind the one-shot report. It
answers three questions a snapshot cannot: how fast swap or tmpfs is filling,
which entities are responsible, and how long until the resource is exhausted.

It is deliberately headless: no TUI, no key handling, no terminal state, no
color. Stdout carries only JSON or NDJSON, so a statusline, CI job, or future
interactive view can consume it directly.

```bash
swapaudit sample --once | jq .
swapaudit sample --stream --interval=1s --record=/tmp/swap-trace.ndjson
```

## Flags and bounds

| Flag | Default | Bound | Meaning |
| :--- | :--- | :--- | :--- |
| `--once` | on when neither mode is given | - | Emit one JSON document and exit. |
| `--stream` | off | - | Emit one NDJSON line per tick. |
| `--interval` | `1s` | `100ms..60s` | Fast-lane cadence. |
| `--window` | `60s` | `1s..1h` | Estimator window. |
| `--tmp-interval` | `30s` | `1s..10m` | Slow-lane `/tmp` cadence. |
| `--duration` | `0` | non-negative | Stop after this duration; `0` means until a signal. |
| `--record` | off | - | Write raw NDJSON samples with mode `0600`. |
| `--no-command` | off | - | Omit command lines from the `--once` snapshot. |
| `--json` | - | - | Accepted no-op; `sample` is always machine-readable. |

`--once` and `--stream` are mutually exclusive. `window / interval` must not
exceed 600 samples. `--record` without either mode implies `--stream`; with
`--once` it records exactly one sample. `sample` never runs the update check.

## Lanes

| Lane | Cadence | Work |
| :--- | :--- | :--- |
| Fast | `--interval` | `/proc/meminfo`, `statfs("/tmp")`, per-PID `status`, `stat`, `comm`; `sched` only on first observation or when `VmSwap` changed. |
| Slow | `--tmp-interval` | The recursive `/tmp` walk in one goroutine; the result is cached under a mutex and reused by every fast tick. |
| On demand | per process | `/proc/<pid>/sched` for idle time, only when needed. |

The fast lane never blocks on the recursive walk. If a fast scan exceeds the
interval, the next tick is skipped and `skipped_ticks` increments; ticks are
never queued. If a slow scan exceeds its interval, `slow_lane_skips` increments.

## Entity identity

- A process is identified by `(pid, start_ticks)`. A PID alone is forbidden:
  Linux reuses PIDs, and a recycled PID would fabricate a multi-gigabyte mover.
- A cache is identified by its absolute path.
- When a process exits, its key disappears and its last bytes go to
  `exited_bytes`, never to a negative mover.
- When a PID is reused, the old key is retired and the new key appears in
  `started_bytes`.

## Estimator and gating

Both swap and tmpfs use ordinary least squares over the window:

- `rate_bytes_per_sec` is the slope.
- `fit_r2 = 1 - SS_res / SS_tot`; a flat series is defined as `R² = 1`.
- `non_negative_ratio` is the fraction of consecutive deltas `>= 0`.

Gates apply in this order:

1. Fewer than 5 samples: `insufficient`, basis `insufficient_samples`.
2. Span below 80% of `--window`: `insufficient`, basis `short_span`.
3. Negative slope: `recovering`, basis `recovering`.
4. Absolute slope below 1 KiB/s: `flat`, basis `flat`.
5. `R² < 0.50`: `insufficient`, basis `noisy_fit`.
6. `non_negative_ratio < 0.70`: `insufficient`, basis `non_monotone`.
7. Otherwise: `filling`, basis `ols`, and
   `eta_seconds = free_bytes / slope`, rounded down.

`eta_seconds` is nullable. A null ETA is a deliberate answer, not a failure:
the series was too short, too noisy, non-monotone, recovering, or flat. The
stats always include `fit_r2`, `non_negative_ratio`, `samples`, and
`span_seconds` so consumers can judge the estimate.

`free_bytes` is `swap_total - swap_used` at the last sample, and
`tmpfs_total - tmpfs_used` for tmpfs. Tmpfs ETA is null when the slow-lane
sample is older than 60 seconds.

## Attribution

For a window from `S0` to `Sn`:

```
system_delta       = swap_used(Sn) - swap_used(S0)
attributed         = sum(current - oldest) for keys present at both ends
started_bytes      = sum(current) for keys absent at S0
exited_bytes       = sum(oldest)  for keys absent at Sn
unattributed_bytes = system_delta - attributed - started_bytes + exited_bytes
```

`unattributed_bytes` is not a bug and is not zero-sum. It includes shared
pages, shmem/tmpfs interactions, kernel structures, page migration, and sampling
skew. Never present attribution as exact reconciliation.

Movers are ranked by absolute delta, then name, then key. `movers` and
`tmpfs_movers` carry at most 10 entries each and are always present.

## Output shapes

`--once`:

```json
{
  "type": "once",
  "sampled_at": "2026-10-21T10:00:00+03:00",
  "snapshot": { "schema_version": 1, "...": "the full JSON snapshot" },
  "window": {
    "seconds": 60,
    "samples": 1,
    "complete": false,
    "swap": {"trend": "insufficient", "rate_bytes_per_sec": 0, "eta_seconds": null,
             "eta_basis": "insufficient_samples", "fit_r2": 0, "non_negative_ratio": 1,
             "samples": 1, "span_seconds": 0},
    "tmpfs": {"trend": "insufficient", "...": "..."},
    "movers": [],
    "tmpfs_movers": [],
    "unattributed_bytes": 0,
    "exited_bytes": 0,
    "started_bytes": 0,
    "tmpfs_sampled_age_seconds": 0,
    "skipped_ticks": 0,
    "slow_lane_skips": 0,
    "tmpfs_cleanable_bytes": 605617408,
    "tmpfs_cleanable_items": 30
  }
}
```

`--stream` emits one compact `{"type":"sample", ...}` line per tick with
`sampled_at`, `monotonic_seconds`, `system`, and `window`. Shutdown emits one
`{"type":"stopped","reason":"signal"|"duration","sampled_at":"..."}` line.
Lines are flushed individually so a pipeline sees each tick.

## Raw trace format

`--record=<path>` writes one JSON object per fast tick, mode `0600`, flushed
after each line:

```json
{"schema_version":1,"type":"raw_sample","sampled_at":"2026-10-21T10:00:00+03:00",
 "monotonic_seconds":12.34,
 "system":{"swap_total_bytes":0,"swap_used_bytes":0,"swap_free_bytes":0,
           "mem_total_bytes":0,"mem_used_bytes":0,"mem_available_bytes":0,
           "tmpfs_total_bytes":0,"tmpfs_used_bytes":0},
 "processes":[{"pid":1420081,"start_ticks":123456,"name":"gopls","swap_bytes":204010945}],
 "caches":[{"path":"/tmp/go-build998","size_bytes":257698037,"age_seconds":4}],
 "tmpfs_sampled_at":"2026-10-21T09:59:52+03:00",
 "tmpfs_cleanable_bytes":605617408,"tmpfs_cleanable_items":30}
```

Traces contain process names and cache paths, never command lines. They can
still reveal workflow details; store them accordingly. The replay helper parses
the same format and feeds `Analyze` with identical results.

## Bounds and memory

The ring holds at most 600 slots. Only processes with `VmSwap > 0` and cache
paths matching the scan patterns are tracked. On a 500-process, 1,500-cache box
that is about 2,000 map entries per slot; at roughly 100 bytes per entry, the
default 60-slot window is about 12 MB. The hard 600-slot cap bounds the worst
case at roughly 120 MB regardless of runtime, and no slice grows without a
bound. This is the documented calculation required in place of a flaky
`runtime.ReadMemStats` assertion.

## Recipes

### Shell statusline

```bash
swapaudit sample --once |
  jq -r '.window | "swap \(.swap.trend) \(.swap.rate_bytes_per_sec/1048576|floor) MiB/s eta \(.swap.eta_seconds // "--")s"'
```

### CI threshold

Fail when the projected swap fill time is under 10 minutes:

```bash
set -e
swapaudit sample --once --duration=0 > sample.json
python3 - <<'PY'
import json
w = json.load(open("sample.json"))["window"]
eta = w["swap"]["eta_seconds"]
if eta is not None and eta < 600:
    raise SystemExit(f"swap ETA {eta}s is under the 600s budget")
PY
```

### Incident capture

```bash
swapaudit sample --stream --interval=1s --record=/tmp/swap-$(date +%s).ndjson
```

The trace can be replayed with the `internal/live` `Replay` helper to reproduce
the same trends, movers, and ETAs offline.
