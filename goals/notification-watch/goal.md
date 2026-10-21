# Goal: Add a Foreground Watch Mode with Reliable Threshold Notifications

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit (personas 1, 5, 11, 32)
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: Pending

## Goal Description

A live screen is pull-based: it only helps while someone is looking at it. Swap
and tmpfs pressure build while the user is in an editor, watching a build, or
away from the terminal — exactly when the screen is not visible. This goal adds
`swapaudit watch`, a foreground process that consumes the `live-sampler-rates`
engine and pushes one notification per incident when a resource is projected to
run out.

The hard problem is not delivery; it is trust. A notifier that fires on a single
noisy tick, repeats every second, or silently fails to deliver is worse than no
notifier, because it teaches the user to mute it. This goal therefore specifies a
trigger state machine (sustain, hysteresis, cooldown, hourly cap, dedup,
escalation), an ETA-null fallback based on absolute pressure plus a rising
slope, a top-grower attribution that excludes exited processes, a probed
transport chain with a guaranteed stderr fallback, and a `--test-notify`
self-check.

Notification delivery on Linux has no single reliable transport: this
workstation has no `notify-send`, but it does have `gdbus`, `dbus-send`,
`kdialog`, a live session bus, KDE/Wayland, and `/dev/tty`. A hardcoded
`notify-send` would silently no-op here, so the transport must be probed and
verified.

### Desired outcome

- `swapaudit watch` runs in the foreground at the sampler cadence and prints a
  concise status line every `--status-every` (default 10s).
- When a resource enters danger for `--sustain`, exactly one notification is
  delivered, naming the resource, the ETA or pressure, the rate, the top grower
  with PID or path, and the reclaimable remedy.
- Idle produces zero notifications. A monotone ramp produces exactly one. A
  recovering or noisy series produces none unless the absolute-pressure
  fallback fires.
- No duplicate notification within `--cooldown`; no more than
  `--max-per-hour` per resource; one escalation from warning to critical is
  allowed per incident.
- `--test-notify` reports which transport was selected and whether it succeeded.
- If every desktop transport fails, the event still reaches stderr.
- No daemon, pidfile, service unit, state directory, or autostart is created.
- `--json` emits the same events as JSON lines and sends nothing, making the
  state machine testable and CI-friendly.

### Non-goals

- No daemon or system service. Background operation is `systemd-run --user`
  around the same foreground binary, documented, not implemented.
- No TUI, alternate screen, raw mode, or key handling.
- No automatic cleanup, process signalling, or file deletion.
- No webhook, email, Slack, Pushover, or network delivery. This is local-only.
- No D-Bus client library; use external helpers or OSC, consistent with the
  zero-dependency floor.
- No notification history database and no cross-restart dedup.
- No changes to the sampler's estimator or the JSON snapshot schema.
- No alerting on RAM exhaustion; `MemAvailable` is reclaimable and not a
  monotone reservoir.

## Scope & Boundaries

### In scope

- `internal/watch/state.go` (new): danger evaluation and the per-resource state
  machine.
- `internal/watch/render.go` (new): notification and status-line formatting.
- `internal/watch/state_test.go`, `render_test.go` (new).
- `internal/notify/notify.go` (new): transport probing, argv-only execution,
  sender goroutine.
- `internal/notify/notify_test.go` (new).
- `internal/app/app.go`: `watch` subcommand, flags, validation, dispatch.
- `internal/app/app_test.go`: flag and validation tests.
- `internal/app/display.go`: usage text.
- `docs/watch.md` (new): triggers, transports, fatigue tuning, systemd-run
  recipe, privacy.
- `README.md`, `CHANGELOG.md`.
- `goals/notification-watch/evidence.md`.

### Out of scope

- `internal/live` estimator changes. This goal consumes its output.
- `internal/picker` changes. The notification may suggest running `swapaudit`,
  not a picker flag that may not exist yet.
- Any new Go module dependency.
- Any global configuration file. Flags and environment variables only.

## Requirements

### Functional requirements

**FR-1 — Command and flags.**

```text
swapaudit watch [--eta=15m] [--pressure=90] [--sustain=10s] [--cooldown=10m]
                [--max-per-hour=4] [--rearm-eta=20m] [--rearm-pressure=85]
                [--resource=both|swap|tmpfs]
                [--notify=auto|desktop|osc|stderr|command]
                [--notify-cmd=<argv>] [--test-notify]
                [--status-every=10s] [--quiet] [--json]
                [--interval=1s] [--window=60s] [--tmp-interval=30s]
```

- `watch` implies `--no-update-check`.
- `--interval`, `--window`, and `--tmp-interval` are validated by the sampler
  rules and forwarded unchanged.
- `--json` emits events only, suppresses the status line, and disables all
  transport sends. It is the test and CI mode.
- `--quiet` suppresses the status line but keeps notifications.
- `--status-every=0` disables the status line (equivalent to `--quiet`).
- `--test-notify` sends one probe notification through the selected transport,
  prints `transport: <name> ok|failed`, and exits 0 if any transport succeeded,
  1 otherwise. It ignores thresholds and does not start the sampler.
- `--notify-cmd` requires `--notify=command`; otherwise exit 2.
- `--resource=both` (default) evaluates swap and tmpfs independently; each may
  alert once per incident.

**FR-2 — Validation.**

| Condition | Behavior |
| :--- | :--- |
| `--eta <= 0` or `--eta > 24h` | Exit 2 |
| `--pressure` outside `1..100` | Exit 2 |
| `--rearm-eta <= --eta` | Exit 2 with `error: --rearm-eta must be greater than --eta` |
| `--rearm-pressure >= --pressure` | Exit 2 with `error: --rearm-pressure must be less than --pressure` |
| `--sustain < --interval` | Exit 2 with `error: --sustain must be at least one interval` |
| `--cooldown < 0` or `--max-per-hour < 1` | Exit 2 |
| `--notify-cmd` without `--notify=command` | Exit 2 |
| `--notify=command` without `--notify-cmd` | Exit 2 |
| `--json` with `--notify=command` | Exit 2 (JSON never sends) |
| Unknown transport | Exit 2 with the five allowed values |

**FR-3 — Danger evaluation, per resource.**

For swap, use `WindowStats.Swap`; for tmpfs, use `WindowStats.Tmpfs`. Let
`eta = stats.ETASeconds`, `pct = used/total*100`, and
`rising = stats.RateBytesPerSec > 0`.

```text
eta_danger      = eta != nil && eta <= --eta
pressure_danger = pct >= --pressure && rising
danger          = eta_danger || pressure_danger
critical        = (eta != nil && eta <= --eta/3) || pct >= 95
```

- The pressure fallback is mandatory: the estimator returns `null` exactly
  during noisy bursts, which are the dangerous case.
- For tmpfs, if `WindowStats.TmpfsSampledAgeSec > 2 * --tmp-interval`, suppress
  tmpfs evaluation entirely and emit at most one stderr line per stale episode:
  `tmpfs sample stale (Ns); suppression active`.
- For swap, no staleness suppression is needed; the fast lane is the current
  tick.

**FR-4 — Sustain.**

- Danger must hold continuously for `--sustain`, measured on the watch clock,
  and must be observed in at least two evaluations.
- A single noisy evaluation never alerts.
- If danger clears before the sustain window elapses, reset silently.
- The status line reports `sustained <duration>` once the timer starts.

**FR-5 — Per-resource state machine.**

States: `armed`, `pending`, `alerted`, `recovering`.

```text
armed --danger--> pending
pending --danger for sustain--> alerted (emit one notification)
pending --not danger--> armed (reset)
alerted --danger persists--> alerted (suppress; no repeat by default)
alerted --not danger for sustain--> recovering
recovering --not danger for sustain--> armed (cooldown still applies to a new alert)
recovering --danger again--> pending
```

Rules:

- Emit exactly one notification on entering `alerted`.
- While `alerted`, suppress unless the severity escalates from `warning` to
  `critical`; one escalation per incident is allowed and bypasses the cooldown.
- No repeats for a new incident until `--cooldown` has elapsed since the last
  notification for that resource.
- At most `--max-per-hour` notifications per resource in a rolling hour. When
  the cap is reached, emit one `suppressed` event with `reason: "hourly_cap"`
  and remain silent until the hour rolls.
- Dedup key is `(resource, top entity key, severity band)`. An identical key
  within cooldown is suppressed.
- Recovery itself is silent by default. `--notify-recovery` (off) emits one
  `recovered` event/notification when a resource returns to `armed`.
- State is in memory only; restarting `watch` starts armed.

**FR-6 — Top grower.**

- For swap, choose the largest positive `delta_bytes` from
  `WindowStats.Movers` where `exited == false`.
- For tmpfs, choose the largest positive `delta_bytes` from
  `WindowStats.TmpfsMovers`.
- Exclude negative and zero deltas. If no positive mover exists, emit
  `top: null` and describe the cause as `unaccounted (shared/kernel)`.
- Compute `delta_bytes_per_min = delta_bytes * 60 / window_seconds`; do not
  present a per-minute rate derived from a single tick.
- Render process tops as `<name> (PID <pid>)`; render cache tops as the
  truncated path.
- Never include command lines, environment variables, or arguments.

**FR-7 — Notification payload.** Title and body are plain text, no ANSI. A
numeric reclaimable line is emitted **only for tmpfs alerts**, where clearing
caches is causal; a swap alert must not imply that tmpfs bytes relieve swap.

Swap alert:

```text
title: swapaudit: swap full in 5m 12s
body:
13.9/17.1 GiB (81.3%) · +205 MB/min · sustained 42s
top grower: gopls (PID 1420081) +96 MB/min, now 1.9 GiB
run 'swapaudit' to review reclaimable tiers
```

Tmpfs alert:

```text
title: swapaudit: tmpfs full in 7m 47s
body:
14.9/15.6 GiB (95.5%) · +92 MB/min · sustained 30s
top grower: /tmp/go-build998 +48 MB/min, now 2.4 GiB
577.7 MB reclaimable (sampled 8s ago) · run 'swapaudit' to review
```

- If the danger is pressure-only (ETA null), the title is
  `swapaudit: swap 93.0% and rising` or `swapaudit: tmpfs 96.0% and rising`.
- The numeric reclaimable line uses `tmpfs_cleanable_bytes` and appends
  `(sampled Ns ago)` when the slow-lane age is over half the tmp interval.
- The swap remedy line never includes a numeric total in v1. A process-swap
  cleanable total would require re-running triage; it is deferred.
- If the top is null, the second line is
  `top grower: unaccounted (shared/kernel)`.
- Body lines are separated by `\n`; transports receive the title and body as two
  separate arguments.
- The payload must be under 512 bytes; truncate names to 40 visible cells and
  the body to three lines.

**FR-8 — Transport chain.** `--notify=auto` probes once at startup and selects
the first available transport:

| Order | Transport | Availability | Invocation |
| :---: | :--- | :--- | :--- |
| 1 | `notify-send` | `exec.LookPath("notify-send")` | `notify-send --app-name=swapaudit --urgency=<normal\|critical> --expire-time=10000 <title> <body>` |
| 2 | `gdbus` | `gdbus` in PATH and `DBUS_SESSION_BUS_ADDRESS` set | `gdbus call --session --dest org.freedesktop.Notifications --object-path /org/freedesktop/Notifications --method org.freedesktop.Notifications.Notify swapaudit 0 "" <title> <body> [] {} 10000` |
| 3 | `dbus-send` | `dbus-send` in PATH and `DBUS_SESSION_BUS_ADDRESS` set | `dbus-send --session --type=method_call --dest=org.freedesktop.Notifications /org/freedesktop/Notifications org.freedesktop.Notifications.Notify string:swapaudit uint32:0 string:"" string:<title> string:<body> array:string:"" dict:string:string:"","" int32:10000` |
| 4 | `kdialog` | `kdialog` in PATH and `DISPLAY` or `WAYLAND_DISPLAY` set | `kdialog --title <title> --passivepopup <body> 10` |
| 5 | `osc` | `/dev/tty` opens | write `ESC ] 777 ; notify ; <title> ; <body> BEL` to `/dev/tty` |
| 6 | `stderr` | always | one line: `NOTIFY <title> \| <body with spaces collapsed>` |

- `--notify=desktop` restricts the chain to 1-4; `--notify=osc` to 5+6;
  `--notify=stderr` to 6; `--notify=command` uses only `--notify-cmd`.
- `--notify=auto` always ends at stderr. A restricted mode
  (`desktop`, `osc`, `command`) with no available transport exits 2 before
  sampling with `error: no <mode> notification transport available`.
- The probe uses `exec.LookPath` and environment checks only; it must not send a
  test notification unless `--test-notify` is passed.
- All executions use `exec.CommandContext` with a 3-second timeout, argv only,
  and no shell. `--notify-cmd` receives `<title>` and `<body>` as two arguments.
- At send time, a failed transport falls back to the next available one and
  finally stderr. The first failure per transport is logged once to stderr:
  `notify: <transport> failed: <error>; falling back`.
- The selected transport is remembered; a successful fallback becomes the new
  primary for the process lifetime.
- `--json` never invokes a transport.

**FR-9 — Sender goroutine and backpressure.**

- The watch loop never blocks on a subprocess. Notifications are handed to one
  sender goroutine through a buffered channel of capacity 4.
- A non-blocking send drops the event and increments `dropped_notifications`
  when the channel is full; the status line reports the count.
- The sender goroutine is the only place that executes a transport.
- On shutdown, the channel is closed, the sender drains, and `watch` waits for
  it before returning.
- This is the only additional goroutine beyond the sampler's slow lane.

**FR-10 — Status output.**

- Default: one line to stderr every `--status-every` (10s), not every tick:

```text
23:41:07 swap 81.3% +205 MB/min eta 5m12s | tmpfs 95.5% +92 MB/min eta 7m47s | top gopls +96 MB/min
```

- Omit a resource when its series is `insufficient` and its pressure is below
  the threshold.
- Show `eta --` when null; never print a negative or zero ETA.
- `--quiet` or `--status-every=0` disables it.
- The status line goes to stderr so `--json` can own stdout.
- No carriage returns, cursor movement, or ANSI escape sequences.

**FR-11 — JSON events.** `--json` writes one object per line to stdout for every
state transition that would produce a notification or suppression:

```json
{"type":"alert","resource":"swap","severity":"warning","sampled_at":"2026-10-21T10:00:00+03:00",
 "eta_seconds":312,"pressure_pct":81.3,"rate_bytes_per_sec":3413.3,
 "top":{"kind":"process","pid":1420081,"start_ticks":123456,"name":"gopls",
        "delta_bytes":96636764,"delta_bytes_per_min":96636764,"current_bytes":2040109465},
 "reclaimable_bytes":605617408,"reclaimable_age_seconds":8,
 "transport":null,"message":"swap full in 5m 12s"}
```

- `type` is `alert`, `recovered`, `suppressed`, or `stopped`.
- `suppressed` includes `reason`: `cooldown`, `hourly_cap`, `stale_tmpfs`, or
  `dedup`.
- `transport` is `null` in JSON mode.
- `eta_seconds` is `null` when the pressure fallback fired.
- `top` is `null` when no positive mover exists.
- Arrays and objects are never `null` unless the schema says so; `top` is the
  only nullable object.
- On signal, emit `{"type":"stopped","reason":"signal","sampled_at":"..."}` and
  exit 0.

**FR-12 — Lifecycle.** Use the sampler's context, ticker, and cancellation. Watch
adds only the sender goroutine. `signal.NotifyContext` covers SIGINT/SIGTERM.
No pidfile, no lock file, no state directory, no fork, no daemonization.

**FR-13 — Privacy.** Notification bodies and JSON events contain process names,
PIDs, cache paths, and sizes. They never contain command lines or arguments.
Document that desktop notifications may appear on a lock screen. `--redact` is
not in v1; users who need it should use `--json` with their own consumer.

**FR-14 — Documentation.** `docs/watch.md` covers the trigger rules with a state
diagram, the transport chain, `--test-notify`, fatigue tuning, the systemd-run
recipe, and the privacy note. README and `PrintUsage` link to it.

**FR-15 — Exit codes.** `0` normal/signal; `2` usage/validation; `1` only when
`--test-notify` finds no working transport.

### Non-functional requirements

- **NFR-1 — Dependency floor.** Standard library only; Linux build tag; Go 1.23;
  no D-Bus, notification, or terminal library.
- **NFR-2 — Size ceiling.** `internal/watch` + `internal/notify` production code
  at most 400 lines; tests at most 450; no file approaches 1,000 lines.
- **NFR-3 — Determinism.** The state machine is a pure function of
  `(event, now, config, prior state)`. Tests inject a clock and a fake notifier
  and assert exact transitions and payloads.
- **NFR-4 — Non-blocking.** No subprocess execution on the sampling path; the
  sender goroutine owns it. Sampling continues while the sender works, and a
  hung transport is killed at 3 seconds and falls back.
- **NFR-5 — No leaks.** The sender exits on context cancellation; a
  goroutine-count test asserts no growth.
- **NFR-6 — Idempotence.** Re-running `--test-notify` is safe and side-effect
  free apart from the notification itself.
- **NFR-7 — Portability.** Linux-only, consistent with the rest of the tool. On
  a machine with no desktop bus and no TTY, stderr still works.
- **NFR-8 — Silence on idle.** A 10-minute idle simulation must produce zero
  notifications and zero JSON alert events.

## Acceptance Criteria

1. `go build ./...`, `go vet ./...`, `go test -race ./...` pass.
2. `./scripts/check.sh --full` passes.
3. An idle trace produces zero alerts over a simulated 10 minutes.
4. A ramp trace produces exactly one alert, names `gopls`, and reports an ETA
   within 10% of the fixture's analytic value.
5. A recovering trace produces zero alerts.
6. A spike that gates the ETA to `null` produces no alert below 90% pressure,
   and one alert when the pressure fallback is satisfied.
7. A one-tick danger blip does not alert; a sustained 10-second danger does.
8. A second incident within `--cooldown` is suppressed with
   `reason: "cooldown"`; after the cooldown it alerts.
9. The fifth alert in an hour is suppressed with `reason: "hourly_cap"` when
   `--max-per-hour=4`.
10. Escalation from warning to critical produces exactly one additional alert
    per incident.
11. `--test-notify` on this machine, where `notify-send` is absent, selects
    `gdbus` or `kdialog`, and reports `ok`.
12. When every transport is forced to fail, the event still appears on stderr
    with exactly one fallback warning.
13. `--notify=command` with an argument containing `; rm -rf /` passes it
    literally as an argv element; no shell is invoked.
14. The notification body contains no ANSI escapes, no command lines, and is
    under 512 bytes.
15. `--json` emits parseable event lines for alert/suppressed/stopped and invokes
    no transport.
16. `--quiet` and `--status-every=0` suppress the status line but not alerts.
17. No pidfile, lock file, state directory, or service unit is created.
18. A 60-second run uses no more CPU than the underlying sampler plus a
    negligible sender cost.
19. `internal/watch` and `internal/notify` stay within the NFR-2 ceiling.
20. `docs/watch.md`, README, and `PrintUsage` document the feature, transport
    chain, fatigue controls, and privacy note.

## Evidence Requirements

`goals/notification-watch/evidence.md` with a standalone `Verdict: PASS` and:

- a state-machine transition table with timestamps for idle, ramp, spike,
  recovery, cooldown, cap, and escalation;
- the fixture-by-fixture expected-versus-actual event sequence;
- `--test-notify` output on this workstation, including the selected transport;
- a forced-failure transcript showing the stderr fallback and the single
  warning;
- an argv-injection test showing `; rm -rf /` is passed literally and no shell
  runs;
- a `--json` capture with valid NDJSON and no transport invocation;
- a 60-second CPU measurement for `watch`;
- a goroutine-count test result;
- `git diff --stat` and line-count checks;
- confirmation that no daemon artifacts and no ANSI escapes were produced.

## Dependencies & Execution Order

- **Mode**: Dependent (requires prerequisite goals)
- **Depends On**: live-sampler-rates
- **Sequence**: 4
- **Shape**: ship
- **Tier**: roadmap

## Files to Touch

- internal/watch/state.go
- internal/watch/render.go
- internal/watch/state_test.go
- internal/watch/render_test.go
- internal/notify/notify.go
- internal/notify/notify_test.go
- internal/app/app.go
- internal/app/app_test.go
- internal/app/display.go
- docs/watch.md
- README.md
- CHANGELOG.md
- goals/notification-watch/evidence.md

## Persona & Audit Plan

- **Persona 1 — agent-readiness-architect:** event schema, `--json` framing,
  transport discoverability, and whether an agent can consume the events without
  scraping prose.
- **Persona 5 — senior-fullstack-engineer:** state-machine correctness,
  hysteresis and cooldown arithmetic, sender-goroutine lifecycle, argv safety,
  and fallback ordering.
- **Persona 11 — ux-information-architect:** notification wording, signal versus
  noise, status-line density, and the cost of a false alarm.
- **Persona 32 — mystery-auditor:** attempt to trigger a storm with alternating
  ticks, bypass the cooldown via escalation, block the sampler with a hung
  transport, inject shell metacharacters into `--notify-cmd`, leak a goroutine on
  SIGTERM, and force a silent no-op when no transport exists.
- **Commands:** `sila persona get 1`, `sila persona get 5`,
  `sila persona get 11`, `sila persona get 32`, `sila hire assess 1,5,11,32`.
- **Artifacts:** `goals/notification-watch/evidence.md` (or a sibling
  `persona-review.md`).

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation | Test |
| :--- | :---: | :---: | :--- | :--- |
| False alarms destroy trust | High | High | Sustain, hysteresis, cooldown, cap, ETA gating, pressure fallback | Fixture matrix |
| Notification storm from flapping | Medium | High | Per-resource state machine, dedup key, one escalation per incident | Alternating-tick test |
| Silent transport no-op | High on this box | High | Probe chain, stderr fallback, `--test-notify` | Transport test |
| Hung transport stalls sampling | Medium | High | Sender goroutine, buffered channel, 3s context timeout | Slow-transport test |
| Shell injection via `--notify-cmd` | Low | High | `exec.CommandContext` argv only; never `sh -c` | Metacharacter test |
| ETA null during dangerous bursts | High | High | Absolute-pressure fallback with rising slope | Spike fixture |
| Duplicate alerts within an incident | Medium | Medium | `alerted` state suppresses; dedup key | State-machine test |
| Goroutine leak on shutdown | Low | Medium | Close channel, drain, wait, leak test | Leak test |
| Privacy exposure on lock screen | Medium | Medium | Names/paths only, docs, `--json` alternative | Payload review |
| Scope creep into a daemon | Medium | High | Explicit non-goal; systemd-run recipe only | Artifact check |
| `dbus-send` signature incompatibility | Medium | Low | Chain fallback to `kdialog`/OSC/stderr | Transport test |

## Open Questions / Deferred Decisions

- Should the remedy line say `swapaudit --interactive` once the picker goal
  lands? v1 says `run 'swapaudit' to review`, which works today.
- Should `--notify-recovery` default on? The goal keeps it off to avoid doubling
  notifications; revisit after field use.
- Should there be a webhook transport for remote/CI machines? Explicitly out of
  scope; users can consume `--json`.
- Should `watch` integrate with tmux (`display-message`/status line) when
  `$TMUX` is set? Deferred; it would be a seventh transport with its own
  semantics.
- Should alert thresholds be per-resource (swap 15m, tmpfs 5m)? The goal uses
  one threshold for v1; a future flag could split them.
- Should the hourly cap be global rather than per-resource? Per-resource is
  safer with `--resource=both`; document the effective maximum.

## References

- **Shared Understanding & Fact Sheet**:
  [`goals/notification-watch/facts.md`](facts.md)
- **Execution Plan**: [`goals/notification-watch/plan.md`](plan.md)
- `internal/live` — sampler, ring, estimator, and trace fixtures.
- `internal/app/app.go` — subcommand dispatch and flag validation.
- `internal/term/term.go` — width and formatting helpers; watch emits no ANSI.
- `goals/live-sampler-rates/goal.md` — prerequisite engine and gating rules.
- `goals/one-shot-action-picker/goal.md` — the remediation surface, not a
  dependency.
- `goals/json-snapshot-output/goal.md` — shared system/snapshot types.
- `notify-send(1)`, `gdbus(1)`, `dbus-send(1)`, `kdialog(1)`,
  `org.freedesktop.Notifications` D-Bus interface.

## Done Condition

1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the
   `implemented` transition.
2. Implementation is committed with `(goals/notification-watch/goal.md)` or
   `[goal: notification-watch]` attribution under a `feat:`, `fix:`,
   `refactor:`, `perf:`, or `chore:` prefix.
3. Goal is certified (`goals/notification-watch/certification.json` with
   `verified: true` or a signed `audit-receipt.json`) and any markdown evidence
   contains an explicit standalone `Verdict: PASS` declaration.
4. `sila goals --json` reports `status: "implemented"`,
   `verification_status: "verified"`, and `unverified_count: 0` for delivered
   goals, with a clean `git status`.
5. `go test -race ./...`, `./scripts/check.sh --full`, `sila facts verify`, and
   `sila gate` pass.
