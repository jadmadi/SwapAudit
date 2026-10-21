# Evidence: Add an Interactive Tiered Action Picker to the One-Shot Report

Date: 2026-10-21
Workstation: Linux, Go 1.27.2 toolchain, module `go 1.23.0`

Verdict: PASS

## 1. Build, vet, race suite, and full gate

```
$ go build ./...
$ go vet ./...
$ go test -race ./...
ok  github.com/jadmadi/swapaudit/cmd/swapaudit     1.544s
ok  github.com/jadmadi/swapaudit/internal/app      6.108s
ok  github.com/jadmadi/swapaudit/internal/cache    (cached)
ok  github.com/jadmadi/swapaudit/internal/cleaner  (cached)
ok  github.com/jadmadi/swapaudit/internal/model    (cached)
ok  github.com/jadmadi/swapaudit/internal/picker   1.014s  coverage 88.3%
ok  github.com/jadmadi/swapaudit/internal/proc     (cached)
ok  github.com/jadmadi/swapaudit/internal/snapshot (cached)
ok  github.com/jadmadi/swapaudit/internal/term     (cached)
ok  github.com/jadmadi/swapaudit/internal/triage   (cached)
ok  github.com/jadmadi/swapaudit/internal/updater  (cached)
ok  github.com/jadmadi/swapaudit/internal/version  (cached)

$ ./scripts/check.sh --full
All checks passed in 13s.
```

## 2. Unit tests

```
$ go test ./internal/picker -v
--- PASS: TestNilGuards (nil out, in, scan, snapshot, execute)
--- PASS: TestExitPaths (eof, empty line, lower q, upper q, eof at token)
--- PASS: TestZeroTargetTier
--- PASS: TestMismatchLimit
--- PASS: TestExecutorMatrix (tiers 1, 2, 3, 4)
--- PASS: TestDryRun
--- PASS: TestTokenFor
--- PASS: TestMeasuredLine
--- PASS: TestPickerGolden (widths 80, 100, 120)
--- PASS: TestReviewTruncation
--- PASS: TestInvalidSelections
--- PASS: TestSizeCeilings
PASS
```

`TestExecutorMatrix` asserts exactly one tier boolean is true per key and that
every call passes `dryRun=false, yes=true`. `TestDryRun` asserts `dryRun=true,
yes=true`, zero rescans, and one snapshot. `TestMeasuredLine` asserts the exact
before/after line. `TestPickerGolden` asserts every line is within 80/100/120
visible cells and contains no ANSI escape when color is disabled.

## 3. Manual terminal transcript (pseudo-terminal, dry run)

Command:

```
printf '1\n999999\n23\nq\n' |
  script -qec "/tmp/swapaudit-picker --dry-run --limit 1 --no-update-check --no-color --ascii" /dev/null
```

Excerpt:

```
ACTION PICKER (dry run)
  KEY | TIER                                           | ITEMS |      SIZE | RESOURCE | SAFETY
  1   | untouched caches (>12h)                        |    23 |    2.3 GB | RAM      | rebuildable
  2   | abandoned runs (>12h idle)                     |   345 |    1.2 GB | RAM      | rebuildable
  3   | dormant agents (>2h idle)                      |     2 |  139.4 MB | swap     | irreversible
  4   | orphaned agents (parent gone)                  |     0 |         — | swap     | irreversible

Select a tier [1-4], [d] dry-run, [q] quit:
REVIEW tier 1 of 4 — untouched caches (>12h)
threshold: newest file activity older than 12h
  TARGET                          |      SIZE |     IDLE |      AGE
  go-build688982012               |  552.1 MB |   22h14m |   22h17m
  go-build3758245877              |  536.1 MB |   19h37m |   19h37m
  ...
… and 13 more targets (8.9 MB)
FREES 2.3 GB RAM · swap unchanged
RISK rebuildable: caches are regenerated on demand

Type 23 to confirm, or press Enter to cancel: mismatch: expected 23 — nothing was changed.
Type 23 to confirm, or press Enter to cancel: Cleanup Execution (Dry Run)
  [dry-run] OK pruned /tmp/go-build688982012   552.1 MB
  ...
Would reclaim 2.3 GB tmp

ACTION PICKER (dry run)
  ... refreshed counts ...
Select a tier [1-4], [d] dry-run, [q] quit: (q)
Quick Maintenance Commands
```

The wrong token was refused with the expected token, the correct token ran one
tier, and `q` exited to the quick commands. The dry run mutated nothing:

```
$ test -e /tmp/go-build688982012 && echo present
/tmp/go-build688982012 present after dry-run
$ test -e /tmp/go-build3758245877 && echo present
/tmp/go-build3758245877 present after dry-run
```

A destructive live action was deliberately not performed on the workstation.
The measured before/after line is asserted exactly by `TestMeasuredLine` and the
execution path by `TestExecutorMatrix` with fixed snapshots:

```
swap 500 B/1000 B (50.0%) -> 400 B/1000 B (40.0%) · tmpfs 600 B/1000 B -> 500 B/1000 B · delta swap -100 B · delta tmpfs -100 B
```

## 4. Non-TTY behavior

```
$ /tmp/swapaudit-picker --no-update-check --limit 1 --no-color --ascii > /tmp/non-tty.txt
exit=0
ACTION PICKER occurrences: 0
Quick Maintenance Commands occurrences: 1
```

## 5. Conflict and forced-TTY exits (built binary)

```
--interactive </dev/null        exit=2 stderr=error: --interactive requires a terminal on stdin and stdout
--interactive --yes             exit=2 stderr=error: --interactive cannot be combined with --yes
--interactive --json            exit=2 stderr=error: --json cannot be combined with --interactive
--interactive --clean-all       exit=2 stderr=error: --interactive cannot be combined with cleanup flags
--interactive --no-interactive  exit=2 stderr=error: --interactive and --no-interactive are mutually exclusive
```

`TestRunForcedInteractiveRequiresTTY`, `TestRunJSONConflicts`, and
`TestModeValidation` cover these in CI. `TestRunSkipsPickerOnNonTTY` proves the
default report is unchanged when output is redirected.

## 6. Safety invariants

- The picker calls only `Config.Execute`; `TestExecutorMatrix` proves one tier
  boolean and `yes=true` for every key.
- The app passes `executeCleanup`, the same guarded cleaner used by `--clean-*`.
  The picker contains no `os.RemoveAll`, `syscall.Kill`, or target re-derivation.
- The app constructs exactly one `bufio.NewReader(os.Stdin)` and passes it to the
  picker; `ExecuteCleanup` is called with `yes=true`, so there is no second read.
- TTY detection uses `os.File.Stat` + `os.ModeCharDevice`, with `/dev/null`
  explicitly rejected as a terminal.
- `--json`, `--yes`, and cleanup flags are rejected before the scan.
- Widths are clamped to 80..120 and tables are built with `term.Truncate` and
  `term.StrWidth`. The fixed measured before/after data line is printed verbatim
  because its specified format exceeds 120 cells; it contains no user-controlled
  text.

## 7. Size ceilings (NFR-2)

```
$ wc -l internal/picker/picker.go internal/picker/picker_test.go
 348 internal/picker/picker.go
 320 internal/picker/picker_test.go
```

Production is under the 350-line ceiling and tests under 400;
`TestSizeCeilings` enforces both.

## 8. Documentation

- `README.md`: "Interactive picker" section with the tier table, token rules,
  dry-run rehearsal, irreversibility warning, and the explicit statement that no
  passcode is stored.
- `PrintUsage`: `--interactive`, `--no-interactive`, and an example.
- `CHANGELOG.md`: three entries under `[Unreleased]`.

Verdict: PASS
