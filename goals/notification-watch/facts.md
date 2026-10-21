# Facts: Add a Foreground Watch Mode with Reliable Threshold Notifications

## 1. Grounded Evidence (live, 2026-10-21)

### 1.1 Notification surface on this workstation

Command and environment probe:

| Item | Result |
| :--- | :--- |
| `notify-send` | **absent** |
| `gdbus` | `/usr/bin/gdbus` |
| `dbus-send` | `/usr/bin/dbus-send` |
| `kdialog` | `/usr/bin/kdialog` |
| `systemd-run` | `/usr/bin/systemd-run` |
| `tmux` | `/usr/bin/tmux` |
| `wall` | `/usr/bin/wall` |
| `DBUS_SESSION_BUS_ADDRESS` | `unix:path=/run/user/1000/bus` |
| `DISPLAY` | `:1` |
| `WAYLAND_DISPLAY` | `wayland-0` |
| `XDG_CURRENT_DESKTOP` | `KDE` |
| `XDG_SESSION_TYPE` | `wayland` |
| `TERM` | `xterm-256color` |
| `TMUX` | unset |

Implication: a hardcoded `notify-send` call would fail silently on the primary
development machine. The transport must be probed, and a stderr fallback is
mandatory. `wall` exists but writes to every TTY and is rejected as noisy.

### 1.2 The codebase has no notification or subprocess layer

- `grep -rn '"os/exec"\|exec.Command' --include='*.go' .` finds only
  `cmd/swapaudit/main_test.go`, which builds and runs the test binary. There is
  no production subprocess usage.
- There is no notification, D-Bus, OSC, or desktop-integration code.
- The only user-facing notice is `term.PrintUpdateNotice`
  (`internal/term/term.go:542-576`), which prints a box to stdout. It is not a
  notification transport and must not be reused.
- This goal introduces the first production `os/exec` usage. The repository's
  zero-third-party-dependency rule still holds: helpers are external binaries,
  not Go modules.

### 1.3 The sampler contract this goal consumes

From `goals/live-sampler-rates`:

- `WindowStats.Swap` and `WindowStats.Tmpfs` are `SeriesStats` with `Trend`,
  `RateBytesPerSec`, `ETASeconds` (nullable), `ETABasis`, `FitR2`,
  `NonNegativeRatio`, `Samples`, and `SpanSeconds`.
- `WindowStats.Movers` and `TmpfsMovers` carry `DeltaBytes`, `CurrentBytes`,
  `Started`, `Exited`, and identity fields.
- `UnattributedBytes`, `ExitedBytes`, and `StartedBytes` are already separated.
- `TmpfsSampledAgeSec`, `SkippedTicks`, and `SlowLaneSkips` expose lane health.
- `TmpfsCleanableBytes` and `TmpfsCleanableItems` come from the slow lane, so
  the notification can state the remedy without a second scan.
- The estimator returns `eta_seconds: null` when samples are insufficient, the
  span is short, the fit is noisy, the series is non-monotone, or the trend is
  recovering or flat. The pressure fallback exists because `null` is common
  during the bursts that matter.

### 1.4 Why the trigger cannot be a single comparison

- ETA is derived from a window; at a threshold crossing it flaps between just
  above and just below as samples enter and leave the window.
- Swap can be reclaimed by the kernel mid-window, so danger can clear without
  user action.
- A single large allocation can produce a one-tick spike that clears
  immediately.
- The estimator intentionally returns `null` when the fit is poor, which is
  exactly when a burst is most dangerous.
- Therefore the design needs sustain (time in danger), hysteresis (a higher
  re-arm threshold), cooldown (time between incidents), and an absolute-pressure
  fallback (independent of the fit).

### 1.5 Transport mechanics

- `notify-send` is a libnotify wrapper. Its flags in FR-8 are stable across
  GNOME/KDE/XFCE desktops.
- `gdbus` calls the freedesktop notification service:
  `org.freedesktop.Notifications.Notify(app_name s, replaces_id u, app_icon s,
  summary s, body s, actions as, hints a{sv}, expire_timeout i)`.
  The `[]` and `{}` GVariant literals are accepted for actions and hints.
- `dbus-send` requires explicit type prefixes:
  `string:`, `uint32:`, `int32:`, `array:string:`, `dict:string:string:`.
  The empty dict syntax is the most fragile part of the chain; fallback to
  `kdialog` or stderr covers it.
- `kdialog --passivepopup <text> <seconds>` works on KDE and is non-modal.
- OSC 777 is `ESC ] 777 ; notify ; title ; body BEL`. It is best-effort:
  terminal support varies, tmux may require `set -g allow-passthrough on`, and
  it requires a controlling terminal (`/dev/tty`).
- `stderr` is the only transport guaranteed to exist in every environment,
  including CI and SSH without X/Wayland.

### 1.6 Security facts

- `exec.CommandContext` does not invoke a shell; it passes argv directly. This
  is the only safe way to support `--notify-cmd`.
- `sh -c` with a notification body would make process names and paths an
  injection vector. It is forbidden.
- A notification body can appear on a lock screen. It must contain names and
  paths only, never command lines or arguments.
- `--notify-cmd` is an explicit opt-in; it must not be read from an environment
  variable by default.

### 1.7 Lifecycle facts

- The sampler already owns tickers, `signal.NotifyContext`, and one slow-lane
  goroutine. Watch must not add another polling loop.
- A subprocess can hang; without a timeout it would stall the sender. Use a
  3-second context deadline and a buffered channel so sampling never blocks.
- The repository has no state directory contract. A daemon would need one for
  pid/lock/log; a foreground watch needs none.

## 2. Architectural Invariants & Constraints

- Watch consumes the sampler; it does not re-implement sampling, rates, or
  gating.
- The sampling loop must never execute a subprocess or block on delivery.
- One sender goroutine, buffered channel capacity 4, non-blocking enqueue.
- Every transport invocation is argv-only with a 3-second timeout.
- Stderr is the guaranteed fallback; no alert may be lost because a desktop
  transport is absent.
- One notification per incident by default; escalation may add at most one.
- Cooldown is per resource; the hourly cap is per resource.
- State is in memory; restart starts armed.
- No daemon, pidfile, lock file, state directory, service unit, or autostart.
- No ANSI escapes in notification bodies or status lines.
- No command lines in payloads.
- `--json` is an observation mode and never sends.

## 3. File & Interface Contracts

### `internal/watch/state.go`

```go
type Config struct {
    ETAThreshold      time.Duration
    PressureThreshold float64
    Sustain           time.Duration
    Cooldown          time.Duration
    MaxPerHour        int
    RearmETA          time.Duration
    RearmPressure     float64
    Resource          string // "both" | "swap" | "tmpfs"
    NotifyRecovery    bool
    Interval          time.Duration
    Now               func() time.Time
}

type Decision struct {
    Emit       bool
    Kind       string // "alert" | "recovered" | "suppressed"
    Reason     string // for suppressed
    Resource   string
    Severity   string // "warning" | "critical"
    Title      string
    Body       string
    Event      Event
}

func Evaluate(cfg Config, prior map[string]*State, stats live.WindowStats) []Decision
```

- `Evaluate` is pure: it does not read the clock; `cfg.Now` supplies time.
- `prior` is mutated only by the caller after a decision is accepted, or the
  function returns a new state map. Choose one and document it; the tests assume
  determinism.
- Decisions are returned in a stable order: swap before tmpfs.
- At most one alert per resource per evaluation.

### `internal/watch/render.go`

- `FormatTitle(resource, severity, eta, pct) string`
- `FormatBody(stats, top, reclaimable, age) string`
- `FormatStatus(stats, top) string`
- All formatting is plain text; names truncated to 40 cells; body at most three
  lines and 512 bytes.

### `internal/notify/notify.go`

```go
type Config struct {
    Mode    string // "auto" | "desktop" | "osc" | "stderr" | "command"
    Command []string
    Timeout time.Duration
    Stderr  io.Writer
    LookPath func(string) (string, error)
    Getenv   func(string) string
    Run      func(ctx context.Context, argv []string) error
    OpenTTY  func() (io.WriteCloser, error)
}

type Transport struct { Name string; Send func(title, body string) error }

func Select(cfg Config) (Transport, error)
func Sender(ctx context.Context, in <-chan Message, t Transport) // one goroutine
```

- `LookPath`, `Getenv`, `Run`, and `OpenTTY` are injectable so tests never send
  real notifications.
- `Select` probes in the FR-8 order and returns the first available transport.
- `Sender` reads messages until the channel closes, sends with the configured
  timeout, falls back down the chain on error, and records the first failure
  per transport.
- `--test-notify` calls `Select` and `Send` once with a fixed payload.

### `internal/app/app.go`

- Add `watch` to the bare-word subcommand switch before `term.InitTerminal`.
- Validate FR-2 before starting the sampler.
- `--json` disables the sender and writes events to stdout.
- `--test-notify` exits before the sampler starts.

## 4. External Contracts & Failure Modes

| Condition | Detection | Expected behavior |
| :--- | :--- | :--- |
| `notify-send` absent | `LookPath` | Skip to `gdbus` |
| No session bus | `DBUS_SESSION_BUS_ADDRESS` empty | Skip `gdbus`/`dbus-send` |
| No display | `DISPLAY` and `WAYLAND_DISPLAY` empty | Skip `kdialog` |
| No `/dev/tty` | open error | Skip OSC |
| All desktop transports fail | send errors | Use stderr, one warning |
| Transport hangs | context deadline | Kill at 3s, fall back |
| Sender channel full | non-blocking send | Drop, increment counter |
| ETA null and pressure below threshold | evaluation | No alert |
| ETA null and pressure above threshold and rising | evaluation | Pressure alert |
| tmpfs sample stale | age > 2× interval | Suppress tmpfs, one line |
| Cooldown active | last alert timestamp | `suppressed: cooldown` |
| Hourly cap reached | rolling hour count | `suppressed: hourly_cap` |
| Same dedup key | key comparison | `suppressed: dedup` |
| Signal | `NotifyContext` | Drain sender, emit `stopped`, exit 0 |
| `--json` | mode | No transport invocation |
| `--notify-cmd` metacharacters | argv construction | Passed literally, no shell |

## 5. Verification Matrix

| Test | Command | Expected |
| :--- | :--- | :--- |
| State machine | `go test -race ./internal/watch` | Pass |
| Transport chain | `go test -race ./internal/notify` | Pass |
| Full race | `go test -race ./...` | Pass |
| Idle fixture | `go test ./internal/watch -run TestIdle` | Zero alerts |
| Ramp fixture | `go test ./internal/watch -run TestRamp` | One alert, `gopls` |
| Spike fixture | `go test ./internal/watch -run TestSpike` | Pressure fallback only |
| Cooldown | `go test ./internal/watch -run TestCooldown` | Suppressed then alerts |
| Cap | `go test ./internal/watch -run TestHourlyCap` | Fifth suppressed |
| Escalation | `go test ./internal/watch -run TestEscalation` | Exactly one extra |
| Test notify | `go run ./cmd/swapaudit watch --test-notify` | `ok` on this box |
| Forced failure | injected failing transports | stderr fallback |
| Injection | `--notify-cmd` with `; rm -rf /` | Literal argv, no shell |
| JSON events | `watch --json --duration=…` | Valid NDJSON |
| Gate | `./scripts/check.sh --full` | Pass |

## 6. Open Questions / Deferred Decisions

- Whether the remedy line should mention `--interactive` after the picker goal
  lands. v1 says `run 'swapaudit' to review`.
- Whether to add a tmux transport (`tmux display-message`) when `$TMUX` is set.
  Deferred; it is a seventh transport with different semantics.
- Whether thresholds should differ for swap and tmpfs. Deferred; one threshold
  in v1.
- Whether the hourly cap should be global. Per-resource is safer with
  `--resource=both`; document the effective maximum.
- Whether recovery notifications should be on by default. Off in v1.
- Whether to persist the last-alert time across restarts. Out of scope; a
  foreground process restarting is a new observation session.

## 7. References

- `goals/live-sampler-rates/goal.md` — sampler, estimator gates, fixtures.
- `goals/json-snapshot-output/goal.md` — shared system and item types.
- `goals/one-shot-action-picker/goal.md` — remediation surface.
- `internal/app/app.go` — subcommand dispatch.
- `internal/term/term.go` — formatting helpers; no ANSI in watch output.
- `notify-send(1)`, `gdbus(1)`, `dbus-send(1)`, `kdialog(1)`,
  `org.freedesktop.Notifications`, OSC 777.
