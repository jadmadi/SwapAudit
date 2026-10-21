# Foreground watch and notifications

`swapaudit watch` consumes the `sample` engine and pushes one notification per
incident when swap or tmpfs is projected to run out. It is a foreground process:
no daemon, pidfile, lock file, service unit, state directory, or autostart is
created.

```bash
swapaudit watch
swapaudit watch --json --duration=30s        # events only, sends nothing
swapaudit watch --test-notify                # probe the transport chain
```

The problem is trust, not delivery. A notifier that fires on one noisy tick,
repeats every second, or silently fails is worse than no notifier. The trigger
therefore uses sustain, hysteresis, cooldown, an hourly cap, deduplication, and
an absolute-pressure fallback when the ETA is null.

## Flags

| Flag | Default | Meaning |
| :--- | :--- | :--- |
| `--eta` | `15m` | Alert when the ETA is at or below this. |
| `--pressure` | `90` | Fallback pressure threshold, percent. |
| `--sustain` | `10s` | Continuous danger required before alerting. |
| `--cooldown` | `10m` | Minimum time between incidents per resource. |
| `--max-per-hour` | `4` | Maximum alerts per resource in a rolling hour. |
| `--rearm-eta` | `20m` | ETA required before a new incident can arm. |
| `--rearm-pressure` | `85` | Pressure required before a new incident can arm. |
| `--resource` | `both` | `both`, `swap`, or `tmpfs`. |
| `--notify` | `auto` | `auto`, `desktop`, `osc`, `stderr`, or `command`. |
| `--notify-cmd` | - | One command argv element; repeat for each argument. |
| `--notify-recovery` | off | Emit one recovery event per incident. |
| `--test-notify` | off | Send one probe and exit; does not sample. |
| `--status-every` | `10s` | Status line cadence to stderr; `0` disables. |
| `--quiet` | off | Suppress the status line but keep notifications. |
| `--json` | off | Emit event NDJSON to stdout and send nothing. |
| `--interval` | `1s` | Fast-lane cadence. |
| `--window` | `60s` | Estimator window. |
| `--tmp-interval` | `30s` | Slow-lane cadence. |
| `--duration` | `0` | Stop after this duration; `0` means until a signal. |

Invalid combinations exit `2` before sampling. `--test-notify` exits `0` if any
transport in the chain succeeds and `1` otherwise.

## Danger evaluation

For each resource:

```text
eta_danger      = eta != null && eta <= --eta
pressure_danger = pct >= --pressure && rate > 0
danger          = eta_danger || pressure_danger
critical        = (eta != null && eta <= --eta/3) || pct >= 95
clear           = eta == null or eta > --rearm-eta, and pct < --rearm-pressure
```

The pressure fallback is mandatory: the estimator returns `null` exactly during
noisy bursts, which are the dangerous case. Tmpfs is suppressed when the slow
lane is older than `2 * --tmp-interval`; one `stale_tmpfs` event is emitted per
stale episode.

## State machine

```text
             danger                sustain + 2 evaluations
  armed ──────────────▶ pending ──────────────────────────▶ alerted
    ▲                      │                                   │
    │                      │ not danger                        │ clear for sustain
    │                      ▼                                   ▼
    └──── clear for sustain ─── recovering ◀───────────────────┘
                                  │
                                  └── danger ──▶ pending
```

- `pending` resets silently if danger clears before the sustain window.
- `alerted` emits exactly one notification and suppresses repeats.
- `warning` may escalate to `critical` once per incident; escalation bypasses
  the cooldown but not the hourly cap.
- After `--cooldown` and a new incident, alerts resume.
- The fifth alert in an hour (with `--max-per-hour=4`) emits one
  `suppressed: hourly_cap` event and stays silent until the hour rolls.
- Deduplication uses `(resource, top entity, severity)`; the per-resource
  cooldown subsumes it when a cooldown is configured.
- State is in memory only. Restarting `watch` starts armed.

## Top grower

The top grower is the largest positive `delta_bytes` in the window, excluding
exited entities. A process renders as `name (PID N)`; a cache renders as its
truncated path. If no positive mover exists, the body says
`top grower: unaccounted (shared/kernel)`. Per-minute rates are derived from the
window, not from a single tick. Command lines are never included.

## Payloads

Swap alert:

```text
title: swapaudit: swap full in 5m 12s
body:
13.9 GiB/17.1 GiB (81.3%) · +205.0 MB/min · sustained 42s
top grower: gopls (PID 1420081) +96.0 MB/min, now 1.9 GB
run 'swapaudit' to review reclaimable tiers
```

Tmpfs alert:

```text
title: swapaudit: tmpfs full in 7m 47s
body:
14.9 GiB/15.6 GiB (95.5%) · +92.0 MB/min · sustained 30s
top grower: /tmp/go-build998 +48.0 MB/min, now 2.4 GB
577.6 MB reclaimable (sampled 8s ago) · run 'swapaudit' to review
```

If the danger is pressure-only, the title is
`swapaudit: swap 93.0% and rising`. The reclaimable line appears only for tmpfs,
where clearing caches is causal; a swap alert never implies that tmpfs bytes
relieve swap. Bodies are plain text, at most three lines and 512 bytes, with
names truncated to 40 cells.

## Transport chain

`--notify=auto` probes once at startup and selects the first available
transport, with the rest as fallbacks:

| Order | Transport | Availability | Invocation |
| :---: | :--- | :--- | :--- |
| 1 | `notify-send` | in PATH | `notify-send --app-name=swapaudit --urgency=... --expire-time=10000 title body` |
| 2 | `gdbus` | in PATH and `DBUS_SESSION_BUS_ADDRESS` set | `gdbus call --session --dest org.freedesktop.Notifications ... Notify ...` |
| 3 | `dbus-send` | in PATH and a session bus | `dbus-send --session --type=method_call ...` |
| 4 | `kdialog` | in PATH and `DISPLAY`/`WAYLAND_DISPLAY` set | `kdialog --title ... --passivepopup ...` |
| 5 | `osc` | `/dev/tty` opens | OSC 777 to `/dev/tty` |
| 6 | `stderr` | always | `NOTIFY <title> \| <body>` |

All invocations are `exec.CommandContext` with argv only, a 3-second timeout,
and no shell. `--notify-cmd` is repeatable and each occurrence is one argv
element, so a value such as `; rm -rf /` is passed literally. The sender runs in
one goroutine with a four-message buffer; a full buffer drops the event and the
status line reports `dropped N`. A failed transport falls back to the next and
finally stderr; the first failure per transport is logged once.

`--test-notify` selects the transport and sends one probe. It ignores thresholds
and does not start the sampler.

## Fatigue tuning

- Raise `--sustain` to require a longer danger window.
- Raise `--cooldown` or lower `--max-per-hour` to mute repeats.
- Raise `--eta` to alert earlier; lower it to alert only when the end is near.
- Raise `--rearm-eta`/lower `--rearm-pressure` to require a deeper recovery.
- Use `--quiet` or `--status-every=0` to keep notifications but silence status.
- Use `--json` to test the state machine and consume events yourself.

## Background operation

The only supported background mode is the same foreground binary wrapped by
`systemd-run`:

```bash
systemd-run --user --unit=swapaudit-watch --collect \
  swapaudit watch --eta=15m --pressure=90
```

Inspect logs with `journalctl --user -u swapaudit-watch`. No unit file, pidfile,
or state directory is installed by swapaudit.

## Privacy

Notification payloads and JSON events contain process names, PIDs, cache paths,
and sizes. They never contain command lines, arguments, or environment values.
Desktop notifications can appear on a lock screen. Users who need stronger
redaction should consume `--json` with their own filter; `--redact` is not part
of v1.

## JSON events

`--json` writes one object per state transition and sends nothing:

```json
{"type":"alert","resource":"swap","severity":"warning","sampled_at":"2026-10-21T10:00:00+03:00",
 "eta_seconds":312,"pressure_pct":81.3,"rate_bytes_per_sec":3413.3,
 "top":{"kind":"process","pid":1420081,"start_ticks":123456,"name":"gopls",
        "delta_bytes":96636764,"delta_bytes_per_min":96636764,"current_bytes":2040109465},
 "reclaimable_bytes":0,"reclaimable_age_seconds":0,"transport":null,
 "message":"swapaudit: swap full in 5m 12s"}
```

`type` is `alert`, `recovered`, `suppressed`, or `stopped`. `suppressed` carries
`reason`: `cooldown`, `hourly_cap`, `stale_tmpfs`, or `dedup`. `eta_seconds` is
null for pressure-only alerts and `top` is null when no positive mover exists.
On shutdown, `{"type":"stopped","reason":"signal"|"duration","sampled_at":"..."}`
is emitted and the exit code is `0`.
