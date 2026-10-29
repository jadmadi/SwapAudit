# Evidence: machine-readable audit

Verdict: PASS

Date: 2026-10-20.

## Live JSON

```text
$ ./swapaudit --json > /tmp/audit.json
$ python3 -c "import json; d=json.load(open('/tmp/audit.json')); print(...)"
schema 1 cleanup None items 557
swap 2658652160 + 3576270848 = 6234923008 True
tmpfs 2845933568 + 1902166016 = 4748099584 True
```

`stdout` parses as one object, contains no ANSI escapes, and contains no human banner. `cleanup` is `null` for an audit-only run. `schema_version` is `1`. The `scan` object carries the liveness evidence:

```text
{"scanned_caches":356,"in_use_caches":356,"orphaned_caches":0,"recent_caches":0,
 "evidence_incomplete":356,"open_index_truncated":false,"open_index_errors":16,
 "open_index_pids":430,"open_index_fds":6104,"fallback_count":0}
```

Population fields are computed from the same protected-view logic as the human table and tie out:

```text
raw=559 protected=552 cleanable=7
aggregated_shells=73 aggregated_rows=2
displayed=26 hidden_by_limit=25 hidden_by_size_filter=430
26 + 25 + 430 = 481 = 552 - 73 + 2
```

## Cleanup object

```text
$ ./swapaudit --json --dry-run --clean-all | python3 ...
True {'pruned': 0, 'ended': 0, 'skipped': 8, 'reasons': [{'reason': 'parent-alive', 'count': 8}]}
```

The dry-run emits `cleanup.dry_run = true` and mutates nothing.

## Guards

```text
$ ./swapaudit --json --clean-all
exit=2
error: --json live cleanup requires --dry-run or --yes
stdout bytes: 0

$ ./swapaudit --json upgrade
exit=2
error: --json is not supported for the upgrade command

$ printf '' | ./swapaudit --clean-all
exit=2
error: cleanup requires --dry-run or --yes when stdin is not a terminal
```

## Golden and determinism

```text
=== RUN   TestJSONGolden           --- PASS  (internal/report/testdata/audit-v1.golden.json)
=== RUN   TestJSONDeterministic    --- PASS  (byte-identical encodes; one newline)
=== RUN   TestJSONFullNames        --- PASS  (no ellipsis)
=== RUN   TestJSONNoANSI           --- PASS
=== RUN   TestJSONSchemaFields     --- PASS  (required keys and numeric types)
=== RUN   TestJSONReconciliation   --- PASS
=== RUN   TestJSONCleanup          --- PASS
=== RUN   TestJSONYesCleanup       --- PASS  (live cleanup object + exit 0)
```

The golden fixture covers both pools, unattributed reasons, a group, a zero-swap agent, an in-use cache, an orphaned cache, and a zero category row.

## Exit codes

```text
=== RUN   TestExitRuntimeOnAuditFailure --- PASS  (exit 1)
=== RUN   TestExitCleanupPartial        --- PASS  (exit 3)
=== RUN   TestJSONCleanupRequiresYesOrDryRun --- PASS  (exit 2)
=== RUN   TestJSONUpgradeRejected       --- PASS  (exit 2)
=== RUN   TestJSONAudit                 --- PASS  (exit 0)
=== RUN   TestDryRunFlagOrderIndependent --- PASS
```

The exit-code contract is `0` success, `1` runtime/report failure, `2` usage error, `3` cleanup partial failure.

## Documentation

`docs/json-schema.md` defines every top-level object, field unit, nullability rule, sorting rule, cleanup matrix, and exit code. `docs/metrics.md` defines the accounting fields reused by the schema. README documents `--json` and a `jq` pipeline.

## Verification gate

```text
$ gofmt -l .            # empty
$ go vet ./...          # pass
$ go test -race ./...   # pass
$ ./scripts/check.sh --full
✔ All checks passed.
```
