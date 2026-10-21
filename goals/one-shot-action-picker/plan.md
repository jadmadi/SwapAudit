# Plan: Add an Interactive Tiered Action Picker to the One-Shot Report

## Execution Steps

- [x] **Phase 1 — Contracts, types, and mode validation**
  - [x] Create `internal/picker` with the `Config` struct and `Run(cfg Config) int`
        signature from `facts.md`.
  - [x] Add constructor guards: nil `In`/`Out`/`Scan`/`Snapshot`/`Execute`
        return 1 with a diagnostic on `Out`; never panic.
  - [x] Clamp `Width` to the `80..120` range used by `term.GetTerminalWidth`.
  - [x] Define the four tier descriptors (key, label, category, boolean,
        resource, token strategy) in one table so rendering and execution cannot
        drift.
  - [x] Add `app`-level flag validation in a pure function that takes
        `json, yes, interactive, noInteractive bool` plus the four cleanup
        booleans and returns an error with the exact messages.
  - [x] Unit-test the validation table for every conflicting combination,
        including both `--interactive --no-interactive`.

- [x] **Phase 2 — Menu rendering**
  - [x] Render the four-row tier table with columns `KEY`, `TIER`, `ITEMS`,
        `SIZE`, `RESOURCE`, `SAFETY`.
  - [x] Compute counts and bytes from `Config.Items` using `Cleanable` and the
        four categories; use `model.ItemSize` only where the report does.
  - [x] Render zero-target tiers as `0` and `—` without hiding the row.
  - [x] Append the tier's threshold to the label (`>12h`, `>12h idle`, `>2h
        idle`, `parent gone`) exactly as the report does.
  - [x] Render the footer `Select a tier [1-4], [d] dry-run, [q] quit: ` and
        flush after every prompt.
  - [x] Assert every line is `<= Width` visible cells using `term.StrWidth`.

- [x] **Phase 3 — Review panel**
  - [x] Render the header `REVIEW tier N of 4 — <LABEL>` and the threshold line.
  - [x] Sort targets by effective size descending, then name ascending, then PID
        ascending; reuse the same tie-break rules as the cleaner.
  - [x] Render at most 10 targets; render `… and N more targets (X MB)` for the
        remainder.
  - [x] Render `PID` only for process tiers; render `AGE` only for filesystem
        tiers.
  - [x] Render the `FREES` line distinguishing RAM/tmpfs from swap.
  - [x] Render the `RISK` line: rebuildable for caches, irreversible for
        processes.
  - [x] Truncate every name and path with `term.Truncate` and pad with
        `term.StrWidth`.

- [x] **Phase 4 — Confirmation state machine**
  - [x] Implement `tokenFor(tier, items)`: decimal count for tiers 1-2, largest
        `SwapBytes` PID for tiers 3-4, ties to the lowest PID.
  - [x] Implement one-line input with `bufio.Reader.ReadString('\n')`; trim
        whitespace; treat empty input as cancel.
  - [x] Treat `q`/`Q` as quit at the menu and at the token prompt.
  - [x] Treat `io.EOF` as a clean exit with code 0 at both prompts.
  - [x] On mismatch print the exact refusal, re-prompt, and stop after three
        attempts with the exact cancellation message.
  - [x] Ensure `--yes` cannot bypass the token; the picker never sees `yes`.
  - [x] Add table-driven tests for `1`, ` 2 `, `3`, `4`, `q`, `Q`, empty, EOF,
        `0`, `5`, `12`, `x`, and three consecutive mismatches.

- [x] **Phase 5 — Execution and measured after-state**
  - [x] Build the single-tier boolean matrix and assert exactly one boolean is
        true for every key.
  - [x] Call the injected `Execute` with `dryRun` and `yes=true`; assert the
        picker never calls `cleaner.PruneTmpTargets` or
        `cleaner.KillProcessTargets`.
  - [x] Skip the rescan when `DryRun` is true.
  - [x] When not dry-run, call `Snapshot` before and after, call `Scan` once, and
        render the measured before/after line with `term.FormatBytes`.
  - [x] Rebuild the menu from the fresh items and loop until quit.
  - [x] Add a spy-executor test asserting the boolean matrix, `dryRun`, and
        `yes=true` for all four keys.
  - [x] Add a before/after test with fixed `SystemSwap` values and a fixed scan
        result; assert the exact line and delta arithmetic.

- [x] **Phase 6 — Application integration**
  - [x] Add `--interactive` and `--no-interactive` flags in `app.Run`.
  - [x] Honor `SWAPAUDIT_NO_INTERACTIVE=1` without overriding an explicit
        `--interactive` in a contradictory way (document the precedence in the
        flag validation).
  - [x] Run validation before `CollectSwapItems` so usage errors exit 2 without
        paying the ~120 ms scan.
  - [x] Insert the picker after `PrintCategorySummary` and before the cleanup
        branch.
  - [x] Keep the existing behavior when any cleanup flag is set.
  - [x] Construct exactly one `bufio.NewReader(os.Stdin)` and pass it to the
        picker; never construct a second reader.
  - [x] On picker exit, call `PrintQuickCommands` so the flags remain
        discoverable.
  - [x] Detect TTY with `os.Stdin.Stat()` and `os.Stdout.Stat()` and the
        `os.ModeCharDevice` bit; do not add a new dependency.
  - [x] Ensure the update notice still prints after the picker exits.
  - [x] Add an `app` test for the validation function and for the
        skip-on-non-TTY decision.

- [x] **Phase 7 — Documentation**
  - [x] Add an `--interactive` / `--no-interactive` section to `PrintUsage`.
  - [x] Add a README section covering tiers, token rules, dry-run, and the
        irreversibility of process tiers.
  - [x] Add a CHANGELOG entry under the unreleased heading.
  - [x] State explicitly in the README that no passcode is stored and that the
        typed token confirms scope, not identity.

- [x] **Phase 8 — Verification and evidence**
  - [x] Run `gofmt -l .` and fix formatting.
  - [x] Run `go vet ./...`.
  - [x] Run `go test -race ./internal/picker ./internal/app`.
  - [x] Run `go test -race ./...`.
  - [x] Run `./scripts/check.sh --pre-commit`.
  - [x] Run `./scripts/check.sh --full`.
  - [x] Add golden tests at widths 80, 100, and 120 with color and ASCII forced.
  - [x] Manually verify the happy path in a terminal: menu, review, wrong token,
        correct token, after-state, quit.
  - [x] Manually verify the non-TTY path with stdout redirected and with stdin
        `/dev/null`.
  - [x] Manually verify the conflict paths exit 2 with the exact messages.
  - [x] Measure `internal/picker` production and test line counts against the
        NFR-2 ceiling.
  - [x] Write `goals/one-shot-action-picker/evidence.md` with a standalone
        `Verdict: PASS` and every command output listed in the goal's evidence
        requirements.
  - [x] Check every completed task in this plan.
  - [x] Commit with
        `feat(picker): add a typed-confirmation action picker after the report (goals/one-shot-action-picker/goal.md)`.
  - [x] Certify with
        `sila goals certify one-shot-action-picker --evidence=goals/one-shot-action-picker/evidence.md`.
  - [x] Run `sila goals` and verify `unverified_count: 0` with a clean
        `git status`.
