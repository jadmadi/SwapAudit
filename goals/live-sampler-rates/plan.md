# Plan: Add a Headless Sampler with Rates, Attribution, and a Gated ETA

## Execution Steps

- [x] **Phase 1 — Fixtures and estimator (test-first)**
  - [x] Create `internal/live/testdata/` and hand-write the nine raw-sample
        fixtures from FR-11: idle, ramp, spike, recovering, noise, short,
        pid-reuse, exit, stale-tmp.
  - [x] Define the raw-sample schema and a `Replay` helper that parses NDJSON
        into `[]sample`.
  - [x] Implement the OLS slope, intercept, R², span, and non-negative-ratio
        helpers with named constants for every gate.
  - [x] Implement `SeriesStats` gating in the FR-5 order and return a nullable
        `ETASeconds`.
  - [x] Implement `Analyze` for deltas, movers, exited/started bytes, and
        `unattributed_bytes` using the explicit formulas in FR-4.
  - [x] Assert empty slices, not `nil`, for `Movers` and `TmpfsMovers`.
  - [x] Add table-driven fixture tests with the expected trend, basis, and ETA
        for every fixture.
  - [x] Compute the analytic ETA for `ramp.ndjson` in the test from free bytes
        and slope; assert within 10%.
  - [x] Add a determinism test calling `Analyze` twice on the same slice and
        comparing bytes.
  - [x] Add a PID-reuse test asserting no phantom mover, a retired key in
        `exited_bytes`, and a new key in `started_bytes`.
  - [x] Add an exit test asserting `exited_bytes > 0` and no negative mover.
  - [x] Add a stale-tmp test asserting the tmpfs ETA is `null`.

- [x] **Phase 2 — Ring and identity**
  - [x] Define `processKey{pid, start_ticks}` and the cache-path key.
  - [x] Implement `Ring` with capacity eviction, `Push`, `Snapshot`, and `Len`.
  - [x] Ensure `Snapshot` returns a copy so the analyzer cannot race the writer.
  - [x] Add a ring test for eviction order, capacity one, and empty state.
  - [x] Add a race test that pushes from one goroutine and snapshots from
        another under `-race`.

- [x] **Phase 3 — Fast lane**
  - [x] Implement `scanFast(now)` using `proc.ReadSystemSwap`,
        `/proc/<pid>/status`, `/proc/<pid>/stat`, and `/proc/<pid>/comm`.
  - [x] Track only processes with `VmSwap > 0`, mirroring `CollectSwapItems`.
  - [x] Read `/proc/<pid>/sched` only on first observation or when `VmSwap`
        changed; cache the last idle value.
  - [x] Record process names for movers without reading full command lines.
  - [x] Measure the fast lane on this workstation and record the result.
  - [x] Add a test with a synthetic slow source proving a tick is skipped, not
        queued, when the previous scan overruns.
  - [x] Add a clock-jump test proving non-positive monotonic deltas are dropped.

- [x] **Phase 4 — Slow lane and lifecycle**
  - [x] Start one slow-lane goroutine with `--tmp-interval`, protected by a
        mutex and a `sync.WaitGroup`.
  - [x] Skip a slow cycle when the previous scan is still running; increment
        `slow_lane_skips`.
  - [x] Store `tmpfs_sampled_at` and emit `tmpfs_sampled_age_seconds`.
  - [x] Sum `model.ItemSize` for `Cleanable` cache items into
        `tmpfs_cleanable_bytes` and count them into `tmpfs_cleanable_items`
        from the slow-lane result, so the notification goal needs no second
        scan.
  - [x] Use `signal.NotifyContext` for SIGINT/SIGTERM with `defer cancel()`.
  - [x] Use `time.NewTicker` with `defer ticker.Stop()`.
  - [x] Wait for the slow lane before `Run` returns.
  - [x] Add a goroutine-leak test that cancels the context and asserts no
        growth after a settle period.
  - [x] Add a test that cancellation closes the recorder and emits `stopped` in
        stream mode.

- [x] **Phase 5 — Output, streaming, and recording**
  - [x] Implement `--once` output: `type`, `sampled_at`, the shared goal-2
        `snapshot`, and `window`.
  - [x] Implement `--stream` output: one compact sample line per tick with
        `type`, `sampled_at`, `monotonic_seconds`, `system`, and `window`.
  - [x] Flush each stream line through a `bufio.Writer`.
  - [x] Implement the final `stopped` line with `reason` and `sampled_at`.
  - [x] Implement `Recorder` with mode `0600`, per-line flush, and `SetEscapeHTML(false)`.
  - [x] Implement `Replay` and assert that a recorded fixture produces the same
        stats as the original in-memory slice.
  - [x] Add a test asserting stdout contains no ANSI escape and no prose.
  - [x] Add a broken-pipe test asserting a clean exit 1 without a panic.

- [x] **Phase 6 — CLI wiring**
  - [x] Add the `sample` subcommand to the bare-word switch in `app.Run`.
  - [x] Parse and validate `--once`, `--stream`, `--interval`, `--window`,
        `--tmp-interval`, `--duration`, `--record`, and `--no-command`.
  - [x] Enforce the 600-sample cap and the range checks with the exact messages.
  - [x] Dispatch before `term.InitTerminal`, the update check, and
        `CollectSwapItems`.
  - [x] Ensure `sample` implies no update check and no human output.
  - [x] Reuse the shared snapshot package from `json-snapshot-output`; do not
        duplicate the schema.
  - [x] Add `sample` to `PrintUsage` with two examples.
  - [x] Add an app test for every validation error and for `--once --stream`.

- [x] **Phase 7 — Documentation**
  - [x] Write `docs/sampling.md` covering lanes, identity, estimator gates,
        attribution caveats, trace format, and three recipes.
  - [x] Add a README section linking to the document.
  - [x] Add a CHANGELOG entry under the unreleased heading.
  - [x] Document that traces contain names and paths, never command lines.

- [x] **Phase 8 — Verification and evidence**
  - [x] Run `gofmt -l .` and fix formatting.
  - [x] Run `go vet ./...`.
  - [x] Run `go test -race ./internal/live`.
  - [x] Run `go test -race ./...`.
  - [x] Run `./scripts/check.sh --pre-commit`.
  - [x] Run `./scripts/check.sh --full`.
  - [x] Run `swapaudit sample --once` and validate with
        `python3 -m json.tool`.
  - [x] Run a 60-second `--stream` capture and validate every line.
  - [x] Measure CPU for the 60-second run and compare against NFR-9.
  - [x] Record a trace with `--record`, verify mode `600`, and replay it.
  - [x] Verify every fixture's expected trend, basis, and ETA in the evidence.
  - [x] Verify `internal/live` stays within the NFR-2 ceiling.
  - [x] Write `goals/live-sampler-rates/evidence.md` with a standalone
        `Verdict: PASS` and every required output.
  - [x] Check every completed task in this plan.
  - [x] Commit with
        `feat(sample): add tiered sampling with gated rate and ETA (goals/live-sampler-rates/goal.md)`.
  - [x] Certify with
        `sila goals certify live-sampler-rates --evidence=goals/live-sampler-rates/evidence.md`.
  - [x] Run `sila goals` and verify `unverified_count: 0` with a clean
        `git status`.
