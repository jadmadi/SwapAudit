# Facts: Add a Headless Sampler with Rates, Attribution, and a Gated ETA

## 1. Grounded Evidence (live, 2026-10-21)

### 1.1 What the current scan costs and covers

- Five consecutive full scans on this workstation measured 92, 109, 112, 162,
  and 144 ms.
- The same scan observed 491 processes, 1,546 `/tmp` entries, and 37,655 files.
- A shell `find /tmp -xdev -type f` over that tree took 63 ms, so the recursive
  walk is the dominant component; the `/proc` work is tens of milliseconds.
- `CollectSwapItems` performs the `/tmp` walk first (`app.go:45`), then iterates
  `/proc` (`47-112`). Reading `cmdline`, `comm`, `stat`, and `sched` for every
  PID is the expensive part of the process lane.
- A real report on this box showed 577.7 MB cleanable: 367.5 MB untouched
  caches, 12.4 MB abandoned runs, 197.9 MB dormant agents, 0 orphaned.

### 1.2 The data sources and their semantics

- `proc.ReadUptime` reads the first field of `/proc/uptime` (`proc.go:19-30`).
- `proc.ReadSystemSwap` reads `/proc/meminfo` and `statfs("/tmp")`
  (`proc.go:60-76`). `ParseMeminfo` derives `SwapUsed = SwapTotal - SwapFree`
  and `MemUsed = MemTotal - MemAvail` (`33-57`).
- `proc.ReadProcStatus` returns `VmSwap` and `PPid` from
  `/proc/<pid>/status` (`109-116`, `79-100`).
- `proc.ReadStartTicks` parses field 22 (`starttime`) from `/proc/<pid>/stat`
  after the last `)` (`163-182`). It assumes `USER_HZ = 100`, which is standard
  on Linux.
- `proc.ReadProcessLastActive` scans `/proc/<pid>/sched` for `se.exec_start` and
  returns `-1` when the file is unreadable (`217-239`). It compares the value to
  uptime in milliseconds.
- `proc.ReadCmdlineArgs` reads `/proc/<pid>/cmdline` (`134-147`).
- `proc.ReadComm` reads `/proc/<pid>/comm` (`125-131`).
- `proc.IsProcessOrphaned` treats `ppid <= 1` or a missing/systemd/init parent
  as orphaned (`203-213`).
- `cache.ScanTmpDirectoriesIn` walks matching directories recursively
  (`cache.go:116-135`) and uses `Lstat`, UID, and symlink checks (`72-89`).
- `triage.CategorizeProcess` assigns the category and `Cleanable` from command
  name, argv, PID/PPID, orphan state, idle time, and the idle threshold.

### 1.3 Process identity and lifecycle hazards

- A PID is not a stable identity. Linux reuses PIDs after wraparound, so a
  `(pid)` key can merge two different processes.
- `(pid, start_ticks)` is stable for a process lifetime and is already used by
  `cleaner.KillProcessTargets` to reject PID reuse (`cleaner.go:152-160`).
- When a process exits, its `VmSwap` disappears from `/proc`. A naive
  current-minus-previous sum therefore produces a negative delta that looks like
  memory creation.
- Shared pages (shmem/tmpfs, shared libraries, `fork` COW) mean one process's
  exit does not necessarily release its full mapped memory.
- The kernel swaps pages back in under pressure, so system swap usage is not
  monotone and can fall without user action.

### 1.4 Concurrency and lifecycle facts

- The only goroutine in the codebase today is the update check in `app.Run`
  (`app.go:187-200`), which uses `context.WithTimeout` and a done channel.
- There is no `signal.Notify`, ticker, or `sync.WaitGroup` anywhere in the
  repository.
- `cleaner` and `app` are synchronous; adding a slow-lane goroutine is a new
  lifecycle surface and must be paired with cancellation and a wait.
- `go.mod` is `go 1.23.0` with no dependencies, so `golang.org/x/sys/unix` is
  unavailable; use `syscall` and the existing `//go:build linux` convention.

### 1.5 Terminal and output facts

- `term.GetTerminalWidth` clamps to 80-120 and defaults to 100
  (`term.go:23-46`); the sampler must not depend on width.
- `term.InitTerminal` disables color for non-TTY stdout (`49-72`); the sampler
  emits no color at all.
- `app.Run` currently prints human output unconditionally after the scan
  (`221-242`), so the `sample` subcommand must dispatch before any renderer.
- `version.Version`, `version.Commit`, and `version.Date` are the build metadata
  used by the shared snapshot schema.

### 1.6 Import-boundary fact

- `internal/app` owns `Run` and `CollectSwapItems`. If the sampler lives in
  `internal/live` and needs the snapshot types from `internal/app`, and
  `internal/app` imports `internal/live` to wire the subcommand, that is an
  import cycle.
- Therefore the `json-snapshot-output` schema and builder must live in a neutral
  package (recommended: `internal/snapshot`). `internal/app` and `internal/live`
  both import it; neither imports the other.
- This goal depends on `json-snapshot-output` and must not fork the schema.

### 1.7 Estimator grounding

- Ordinary least squares slope over `(t_seconds, bytes)` is
  `sum((t-tbar)(y-ybar)) / sum((t-tbar)^2)`.
- `R² = 1 - SS_res/SS_tot`; when `SS_tot == 0`, the series is flat and `R²` is
  defined as `1`.
- Gate thresholds chosen for this goal: at least 5 samples, span at least 80% of
  the requested window, `R² >= 0.50`, non-negative delta ratio `>= 0.70`, and a
  positive slope. These are policy constants and must be named, not inlined.
- A single large jump can pass a monotonicity check while failing the fit; both
  gates are required.
- A negative slope is a real state (`recovering`), not an error, and must not
  produce a negative ETA.
- Swap is measured in bytes; rates are bytes per second; ETAs are seconds.

### 1.8 Cost model for the design

- Fast lane per tick: `meminfo` + `statfs` + per-PID `status`/`stat`/`comm`.
  Based on the full-scan measurements, this is well under 100 ms on a
  500-process box.
- Slow lane: the recursive `/tmp` walk, 60-90 ms, every 30 s, off the tick
  thread. Average cost is under 0.3% of a core.
- At 1 s cadence, the target upper bound is 15% of one core; the evidence must
  measure it rather than assume it.
- Ring memory: 60 slots × ~900 tracked entities ≈ 54,000 map entries; at roughly
  50-100 bytes per map entry this is a few megabytes. The 600-slot cap bounds
  the worst case but should be documented as a memory/latency trade-off.

## 2. Architectural Invariants & Constraints

- Process identity is `(pid, start_ticks)`; cache identity is the absolute path.
- The sampler never blocks the fast tick on the `/tmp` walk.
- No goroutine outlives `Run`; every ticker is stopped; every context is
  cancelled with a deferred call.
- The analyzer is pure and deterministic; it takes samples and returns stats.
- ETA is nullable and gated. A confident-looking number is worse than `null`.
- Exited entities are never negative movers; they are reported separately.
- `unattributed_bytes` is explicitly not zero-sum; document shared pages and
  kernel effects.
- No terminal state, no input handling, no color, no human prose on stdout.
- Raw traces contain names and paths, never command lines; files are `0600`.
- The window and capacity are bounded; no unbounded growth.
- Monotonic time is used for spans; wall-clock time is used only for
  `sampled_at`.

## 3. File & Interface Contracts

### `internal/live/ring.go`

```go
type processKey struct {
    PID        int
    StartTicks int64
}

type sample struct {
    At          time.Time
    MonoSeconds float64
    System      model.SystemSwap
    Processes   map[processKey]int64
    Caches      map[string]int64
}

type Ring struct {
    capacity int
    items    []sample
}

func NewRing(capacity int) *Ring
func (r *Ring) Push(s sample)
func (r *Ring) Snapshot() []sample
func (r *Ring) Len() int
```

- `Push` evicts the oldest slot at capacity.
- `Snapshot` returns a copy safe for the analyzer.

### `internal/live/analyze.go`

```go
type Trend string // "filling", "recovering", "flat", "insufficient"

type SeriesStats struct {
    Trend              Trend    `json:"trend"`
    RateBytesPerSec    float64  `json:"rate_bytes_per_sec"`
    ETASeconds         *int64   `json:"eta_seconds"`
    ETABasis           string   `json:"eta_basis"`
    FitR2              float64  `json:"fit_r2"`
    NonNegativeRatio   float64  `json:"non_negative_ratio"`
    Samples            int      `json:"samples"`
    SpanSeconds        float64  `json:"span_seconds"`
}

type Mover struct {
    Kind        string `json:"kind"` // "process" | "cache"
    PID         *int   `json:"pid"`
    StartTicks  *int64 `json:"start_ticks"`
    Name        string `json:"name"`
    Path        *string `json:"path"`
    DeltaBytes  int64  `json:"delta_bytes"`
    CurrentBytes int64 `json:"current_bytes"`
    Started     bool   `json:"started"`
    Exited      bool   `json:"exited"`
}

type WindowStats struct {
    Seconds              int          `json:"seconds"`
    Samples              int          `json:"samples"`
    Complete             bool         `json:"complete"`
    Swap                 SeriesStats  `json:"swap"`
    Tmpfs                SeriesStats  `json:"tmpfs"`
    Movers               []Mover      `json:"movers"`
    TmpfsMovers          []Mover      `json:"tmpfs_movers"`
    UnattributedBytes    int64        `json:"unattributed_bytes"`
    ExitedBytes          int64        `json:"exited_bytes"`
    StartedBytes         int64        `json:"started_bytes"`
    TmpfsSampledAgeSec   int64        `json:"tmpfs_sampled_age_seconds"`
    SkippedTicks         int          `json:"skipped_ticks"`
    SlowLaneSkips        int          `json:"slow_lane_skips"`
    TmpfsCleanableBytes  int64        `json:"tmpfs_cleanable_bytes"`
    TmpfsCleanableItems  int          `json:"tmpfs_cleanable_items"`
}

func Analyze(samples []sample, window time.Duration, tmpfsSampledAt time.Time, now time.Time) WindowStats
```

- `Analyze` must return empty slices, not `nil`.
- `ETASeconds` is a pointer; `nil` is the only representation of "unknown".
- All policy thresholds are named constants in this file.

### `internal/live/sampler.go`

```go
type Config struct {
    Interval     time.Duration
    Window       time.Duration
    TmpInterval  time.Duration
    Duration     time.Duration
    RecordPath   string
    NoCommand    bool
    Now          func() time.Time
    ScanFast     func(now time.Time) sample
    ScanTmp      func() []model.SwapItem
    Out          io.Writer
    Clock        func() time.Time // wall clock for sampled_at
    Mono         func() time.Time // monotonic source for span
}

func Run(ctx context.Context, cfg Config) error
```

- `Run` returns `nil` on signal or duration completion, and a wrapped error on
  recorder/encoder failure.
- The slow lane is started once and waited on before return.
- The fast lane skips a tick rather than queueing when the previous scan
  overruns.

### `internal/live/record.go`

- `Recorder` writes the FR-6 raw line with `json.Encoder` and `SetEscapeHTML(false)`.
- Opens with `os.OpenFile(path, O_CREATE|O_WRONLY|O_TRUNC, 0600)`.
- `Flush` after each line.
- `Replay(r io.Reader) ([]sample, error)` parses raw lines for tests.

### `internal/app/app.go`

- Add `sample` to the bare-word subcommand switch.
- Dispatch before `term.InitTerminal` and before `CollectSwapItems`.
- Forward `--no-command` to the sampler.
- Keep `help` and `upgrade` behavior unchanged.

## 4. External Contracts & Failure Modes

| Condition | Detection | Expected behavior |
| :--- | :--- | :--- |
| `--once` and `--stream` together | flag validation | Exit 2 |
| interval outside `100ms..60s` | validation | Exit 2 |
| window outside `1s..1h` | validation | Exit 2 |
| `window/interval > 600` | validation | Exit 2, exact message |
| `/proc/[pid]/sched` unreadable | reader returns `-1` | Idle `null`; no ETA impact |
| PID reused | start ticks differ | New key; old key retired |
| Process exits | key absent at end | `exited_bytes`, not a negative mover |
| Swap falls | OLS slope < 0 | `recovering`, ETA `null` |
| Spike | low R² | `noisy_fit`, ETA `null` |
| Few samples | `< 5` | `insufficient_samples`, ETA `null` |
| Stale tmp scan | `now - tmpfsSampledAt > 2*TmpInterval` | tmpfs ETA `null` |
| Slow scan overruns tick | ticker fired while busy | Skip tick, increment counter |
| SIGINT/SIGTERM | `NotifyContext` | Emit `stopped`, flush, exit 0 |
| Record path unwritable | open error | Exit 1, stderr |
| Broken pipe on stream | write error | Exit 1, stop goroutines |
| Clock jump | monotonic delta `<= 0` | Drop sample, increment counter |

## 5. Verification Matrix

| Test | Command | Expected |
| :--- | :--- | :--- |
| Unit + fixtures | `go test -race ./internal/live` | Pass |
| Full race | `go test -race ./...` | Pass |
| `--once` | `go run ./cmd/swapaudit sample --once \| python3 -m json.tool` | Valid JSON |
| Stream | `go run ./cmd/swapaudit sample --stream --interval=100ms --duration=1s` | 3+ samples + stopped |
| Fixture matrix | `go test ./internal/live -run TestAnalyzeFixtures` | All expectations |
| Purity | `go test ./internal/live -run TestAnalyzeDeterministic` | Byte-identical |
| Leak | `go test ./internal/live -run TestNoGoroutineLeak` | No growth |
| Record mode | `stat -c '%a' trace.ndjson` | `600` |
| Replay | `go test ./internal/live -run TestReplay` | Identical stats |
| CPU | 60 s run with `time` | Under 15% of one core |
| Bounds | `--window=1h --interval=1s` | Exit 2 |
| No ANSI | capture stdout | No `\x1b` |
| Gate | `./scripts/check.sh --full` | Pass |

## 6. Open Questions / Deferred Decisions

- Whether OLS is sufficiently spike-resistant versus Theil-Sen. The fixture
  matrix gates this decision; revisit with field traces.
- Whether to expose `intercept` or a predicted-fill timestamp in v1. Deferred;
  `eta_seconds` is the consumer-facing number.
- Whether `--stream` should include a full snapshot every N ticks. Deferred.
- Whether to split `unattributed_bytes` into shared/kernel/exited buckets.
  Deferred until a trace can attribute them defensibly.
- Whether to persist the ring across restarts. Explicitly out of scope.
- Whether a future TUI should consume this engine directly. Yes, but that is a
  separate goal with its own input and rendering contracts.

## 7. References

- `internal/proc/proc.go`
- `internal/cache/cache.go`
- `internal/triage/triage.go`
- `internal/app/app.go`
- `internal/cleaner/cleaner.go` (PID-reuse precedent)
- `goals/json-snapshot-output/goal.md` (schema dependency)
- `goals/one-shot-action-picker/goal.md` (separate surface)
- Linux `proc(5)` for `/proc/[pid]/stat`, `status`, and `sched`
