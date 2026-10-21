# Facts: Add an Interactive Tiered Action Picker to the One-Shot Report

## 1. Grounded Evidence (live, 2026-10-21)

### 1.1 Current report flow

- `app.Run` parses flags at `internal/app/app.go:135-169`, validates `--sort` at
  `205-212`, scans once at `214-219`, then renders in this order:
  `PrintHeader` (221), `PrintActionableSection` (222), `PrintMainTable` (223),
  `PrintCategorySummary` (224).
- After rendering, cleanup flags are consolidated at `227-236`:
  `--clean-all` sets all four; `--clean-tmp` sets untouched + abandoned.
- If any of the four booleans is set, `cleaner.ExecuteCleanup` runs at `239`;
  otherwise `PrintQuickCommands` runs at `241`.
- The update notice prints after cleanup at `244-258`. It writes to stdout via
  `term.PrintUpdateNotice`, which uses `fmt.Println`.
- `Run` returns `0` on success and `2` on flag/`--sort` errors.

### 1.2 Cleaner interface and guards

- `cleaner.ExecuteCleanup(items []model.SwapItem, cleanUntouched, cleanAbandoned,
  cleanDormant, cleanOrphaned, dryRun, yes bool)` at
  `internal/cleaner/cleaner.go:180`. It returns nothing.
- It always prints a box header (`Cleanup Execution` or
  `Cleanup Execution (Dry Run)`) at `181-185`.
- Each enabled group filters `items` by category and `Cleanable` at
  `200-247`, then calls `PruneTmpTargets` or `KillProcessTargets`.
- `yes=true` skips the per-group `AskConfirmation` prompt
  (`cleaner.go:56-63` and `137-144`). This is the reuse path for a picker that
  has already confirmed.
- `PruneTmpTargets` sorts by `SizeBytes` desc then path, rejects empty, `/`,
  `/tmp`, `/var/tmp`, relative paths, paths outside `/tmp/`, `/var/tmp/`, or
  `os.TempDir()` (`cleaner.go:70-77`), rejects symlinks after `Lstat`
  (`79-86`), rejects non-UID-owner targets (`87-92`), and calls
  `os.RemoveAll` (`93-96`).
- `KillProcessTargets` sorts by `SwapBytes` desc then PID, skips `PID <= 1` and
  `os.Getpid()` (`148-150`), re-reads `StartTicks` and refuses if the process is
  gone or the PID was reused (`152-160`), then sends `SIGTERM` (`161`).
- There is no `SIGKILL` escalation in the current code; the picker must not add
  one.
- `cleaner.defaultStdinReader` is package-level and initialized from
  `os.Stdin` at `cleaner.go:19`. `AskConfirmation` uses it at `22-24`.
- If the picker created a second reader and `ExecuteCleanup` also read stdin,
  buffered bytes could be lost. The picker must call `ExecuteCleanup` with
  `yes=true`, so no second read occurs.

### 1.3 Terminal and input facts

- No file in the repository reads `os.Stdin` except
  `internal/cleaner/cleaner.go`. There is no termios, `MakeRaw`, `SIGWINCH`, or
  alternate-screen handling anywhere (`grep -rln "termios\|TCGETS\|MakeRaw\|SIGWINCH"
  --include='*.go' .` returns nothing).
- `term.InitTerminal` (`internal/term/term.go:49-72`) sets color on only when
  stdout is a character device and `NO_COLOR`/`TERM=dumb` are absent.
- `term.GetTerminalWidth` (`term.go:23-46`) clamps to 80-120 and defaults to
  100. The report already branches on `w >= 100` in `display.go`.
- `term.Truncate` and `term.StrWidth` are the width authority and understand
  ANSI sequences. Tests can force width, ASCII, and color with
  `term.SetTermWidth`, `term.SetAsciiMode`, and `term.SetColorEnabled`
  (`term.go:74-92`).

### 1.4 Model facts

- `model.SwapItem` fields: `PID`, `StartTicks`, `Name`, `Command`, `Category`,
  `SwapBytes`, `SizeBytes`, `Age`, `LastActive`, `LastActiveAt`, `Path`,
  `Cleanable`, `Count` (`internal/model/model.go:19-33`).
- `model.ItemSize` returns `SizeBytes` when positive, else `SwapBytes`
  (`model.go:55-59`).
- Categories used by the four tiers: `CatUntouchedCache`, `CatAbandonedRun`,
  `CatDormantAgent`, `CatAbandonedAgent` (`model.go:8-16`).
- `cache.ScanTmpDirectoriesIn` sets `SizeBytes`, `Path`, `Age`, `LastActive`,
  and `LastActiveAt` for filesystem items and leaves `SwapBytes` at zero.
- `CollectSwapItems` sets `SwapBytes` for process items and leaves `SizeBytes` at
  zero.

### 1.5 Measured cost and real data

- Five consecutive full scans on this workstation: 92, 109, 112, 162, 144 ms.
- The scan covered 491 processes, 1,546 `/tmp` entries, and 37,655 files.
- A real report run reported 577.7 MB cleanable:
  untouched caches 367.5 MB (5 candidates), abandoned runs 12.4 MB (25),
  dormant agents 197.9 MB (3), orphaned agents 0.
- The post-action rescan therefore costs roughly 0.1-0.2 s once per executed
  tier, which is acceptable for an explicit action.

### 1.6 Build and verification facts

- `go.mod` declares `go 1.23.0` and no dependencies.
- `Makefile` targets: `test`, `build`, `lint`, `cover`, `install`, `all`.
- `scripts/check.sh --pre-commit` runs scoped formatting, a diff sentry, and
  modified-package tests; `--full` adds `go vet`, `golangci-lint`, the race
  detector, coverage, and a build/self-verification check.
- Every package uses the `//go:build linux` constraint where it touches
  `/proc` or syscalls.

## 2. Architectural Invariants & Constraints

- The picker must not implement deletion or signalling. It delegates to
  `ExecuteCleanup` and passes the original items.
- `yes=true` is mandatory after the picker's typed confirmation. A second
  `[y/N]` prompt is a duplicate confirmation and a stdin-buffer hazard.
- The picker must never call `AskConfirmation` or create a second
  `bufio.Reader` over `os.Stdin`.
- The picker may only run when both stdin and stdout are character devices,
  unless `--interactive` is passed, in which case a non-TTY is a usage error.
- `--json`, `--yes`, and cleanup flags are mutually exclusive with
  `--interactive` and must fail with exit 2.
- No terminal state is mutated: no raw mode, no alternate screen, no cursor
  hiding, no signal handlers.
- No new filesystem paths are written. The only mutations are the cleaner's.
- All rendered widths come from `term.TermWidth()` and `term.StrWidth`.
- Output must be identical for identical injected inputs; time and scan data
  come through `Config`, not global state.

## 3. File & Interface Contracts

### `internal/picker/picker.go`

```go
package picker

type Config struct {
    In       *bufio.Reader
    Out      io.Writer
    Items    []model.SwapItem
    DryRun   bool
    Width    int
    Scan     func() []model.SwapItem
    Snapshot func() model.SystemSwap
    Execute  func(items []model.SwapItem, untouched, abandoned, dormant, orphaned, dryRun, yes bool)
}

func Run(cfg Config) int
```

- `Run` returns 0 on quit/cancel/EOF and 1 only on an unrecoverable I/O error.
- If `Width < 80`, clamp to 80. If `Width > 120`, clamp to 120 (parity with
  `term.GetTerminalWidth`).
- If `Scan`, `Snapshot`, or `Execute` is nil, `Run` returns 1 without panicking.
- `Run` must not print outside `cfg.Out` and must not read outside `cfg.In`.

### Tier table

| Key | Category | Boolean | Token |
| :---: | :--- | :--- | :--- |
| 1 | `CatUntouchedCache` | `cleanUntouched` | count |
| 2 | `CatAbandonedRun` | `cleanAbandoned` | count |
| 3 | `CatDormantAgent` | `cleanDormant` | largest-swap PID |
| 4 | `CatAbandonedAgent` | `cleanOrphaned` | largest-swap PID |

### `internal/app/app.go`

- Add flags: `interactive`, `noInteractive`.
- Add validation before the scan so usage errors do not pay the scan cost.
- After `PrintCategorySummary`, call the picker when eligible; on quit, call
  `PrintQuickCommands` so the flags remain discoverable.
- Pass a single `bufio.NewReader(os.Stdin)` to the picker.

### Exact strings

- Menu prompt: `Select a tier [1-4], [d] dry-run, [q] quit: `
- Token prompt: `Type <token-hint> to confirm, or press Enter to cancel: `
- Mismatch: `mismatch: expected <token> — nothing was changed.`
- Attempt limit: `cancelled after 3 attempts — nothing was changed.`
- Empty tier: `tier N: nothing to clean`
- Forced non-TTY: `error: --interactive requires a terminal on stdin and stdout`
- Conflicts: `error: --interactive cannot be combined with --yes|--json|cleanup flags`

## 4. External Contracts & Failure Modes

| Condition | Detection | Expected behavior |
| :--- | :--- | :--- |
| stdin is a pipe or `/dev/null` | `os.Stdin.Stat()` mode has no `ModeCharDevice` | Skip by default; exit 2 if `--interactive` |
| stdout is redirected | `os.Stdout.Stat()` mode has no `ModeCharDevice` | Skip by default; exit 2 if `--interactive` |
| EOF during menu | `ReadString` returns `io.EOF` with empty input | Exit 0, no action |
| EOF during token | `ReadString` returns `io.EOF` | Exit 0, no action |
| Token mismatch | trimmed input != expected | Refuse, re-prompt, max 3 |
| Zero-target tier | computed target count == 0 | Print `nothing to clean`, re-prompt |
| Cleaner skips a target | `StartTicks` mismatch, symlink, wrong UID | Cleaner prints the skip; picker continues |
| Cleaner removal fails | `os.RemoveAll` error | Cleaner prints failure; picker continues |
| Width < 80 | `cfg.Width` | Clamp to 80; never emit a wider line |
| `--yes` with `--interactive` | flag validation | Exit 2 before scanning |
| `--json` with `--interactive` | flag validation | Exit 2 before scanning |
| Cleanup flag with `--interactive` | flag validation | Exit 2 before scanning |
| `Scan`/`Snapshot`/`Execute` nil | constructor guard | Exit 1 with a diagnostic to `Out` |

## 5. Verification Matrix

| Test | Command | Expected |
| :--- | :--- | :--- |
| Unit tests | `go test -race ./internal/picker` | Pass |
| App validation | `go test -race ./internal/app -run TestModeValidation` | Pass |
| Golden widths | `go test ./internal/picker -run TestPickerGolden` | 80/100/120 exact |
| Full suite | `go test -race ./...` | Pass |
| Pre-commit gate | `./scripts/check.sh --pre-commit` | Pass |
| Full gate | `./scripts/check.sh --full` | Pass |
| Non-TTY skip | `go run ./cmd/swapaudit --no-update-check > /tmp/r.txt` | No menu, exit 0 |
| Forced non-TTY | `go run ./cmd/swapaudit --interactive < /dev/null` | Exit 2, exact message |
| Conflict | `go run ./cmd/swapaudit --interactive --yes` | Exit 2, exact message |
| Dry run | manual `--dry-run`, choose tier 1, type count | `[dry-run]` output, no mutation |
| Manual flow | terminal session | Menu, review, refusal, success, after-state |

## 6. Open Questions / Deferred Decisions

- Default-on versus opt-in. The goal chooses default-on with a TTY gate; this is
  the only decision likely to need field feedback.
- Whether to persist an action log under a future `XDG_STATE_HOME` contract.
- Whether to support per-item selection after the tier model proves insufficient.
- Whether `--interactive --dry-run` should skip the typed token for faster
  rehearsal. The goal keeps the token for realism.

## 7. References

- `internal/app/app.go`
- `internal/app/display.go`
- `internal/cleaner/cleaner.go`
- `internal/model/model.go`
- `internal/term/term.go`
- `internal/proc/proc.go`
- `goals/cleaner-safety-guards/goal.md` (the guards this picker relies on)
- `goals/json-snapshot-output/goal.md` (mutually exclusive mode)
