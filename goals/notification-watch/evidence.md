# Evidence: Add a Foreground Watch Mode with Reliable Threshold Notifications

Date: 2026-10-21
Workstation: Linux, KDE/Wayland, session bus available, `notify-send` absent

Verdict: PASS

## 1. Build, vet, race suite, and full gate

```
$ go build ./...
$ go vet ./...
$ go test -race ./...
ok  github.com/jadmadi/swapaudit/cmd/swapaudit     1.804s
ok  github.com/jadmadi/swapaudit/internal/app      9.298s
ok  github.com/jadmadi/swapaudit/internal/cache    (cached)
ok  github.com/jadmadi/swapaudit/internal/cleaner  (cached)
ok  github.com/jadmadi/swapaudit/internal/live     1.393s
ok  github.com/jadmadi/swapaudit/internal/model    (cached)
ok  github.com/jadmadi/swapaudit/internal/notify   (cached)
ok  github.com/jadmadi/swapaudit/internal/picker   (cached)
ok  github.com/jadmadi/swapaudit/internal/proc     (cached)
ok  github.com/jadmadi/swapaudit/internal/snapshot (cached)
ok  github.com/jadmadi/swapaudit/internal/term     (cached)
ok  github.com/jadmadi/swapaudit/internal/triage   (cached)
ok  github.com/jadmadi/swapaudit/internal/updater  (cached)
ok  github.com/jadmadi/swapaudit/internal/version  (cached)
ok  github.com/jadmadi/swapaudit/internal/watch    1.086s

$ ./scripts/check.sh --full
All checks passed in 19s.
```

## 2. State-machine transitions

`go test ./internal/watch -v` covers the following (t0 is the fake clock base):

| Scenario | Transition sequence | Decisions |
| :--- | :--- | :--- |
| Idle (10 minutes) | 200 evaluations, always clear | none |
| Ramp | t0 danger → pending; t0+10s danger → alerted | one `alert`, warning, ETA 5m12s, top `gopls`, `sustained 10s` |
| Blip | t0 danger → pending; t0+5s clear → armed | none |
| Pressure fallback | t0 danger (93%, ETA null) → pending; t0+10s → alerted | one `alert`, title `swapaudit: swap 93.0% and rising`, ETA null, top null |
| Cooldown | alert at t0+10; clear at t0+20/30/40 → armed; danger t0+41 → pending; t0+51 → suppressed | `suppressed: cooldown`; after 10m and a new recovery, the next incident alerts |
| Hourly cap | four incidents with cooldown 0; fifth incident | `suppressed: hourly_cap`, emitted once |
| Escalation | warning at t0+10; critical at t0+20; critical at t0+30 | one critical alert; no repeat |
| Stale tmpfs | first evaluation with age 100s | one `suppressed: stale_tmpfs`; second evaluation silent; flag resets when fresh |

`TestIdleProducesNoAlerts`, `TestRampProducesOneAlert`,
`TestSustainBlipDoesNotAlert`, `TestPressureFallback`, `TestCooldown`,
`TestHourlyCap`, `TestEscalation`, `TestStaleTmpfsSuppressedOnce`, and
`TestResourceFilter` all pass.

## 3. Fixture replay through the state machine

`go test ./internal/watch -run TestFixtureSequence -v`:

| Fixture | Alerts | Details |
| :--- | :---: | :--- |
| `idle.ndjson` | 0 | flat, ETA null |
| `ramp.ndjson` | 1 | `2026-10-21T10:00:58+03:00`, ETA 3526s, top `gopls` |
| `spike.ndjson` | 0 | ETA null (noisy fit), pressure below 90% |
| `recovering.ndjson` | 0 | negative slope, ETA null |
| `noise.ndjson` | 0 | non-monotone, ETA null |
| `short.ndjson` | 0 | fewer than 5 samples |
| `pid-reuse.ndjson` | 0 | flat, old key retired in `exited_bytes` |
| `exit.ndjson` | 0 | flat, exit counted in `exited_bytes` |
| `stale-tmp.ndjson` | 0 | swap flat; tmpfs suppressed as stale |

The ramp alert ETA is asserted within 10% of `free_bytes / slope` in the same
test. The sampler's own `TestRampETAWithin10Percent` measured 3525s versus the
analytic 3525.0s.

## 4. Transport probe on this workstation

```
$ swapaudit watch --test-notify
transport: gdbus ok
exit=0
```

`notify-send` is absent; `gdbus` was selected first and delivered successfully.
The chain order is covered by `TestSelectAutoChain`
(`notify-send,gdbus,dbus-send,kdialog,osc,stderr`).

## 5. Forced failure and stderr fallback

`TestSenderWarnsOnceAndDrains` forces `notify-send` to fail and all desktop
transports to be unavailable:

```
notify: notify-send failed: boom; falling back
NOTIFY T1 | B
NOTIFY T2 | B
```

Exactly one warning per transport and two stderr deliveries, even though two
messages failed the primary.

## 6. argv-only injection safety

`TestCommandLiteralArgv` uses `--notify=command` with
`Command = ["/bin/echo", "; rm -rf /"]` and asserts the captured argv is exactly:

```
["/bin/echo", "; rm -rf /", "T", "B"]
```

`exec.CommandContext` receives argv directly; no `sh -c` exists anywhere in
`internal/notify`.

## 7. JSON events

`TestEventJSON` asserts the alert shape:

```
type=alert resource=swap severity=warning eta_seconds=312
top.name=gopls top.pid=1420081 transport=null
```

and the `suppressed` (`reason: cooldown`) and `recovered` shapes.
`TestRunWatchJSON` runs `watch --json --duration=300ms` and validates every
line; a calm interval emits the final:

```
{"reason":"duration","sampled_at":"2026-10-21T11:09:40+03:00","type":"stopped"}
```

No transport is selected or invoked in JSON mode. `TestRunWatchTestNotifyStderr`
proves `--test-notify --notify=stderr` prints `transport: stderr ok` and one
`NOTIFY` line.

## 8. Cost and lifecycle

```
$ bash -c 'TIMEFORMAT="watch60 real=%R user=%U sys=%S"; time
  swapaudit watch --quiet --duration=60s' 2>/tmp/watch60.time
watch60 real=60.017 user=0.447 sys=0.634
exit=0, stdout=0 bytes, stderr=0 bytes
```

CPU = (0.447 + 0.634) / 60.017 = **1.80% of one core**, versus 1.75% for the
sampler alone; the idle sender goroutine costs about 0.05%.

- `live.TestNoGoroutineLeak` asserts the sampler returns to baseline after a run.
- `notify.TestSenderWarnsOnceAndDrains` closes the channel and waits for the
  sender to drain.
- The sender owns all subprocess execution; the sampling path only performs a
  non-blocking channel send and increments `dropped` when the buffer is full.
- Each transport uses a 3-second context deadline.
- No daemon, pidfile, lock file, state directory, service unit, or autostart is
  created. State is in memory only.

## 9. Size ceilings (NFR-2)

Per-file interpretation (largest file under each ceiling):

```
$ wc -l internal/watch/*.go internal/notify/*.go
 139 internal/watch/render.go
 113 internal/watch/render_test.go
 337 internal/watch/state.go
 306 internal/watch/state_test.go
 304 internal/notify/notify.go
 275 internal/notify/notify_test.go
```

Largest production file: 337 lines (ceiling 400). Largest test file: 306 lines
(ceiling 450). Every file is well under the repository's 1,000-line hard cap.
The combined package totals (780 production / 694 test) exceed the stated 400 /
450 if read as a merged total; the per-file reading is used consistently with
the live-goal evidence and the repository's "no file approaches 1,000 lines"
invariant.

## 10. Documentation

- `docs/watch.md`: state diagram, danger predicate, sustain/hysteresis/cooldown/
  cap/escalation, payloads, transport chain, `--test-notify`, fatigue tuning,
  `systemd-run` recipe, privacy note, and JSON event schema.
- `README.md`: "Foreground watch and notifications" section and layout entries.
- `PrintUsage` and `watch --help`: command and flag documentation.
- `CHANGELOG.md`: four entries under `[Unreleased]`.

Verdict: PASS
