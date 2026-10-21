# Evidence: Add a Stable Machine-Readable JSON Snapshot

Date: 2026-10-21
Workstation: Linux, Go 1.27.2 toolchain, module `go 1.23.0`

Verdict: PASS

## 1. Build, vet, and race suite

```
$ go build ./...
$ go vet ./...
$ go test -race ./...
ok  github.com/jadmadi/swapaudit/cmd/swapaudit         (cached)
ok  github.com/jadmadi/swapaudit/internal/app          (cached)
ok  github.com/jadmadi/swapaudit/internal/cache        (cached)
ok  github.com/jadmadi/swapaudit/internal/cleaner      (cached)
ok  github.com/jadmadi/swapaudit/internal/model        (cached)
ok  github.com/jadmadi/swapaudit/internal/proc         (cached)
ok  github.com/jadmadi/swapaudit/internal/snapshot     (cached)
ok  github.com/jadmadi/swapaudit/internal/term         (cached)
ok  github.com/jadmadi/swapaudit/internal/triage       (cached)
ok  github.com/jadmadi/swapaudit/internal/updater      (cached)
ok  github.com/jadmadi/swapaudit/internal/version      (cached)
```

```
$ ./scripts/check.sh --full
1. Diff sentry clean.
2. All Go files formatted.
3. go vet passed cleanly.
4. Race-detected tests passed.
5. Testing Binary Build Pipeline...
   Built binary self-verified: swapaudit 337edba-dirty (commit: 337edba, built: 2026-10-21T07:18:27Z)
All checks passed in 11s.
```

## 2. Golden file

```
$ go test ./internal/snapshot -run TestGolden
ok  github.com/jadmadi/swapaudit/internal/snapshot  0.002s

$ go test ./internal/snapshot -run TestGolden -update
$ git diff --exit-code -- internal/snapshot/testdata/snapshot_v1.json
(no diff; regeneration is byte-identical)
```

The golden file is `internal/snapshot/testdata/snapshot_v1.json` and covers a
known-idle process, an unknown-idle process, an untouched cache, an active cache
with zero cleanable bytes, and a non-ASCII path containing `&` and `<`.

## 3. Runtime stream purity

```
$ go run ./cmd/swapaudit --json --no-update-check > /tmp/s.json 2>/tmp/s.err
exit=0
305456 /tmp/s.json
stderr_bytes=0
$ python3 -m json.tool < /tmp/s.json > /dev/null
VALID_JSON
```

stdout begins with `{"schema_version": 1, ...}` and ends with exactly one `\n`.
`TestRunJSONStreamPurity` asserts the absence of ANSI escapes, box glyphs,
`Cleanable`, the audit banner, and the update notice.

## 4. Conflict transcripts (built binary, true exit codes)

```
$ /tmp/swapaudit-test --json <flag>
--clean-untouched    exit=2 stdout_bytes=0 stderr=error: --json is read-only and cannot be combined with cleanup flags
--clean-abandoned    exit=2 stdout_bytes=0 stderr=error: --json is read-only and cannot be combined with cleanup flags
--clean-dormant      exit=2 stdout_bytes=0 stderr=error: --json is read-only and cannot be combined with cleanup flags
--clean-orphaned     exit=2 stdout_bytes=0 stderr=error: --json is read-only and cannot be combined with cleanup flags
--clean-tmp          exit=2 stdout_bytes=0 stderr=error: --json is read-only and cannot be combined with cleanup flags
--clean-all          exit=2 stdout_bytes=0 stderr=error: --json is read-only and cannot be combined with cleanup flags
--dry-run            exit=2 stdout_bytes=0 stderr=error: --json is read-only and cannot be combined with --dry-run
--yes                exit=2 stdout_bytes=0 stderr=error: --json is read-only and cannot be combined with --yes
--interactive        exit=2 stdout_bytes=0 stderr=error: --json cannot be combined with --interactive
--no-command         exit=2 stdout_bytes=0 stderr=error: --no-command requires --json
```

`TestRunJSONConflicts` and `TestModeValidation` cover the same table in CI.
`TestRunJSONAcceptsCosmeticFlags` proves `--no-color` and `--ascii` are accepted
and do not alter the document.

## 5. `--no-command` diff

`TestNoCommand` builds the same frozen-clock inputs twice, clears every
`command` on the first document, and asserts `reflect.DeepEqual` against the
`noCommand` document. Only the `command` fields differ; keys and all other
values, including nullable fields and totals, are identical.

A live `--json --no-command` run also produced `exit=0`, `stderr=0` bytes, and
every `items[].command` equal to `""`.

## 6. Hard failure

`TestRunJSONHardFailure` points the preflight seam at a missing path and asserts
exit `1`, empty stderr, and exactly:

```json
{
  "schema_version": 1,
  "error": {
    "code": "proc_unavailable",
    "message": "cannot read /proc/self/status"
  }
}
```

`TestErrorDocument` asserts the exact byte sequence.

## 7. Determinism, ordering, nullability

```
$ go test ./internal/snapshot -v
--- PASS: TestGolden
--- PASS: TestNullability
--- PASS: TestEmptyScan
--- PASS: TestDeterminism
--- PASS: TestCanonicalOrdering
--- PASS: TestNoCommand
--- PASS: TestHTMLNotEscaped
--- PASS: TestErrorDocument
--- PASS: TestRoundTripTypes
--- PASS: TestProductionSizeCeiling
--- PASS: TestMarshalLargeScan
PASS
```

- `TestDeterminism` compares two frozen-input builds byte for byte.
- `TestCanonicalOrdering` shuffles the input and asserts category, size, name,
  and PID order, plus ascending summary categories.
- `TestNullability` asserts `pid`/`start_ticks`/`path` nullability and that
  `idle_seconds: null` implies `last_active_at: null` and never `-1`.
- `TestEmptyScan` asserts `summary`, `items`, and `warnings` are empty arrays and
  totals are zero.
- `TestHTMLNotEscaped` asserts raw `&`, `<`, and `é` round-trip.

## 8. Cost

```
$ go test ./internal/snapshot -run '^$' -bench BenchmarkMarshal500Items -benchtime=50x
BenchmarkMarshal500Items-8   50   1002050 ns/op   659272 B/op   2517 allocs/op
```

1.0 ms per 500-item build+encode, well under the 20 ms ceiling.

## 9. Size ceilings (NFR-2)

```
$ wc -l internal/snapshot/snapshot.go internal/snapshot/snapshot_test.go
 284 internal/snapshot/snapshot.go
 331 internal/snapshot/snapshot_test.go
```

Both are within the 300/400 line ceilings; `TestProductionSizeCeiling` enforces
the production ceiling automatically.

## 10. Read-only proof

`TestRunJSONNeverExecutesCleanup` replaces the `executeCleanup` seam with a spy
and asserts it is never called during a successful `--json` run. JSON dispatch
returns before `term.InitTerminal`, before the update-check goroutine, and before
any human renderer.

## 11. Documentation

- `docs/json-schema.md`: field table, nullability, canonical ordering,
  `effective_size_bytes`, `idle_seconds: null`, `--no-command` privacy, hard
  failure, two consumer recipes, complete example, and the additive/v2 rule.
- `README.md`: "Machine-readable JSON output" section and CLI reference rows.
- `CHANGELOG.md`: three entries under `[Unreleased]`.
- `PrintUsage`: `--json`, `--no-command`, plus a usage example.

Verdict: PASS
