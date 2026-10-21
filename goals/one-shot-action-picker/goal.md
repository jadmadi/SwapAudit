# Goal: Add an Interactive Tiered Action Picker to the One-Shot Report

## Chain of Custody (Isnad)
- **Registered By**: Pending Registration
- **Spec Audited By**: Pending Spec Audit (personas 5, 11, 32)
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: @mystery-auditor | Report: Pending

## Goal Description

`swapaudit` is read-only unless the user re-runs it with one of six
`--clean-*` flags. The report already computes exactly which entities are
cleanable, but the handoff from "I can see it" to "I removed it" requires
memorizing flags, retyping the command, and trusting a `[y/N]` prompt that
appears after the fact. This goal closes that gap with a post-report action
picker.

The picker groups the scan into four tiers that mirror the existing categories,
renders the exact targets and the resource each tier releases, and requires the
user to type the scope before anything runs. Execution is delegated to
`cleaner.ExecuteCleanup`; the picker contains no deletion or signalling logic of
its own. Input is line-oriented (`1` then Enter), so the change needs no
termios, no alternate screen, and no goroutines.

### Desired outcome

- After the report, a four-row menu appears when stdin and stdout are terminals.
- Pressing `1`-`4` opens a review panel with the exact targets, sizes, and idle
  times, plus one line stating what the tier frees (RAM/tmpfs vs swap).
- Nothing executes until the user types the correct scope token: the target
  count for filesystem tiers, the PID of the largest target for process tiers.
- A mismatch is refused with the expected token, at most three times, then the
  tier is cancelled; no partial action runs.
- On success the picker re-scans once and prints a measured before/after for
  swap and tmpfs, then returns to the menu.
- `q`, empty input, or EOF exits without touching anything.

### Non-goals

- No full-screen TUI, alternate screen, raw mode, termios, or single-key reads.
- No live sampling, ring buffer, rates, or ETA (owned by `live-sampler-rates`).
- No JSON output (owned by `json-snapshot-output`).
- No new deletion, signalling, path-containment, or PID-reuse logic.
- No stored passcode, config file, or secret management.
- No undo, scheduled cleanup, daemon, or background work.
- No per-item exclusion in v1; the tier is the unit of action.

## Scope & Boundaries

### In scope

- `internal/picker/picker.go` (new)
- `internal/picker/picker_test.go` (new)
- `internal/app/app.go` (flag wiring and post-report dispatch)
- `internal/app/app_test.go` (mode validation)
- `internal/app/display.go` only if a help line must be added
- `README.md`, `PrintUsage`, and `CHANGELOG.md` documentation
- `goals/one-shot-action-picker/evidence.md`

### Out of scope

- `internal/cleaner/cleaner.go` semantics; the picker calls the exported
  `ExecuteCleanup` unchanged.
- `internal/term` table rendering; the menu is rendered with `fmt.Fprintf` to an
  injected writer so tests do not need to capture `os.Stdout`.
- Any TUI dependency, terminal raw-mode package, or `golang.org/x/sys`.
- Changes to `CollectSwapItems`, `triage`, `cache`, or `proc`.

## Requirements

### Functional requirements

**FR-1 — Activation and precedence.**

| Condition | Behavior |
| :--- | :--- |
| Any `--clean-*`, `--clean-all`, or `--clean-tmp` | Picker skipped; the existing `ExecuteCleanup` path runs unchanged. |
| `--json` | Picker skipped; JSON mode owns stdout (goal `json-snapshot-output`). |
| `--yes` with `--interactive` | Exit 2 with `error: --interactive cannot be combined with --yes`. |
| `--json` with `--interactive` | Exit 2 with `error: --interactive cannot be combined with --json`. |
| Any cleanup flag with `--interactive` | Exit 2 with `error: --interactive cannot be combined with cleanup flags`. |
| stdin or stdout is not a character device | Picker skipped by default; with `--interactive`, exit 2 with `error: --interactive requires a terminal on stdin and stdout`. |
| `--no-interactive` or `SWAPAUDIT_NO_INTERACTIVE=1` | Picker skipped; `PrintQuickCommands` runs as today. |
| `--interactive` with `--no-interactive` | Exit 2 with `error: --interactive and --no-interactive are mutually exclusive`. |
| Otherwise | Picker runs after `PrintCategorySummary`, before `PrintQuickCommands`. |

Default-on is intentional: scripts, pipes, and CI have no TTY, so their behavior
does not change. `--interactive` forces the picker and turns the silent skip into
a usage error.

**FR-2 — Tier mapping.** The menu has exactly four rows, in this order:

| Key | Label | Category | `ExecuteCleanup` argument | Resource released |
| :---: | :--- | :--- | :--- | :--- |
| 1 | untouched caches (>12h) | `model.CatUntouchedCache` | `cleanUntouched` | tmpfs (RAM-backed) |
| 2 | abandoned runs (>12h idle) | `model.CatAbandonedRun` | `cleanAbandoned` | tmpfs (RAM-backed) |
| 3 | dormant agents (>2h idle) | `model.CatDormantAgent` | `cleanDormant` | process swap |
| 4 | orphaned agents (parent gone) | `model.CatAbandonedAgent` | `cleanOrphaned` | process swap |

Each row shows: key, label, item count, total effective bytes, and the resource.
Zero-target tiers render `0` and `—`; pressing the key prints
`tier N: nothing to clean` and re-prompts without opening a review panel.

**FR-3 — Review panel.** A selected tier renders:

1. Header: `REVIEW tier N of 4 — <LABEL>`.
2. One line stating the threshold that produced the tier.
3. Up to 10 targets sorted by effective size descending, then name ascending,
   then PID ascending (matching `cleaner.PruneTmpTargets`/`KillProcessTargets`
   ordering where applicable).
4. Columns: `PID` (processes only), `TARGET`, `SIZE`, `IDLE`, and for filesystem
   tiers `AGE`.
5. A truncation line `… and N more targets (X MB)` when more than 10 exist.
6. A `FREES` line: `FREES 367.5 MB RAM · swap unchanged` for tiers 1-2, or
   `FREES 197.9 MB swap · RAM 12.0 MB` for tiers 3-4.
7. A `RISK` line: caches are rebuildable; process tiers are irreversible.

All widths are computed from `term.TermWidth()` and every user-controlled string
passes through `term.Truncate`; no line may exceed the terminal width.

**FR-4 — Scope token.** The confirmation token is:

- Tiers 1 and 2: the decimal count of cleanable targets in the tier (for
  example `5` or `25`).
- Tiers 3 and 4: the PID of the target with the largest `SwapBytes`; ties break
  to the lowest PID (for example `1618347`).

Input rules: trim surrounding whitespace; empty input cancels the tier; `q`
(exact, case-insensitive) exits the picker; EOF exits the picker with code 0.
On mismatch print `mismatch: expected <token> — nothing was changed.` and
re-prompt. After three mismatches print
`cancelled after 3 attempts — nothing was changed.` and return to the menu.
There is no bypass, including `--yes`.

**FR-5 — Execution contract.** Exactly one tier runs per action:

```go
cleaner.ExecuteCleanup(items,
    key == 1, // cleanUntouched
    key == 2, // cleanAbandoned
    key == 3, // cleanDormant
    key == 4, // cleanOrphaned
    dryRun,
    true,     // the picker already confirmed; suppress the second [y/N]
)
```

The picker must pass the original `items` slice. It must never call
`PruneTmpTargets` or `KillProcessTargets` directly, never re-derive targets, and
never relax the cleaner's path-containment, UID, symlink, PID<=1, self-PID, or
`StartTicks` checks.

**FR-6 — Measured after-state.** When `!dryRun`, after `ExecuteCleanup` returns:

1. Read `proc.ReadSystemSwap()` again.
2. Re-run `CollectSwapItems` once with the same thresholds and patterns.
3. Print exactly one line:
   `swap 16.1/17.1 GiB (94.2%) -> 15.9/17.1 GiB (93.0%) · tmpfs 14.9/15.6 GiB -> 14.5/15.6 GiB · delta swap -197.9 MB · delta tmpfs -367.5 MB`
4. Re-render the menu from the new items.

Deltas come from the system counters, not from assumptions about what the tier
contained. A dry run skips the rescan.

**FR-7 — Loop.** The menu repeats until `q`, empty input, or EOF. After each
successful action, tier counts and sizes are recomputed from the fresh scan.

**FR-8 — Dry run.** `--dry-run` keeps the picker active, labels the menu
`(dry run)`, still requires the scope token, calls `ExecuteCleanup` with
`dryRun=true, yes=true`, skips the rescan, and must not delete, signal, or write
anything.

**FR-9 — Seams for tests.** The picker is a package with an injected
configuration:

```go
type Config struct {
    In       *bufio.Reader
    Out      io.Writer
    Items    []model.SwapItem
    DryRun   bool
    Width    int
    Scan     func() []model.SwapItem        // rescan after an action
    Snapshot func() model.SystemSwap        // system counters for before/after
    Execute  func(items []model.SwapItem, untouched, abandoned, dormant, orphaned, dryRun, yes bool)
}

func Run(cfg Config) int
```

`app.Run` wires the real implementations. Tests inject buffers, spies, and fixed
data. The picker must not read `os.Stdin` or write `os.Stdout` directly.

**FR-10 — Exact output strings.** The implementation must use these strings so
tests and documentation agree:

- Prompt: `Select a tier [1-4], [d] dry-run, [q] quit: `
- Token prompt: `Type <token-hint> to confirm, or press Enter to cancel: `
- Mismatch: `mismatch: expected <token> — nothing was changed.`
- Attempt limit: `cancelled after 3 attempts — nothing was changed.`
- Empty tier: `tier N: nothing to clean`
- Non-TTY forced: `error: --interactive requires a terminal on stdin and stdout`
- Conflict: `error: --interactive cannot be combined with <flag>`

**FR-11 — Flags and environment.** Add `--interactive` and `--no-interactive`;
honor `SWAPAUDIT_NO_INTERACTIVE=1`. Update `PrintUsage` with both flags and one
example. Passing both flags is an exit-2 error
(`error: --interactive and --no-interactive are mutually exclusive`); the
environment variable only disables the picker and cannot override an explicit
`--interactive`.

**FR-12 — Documentation.** Add a README section "Interactive picker" covering
activation, the four tiers, the token rule, dry-run rehearsal, and the fact that
process tiers are irreversible. Add one CHANGELOG entry under the unreleased
section.

**FR-13 — Exit codes.** `0` for normal completion, cancellation, and EOF; `2`
for usage/conflict/non-TTY-forced errors; `1` reserved for an unexpected write or
read error. EOF is not an error.

**FR-14 — Formatting.** Use `term.FormatBytes`, `term.FormatDuration`, and
`term.Truncate` for parity with the report. Respect `term`'s color setting; tests
force color off. No ANSI escape may appear when color is disabled.

### Non-functional requirements

- **NFR-1 — Dependency floor.** Standard library only; Linux build tag; Go 1.23.
- **NFR-2 — Size ceiling.** `internal/picker` production code at most 350 lines;
  tests at most 400 lines; no file may approach the 1,000-line repository cap.
- **NFR-3 — Determinism.** Identical `Config` inputs produce byte-identical
  output. Golden tests run at widths 80, 100, and 120.
- **NFR-4 — No concurrency.** No goroutines, timers, signal handlers, or
  background work. Ctrl-C keeps its default process-terminating behavior.
- **NFR-5 — Single reader.** The picker owns stdin for its lifetime. Because
  `ExecuteCleanup` is called with `yes=true`, it never reads stdin; this avoids
  competing `bufio.Reader` buffers and swallowed keystrokes.
- **NFR-6 — Safety.** The picker performs no filesystem writes and sends no
  signals. All mutations flow through the already-tested cleaner guards.
- **NFR-7 — Width safety.** Every rendered line is `<= Width` visible cells;
  `term.StrWidth` is the authority, not `len`.
- **NFR-8 — Predictability.** The menu never appears when stdout is redirected,
  so `swapaudit > report.txt` and `swapaudit | grep` behave exactly as today.

## Acceptance Criteria

1. `go build ./...` and `go vet ./...` pass.
2. `go test -race ./internal/picker ./internal/app` passes.
3. `go test -race ./...` passes.
4. `./scripts/check.sh --pre-commit` passes; `./scripts/check.sh --full` passes.
5. Running `swapaudit --no-update-check` in a terminal prints the four-row menu
   after the report; running it with stdout redirected prints no menu.
6. `swapaudit --interactive --yes`, `--interactive --json`, and
   `--interactive --clean-all` each exit 2 with the specified message.
7. Pressing a zero-target tier prints `tier N: nothing to clean` and returns to
   the menu.
8. Pressing `1` renders the review panel with the exact count, bytes, and
   targets; no filesystem change occurs before the token is accepted.
9. Typing a wrong token three times returns to the menu with no mutation.
10. Typing the correct token calls the injected executor exactly once with
    `yes=true` and the correct single-tier boolean.
11. A successful action prints the measured before/after line and the refreshed
    menu; `git status` and `/tmp` show only the intended removal.
12. `--dry-run` rehearses the same flow, prints `[dry-run]` output from the
    cleaner, and mutates nothing.
13. EOF and `q` exit with code 0 and no action.
14. Golden output at widths 80/100/120 contains no line wider than the width and
    no ANSI escapes when color is disabled.
15. README, `PrintUsage`, and CHANGELOG document the feature and the
    irreversible process tiers.
16. No file in `internal/picker` exceeds the NFR-2 line ceiling.

## Evidence Requirements

`goals/one-shot-action-picker/evidence.md` with a standalone `Verdict: PASS`
and:

- `git diff --stat` for the implementation commit;
- `go test -race ./...` output;
- `./scripts/check.sh --pre-commit` and `--full` output;
- unit-test names covering selection parsing, token generation, mismatch limits,
  zero-target tiers, EOF, dry-run, and the single-tier executor matrix;
- a manual terminal transcript showing: menu, review panel, wrong token refusal,
  correct token, measured before/after, and quit;
- a non-TTY transcript proving the menu is absent and exit code is 0;
- `go test -run TestPickerGolden` output for widths 80/100/120;
- confirmation that `internal/picker` stays within the line ceiling.

## Dependencies & Execution Order

- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: 1
- **Shape**: ship
- **Tier**: roadmap

## Files to Touch

- internal/picker/picker.go
- internal/picker/picker_test.go
- internal/app/app.go
- internal/app/app_test.go
- internal/app/display.go
- README.md
- CHANGELOG.md
- goals/one-shot-action-picker/evidence.md

## Persona & Audit Plan

- **Persona 5 — senior-fullstack-engineer:** review the `Config` seam, flag
  precedence, error paths, and the single-reader invariant.
- **Persona 11 — ux-information-architect:** review reading order, the review
  panel's information density, the wording of the irreversible warning, and the
  cost of the typed token.
- **Persona 32 — mystery-auditor:** attempt to delete outside `/tmp`, signal
  PID 1 or the auditor itself, exploit a recycled PID between review and
  execution, loop on EOF, bypass the token via `--yes`, and produce a menu wider
  than 80 columns.
- **Commands:** `sila persona get 5`, `sila persona get 11`,
  `sila persona get 32`, `sila hire assess 5,11,32`.
- **Artifacts:** `goals/one-shot-action-picker/evidence.md` (persona findings may
  be recorded there or in a sibling `persona-review.md`).

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation | Test |
| :--- | :---: | :---: | :--- | :--- |
| Swallowed keystrokes from a second `bufio.Reader` | Medium | High | Picker owns stdin; `ExecuteCleanup` gets `yes=true` | EOF/token tests; manual transcript |
| Accidental deletion from a mistyped key | Low | High | Review panel + typed scope token + 3-attempt limit | Mismatch tests |
| Menu blocks scripts or CI | Medium | Medium | TTY gate on stdin and stdout; `--no-interactive` | Redirect test |
| Width overflow at 80 columns | Medium | Medium | `term.Truncate` + `term.StrWidth`; goldens at 3 widths | Golden tests |
| Stale targets after review | Medium | High | Cleaner re-verifies `StartTicks` and `Lstat`; picker passes original items | PID-reuse and symlink integration tests |
| `--dry-run` still mutates | Low | High | Pass `dryRun=true`; executor spy asserts it | Dry-run test |
| Duplicate confirmation fatigue | Medium | Low | One typed token, no second `[y/N]` because `yes=true` | Executor matrix test |
| Picker appears in `--json` mode | Low | High | Explicit conflict exit 2 | Flag table test |

## Open Questions / Deferred Decisions

- Should the picker become opt-in (`--interactive`) after one release? The goal
  specifies default-on with a TTY gate; the maintainer may flip FR-1 after
  field feedback.
- Should per-item exclusion (space to deselect) be added? Deferred; the tier is
  the unit of action in v1.
- Should the after-state include a projected effect before execution? Deferred
  to `live-sampler-rates`, which owns rates and ETA.
- Should the picker log actions to a file? The cleaner already prints every
  target; a durable log is deferred until there is a state directory contract.

## References

- **Shared Understanding & Fact Sheet**:
  [`goals/one-shot-action-picker/facts.md`](facts.md)
- **Execution Plan**: [`goals/one-shot-action-picker/plan.md`](plan.md)
- `internal/app/app.go` — report flow and cleanup dispatch.
- `internal/cleaner/cleaner.go` — `ExecuteCleanup`, guards, prompts.
- `internal/model/model.go` — categories and item fields.
- `internal/term/term.go` — width, truncation, formatting, color gate.
- `goals/json-snapshot-output/goal.md` — machine-readable mode that must bypass
  the picker.
- `goals/live-sampler-rates/goal.md` — deferred rates/ETA work.

## Done Condition

1. All tasks in `plan.md` are checked (`- [x]`); unchecked tasks block the
   `implemented` transition.
2. Implementation is committed with `(goals/one-shot-action-picker/goal.md)` or
   `[goal: one-shot-action-picker]` attribution under a `feat:`, `fix:`,
   `refactor:`, `perf:`, or `chore:` prefix.
3. Goal is certified (`goals/one-shot-action-picker/certification.json` with
   `verified: true` or a signed `audit-receipt.json`) and any markdown evidence
   contains an explicit standalone `Verdict: PASS` declaration.
4. `sila goals --json` reports `status: "implemented"`,
   `verification_status: "verified"`, and `unverified_count: 0` for delivered
   goals, with a clean `git status`.
5. `go test -race ./...`, `./scripts/check.sh --full`, `sila facts verify`, and
   `sila gate` pass.
