# Plan: Add a Foreground Watch Mode with Reliable Threshold Notifications

## Execution Steps

- [x] **Phase 1 — Trigger contracts and the state machine (test-first)**
  - [x] Define `Config`, `State`, `Decision`, and `Event` types in
        `internal/watch/state.go` with an injected `Now func() time.Time`.
  - [x] Implement the danger predicate exactly as FR-3: ETA threshold OR
        absolute pressure plus a rising rate.
  - [x] Implement the critical band: ETA ≤ threshold/3 OR pressure ≥ 95%.
  - [x] Implement sustain: danger must hold for `--sustain` and at least two
        evaluations before an alert.
  - [x] Implement the per-resource state machine `armed -> pending -> alerted ->
        recovering -> armed` with the FR-5 transitions.
  - [x] Implement cooldown, dedup key, hourly cap, and one escalation per
        incident.
  - [x] Implement re-arm hysteresis using `--rearm-eta` and
        `--rearm-pressure`.
  - [x] Implement the tmpfs stale-sample suppression and its single stderr line.
  - [x] Ensure decisions are returned in a stable order (swap before tmpfs).
  - [x] Add table-driven tests for idle, ramp, spike, recovering, one-tick blip,
        sustained danger, cooldown, cap, escalation, and recovery.
  - [x] Add a pure-determinism test: the same event sequence and clock produce
        byte-identical decisions.
  - [x] Add a test that `--resource=swap` never evaluates tmpfs and vice versa.

- [x] **Phase 2 — Top grower and payload rendering**
  - [x] Select the top swap mover from positive deltas, excluding `exited`.
  - [x] Select the top tmpfs mover from `TmpfsMovers`.
  - [x] Fall back to `top: null` and `unaccounted (shared/kernel)` when no
        positive mover exists.
  - [x] Compute `delta_bytes_per_min` from the window length, not a single tick.
  - [x] Implement the title for ETA danger and pressure-only danger.
  - [x] Implement the three-line body with size/pct/rate/sustain, top grower,
        and the reclaimable remedy.
  - [x] Use `TmpfsCleanableBytes` and its sample age for tmpfs alerts only;
        never imply that tmpfs bytes relieve swap.
  - [x] Truncate names to 40 visible cells and enforce the 512-byte body cap.
  - [x] Strip ANSI and collapse newlines for the stderr transport.
  - [x] Add golden tests for process-top, cache-top, null-top, pressure-only,
        and stale-tmpfs payloads.
  - [x] Add a test proving no command line, argument, or environment value can
        enter the payload.

- [x] **Phase 3 — Transport chain**
  - [x] Implement `Select` with the FR-8 probe order and environment checks.
  - [x] Implement `notify-send` argv construction with severity mapping.
  - [x] Implement the `gdbus` Notify call with the exact argument order and
        GVariant literals.
  - [x] Implement the `dbus-send` Notify call with explicit type prefixes.
  - [x] Implement `kdialog --title ... --passivepopup ...`.
  - [x] Implement OSC 777 by opening `/dev/tty` and writing the escape sequence.
  - [x] Implement the stderr fallback and the one-line warning format.
  - [x] Ensure every invocation uses `exec.CommandContext` with a 3s timeout and
        no shell.
  - [x] Implement `--notify-cmd` as argv only; reject an empty command.
  - [x] Add tests with injected `LookPath`, `Getenv`, `Run`, and `OpenTTY`
        covering every availability and failure combination.
  - [x] Add a metacharacter test: `; rm -rf /` remains a single argv element.
  - [x] Add a transport-failure test proving fallback order and a single
        warning per transport.

- [x] **Phase 4 — Sender goroutine and backpressure**
  - [x] Implement a buffered channel of capacity 4 and a non-blocking enqueue.
  - [x] Drop and count events when the channel is full.
  - [x] Start exactly one sender goroutine; it is the only caller of transport
        `Send`.
  - [x] Close the channel on shutdown, drain the sender, and wait before return.
  - [x] Add a goroutine-leak test with context cancellation.
  - [x] Add a hung-transport test asserting the 3s timeout and continued
        sampling.

- [x] **Phase 5 — Watch loop and CLI**
  - [x] Add the `watch` subcommand to the bare-word switch before
        `term.InitTerminal`.
  - [x] Implement FR-2 validation with the exact messages; exit 2 before
        starting the sampler.
  - [x] Forward `--interval`, `--window`, and `--tmp-interval` to the sampler.
  - [x] Evaluate `WindowStats` on each tick and enqueue decisions.
  - [x] Implement `--status-every` output to stderr, `--quiet`, and
        `--status-every=0`.
  - [x] Implement `--json` event output and disable transport sends.
  - [x] Implement `--test-notify` and its exit codes.
  - [x] Ensure the update check is disabled and stdout stays clean unless
        `--json` is set.
  - [x] Add app tests for every validation combination and for `--test-notify`
        short-circuiting the sampler.

- [x] **Phase 6 — JSON events and documentation**
  - [x] Emit `alert`, `recovered`, `suppressed`, and `stopped` event objects with
        the FR-11 fields.
  - [x] Ensure `eta_seconds` is null for pressure-only alerts and `top` is null
        when unattributed.
  - [x] Add NDJSON validation tests and a golden event sequence for the ramp
        fixture.
  - [x] Write `docs/watch.md` with the state diagram, trigger rules, transport
        chain, `--test-notify`, fatigue tuning, systemd-run recipe, and privacy
        note.
  - [x] Add `watch` to `PrintUsage` with two examples.
  - [x] Add a README section and a CHANGELOG entry.
  - [x] Document the systemd-run recipe as the only supported background mode.

- [x] **Phase 7 — Verification and evidence**
  - [x] Run `gofmt -l .` and fix formatting.
  - [x] Run `go vet ./...`.
  - [x] Run `go test -race ./internal/watch ./internal/notify`.
  - [x] Run `go test -race ./...`.
  - [x] Run `./scripts/check.sh --pre-commit`.
  - [x] Run `./scripts/check.sh --full`.
  - [x] Replay every sampler fixture through `Evaluate` and record the event
        sequence in the evidence.
  - [x] Run `watch --test-notify` on this machine and record the selected
        transport and result.
  - [x] Force every transport to fail with injected fakes and record the stderr
        fallback and warning.
  - [x] Run a 60-second `watch` and measure CPU against the sampler baseline.
  - [x] Capture a `--json` stream and validate every line.
  - [x] Verify no pidfile, lock file, state directory, or ANSI escape exists.
  - [x] Verify `internal/watch` and `internal/notify` stay within the NFR-2
        ceiling.
  - [x] Write `goals/notification-watch/evidence.md` with a standalone
        `Verdict: PASS` and the required outputs.
  - [x] Check every completed task in this plan.
  - [x] Commit with
        `feat(watch): add threshold notifications with a probed transport chain (goals/notification-watch/goal.md)`.
  - [x] Certify with
        `sila goals certify notification-watch --evidence=goals/notification-watch/evidence.md`.
  - [x] Run `sila goals` and verify `unverified_count: 0` with a clean
        `git status`.
