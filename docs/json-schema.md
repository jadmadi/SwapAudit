# JSON snapshot schema (v1)

`swapaudit --json` writes exactly one machine-readable document to stdout and
nothing else. Diagnostics go to stderr. The mode is read-only: cleanup, dry-run,
confirmation, and interactive flags are rejected before any scan.

```bash
swapaudit --json > snapshot.json
swapaudit --json --no-command > snapshot.json   # omit command lines for logs/CI
```

## Versioning

`schema_version` is an integer and is `1` for this contract.

- Adding a new field is a minor change and keeps `schema_version: 1`.
- Removing a field, renaming a field, or changing a field's type or units
  requires `schema_version: 2`.

Consumers should compare the number, not parse a string, and should ignore
unknown fields.

## Top-level fields

| Field | Type | Null? | Units / rule |
| :--- | :--- | :---: | :--- |
| `schema_version` | integer | no | Always `1`. |
| `tool.name` | string | no | Always `swapaudit`. |
| `tool.version` | string | no | Build version, for example `dev`. |
| `tool.commit` | string | no | Build commit, for example `none`. |
| `tool.built` | string | no | Build date, for example `unknown`. |
| `generated_at` | string | no | RFC3339 with the local numeric offset, second precision. |
| `uptime_seconds` | number | no | First field of `/proc/uptime`; the only floating-point value. |
| `thresholds.max_age_seconds` | integer | no | Truncated to whole seconds. |
| `thresholds.idle_threshold_seconds` | integer | no | Truncated to whole seconds. |
| `system.swap_total_bytes` | integer | no | Bytes; never negative; `0` when unknown. |
| `system.swap_used_bytes` | integer | no | Bytes; never negative. |
| `system.swap_free_bytes` | integer | no | Bytes; never negative. |
| `system.mem_total_bytes` | integer | no | Bytes; never negative. |
| `system.mem_used_bytes` | integer | no | Bytes; never negative. |
| `system.mem_available_bytes` | integer | no | Bytes; never negative. |
| `system.tmpfs_total_bytes` | integer | no | Bytes; never negative. |
| `system.tmpfs_used_bytes` | integer | no | Bytes; never negative. |
| `totals.items` | integer | no | `len(items)`. |
| `totals.process_swap_bytes` | integer | no | Sum of all `items[].swap_bytes`. |
| `totals.tmpfs_cache_bytes` | integer | no | Sum of all `items[].size_bytes`. |
| `totals.cleanable_bytes` | integer | no | Sum of `effective_size_bytes` over cleanable items. |
| `totals.cleanable_process_swap_bytes` | integer | no | Cleanable `swap_bytes` only. |
| `totals.cleanable_tmpfs_bytes` | integer | no | Cleanable `size_bytes` only. |
| `summary` | array | no | One entry per category; sorted by `category` ascending; never `null`. |
| `items` | array | no | Every raw scan item; never truncated or aggregated; never `null`. |
| `warnings` | array of strings | no | Empty array in v1; reserved for scan diagnostics. |

## Summary fields

| Field | Type | Null? | Rule |
| :--- | :--- | :---: | :--- |
| `summary[].category` | string | no | Exact `model.Category` value. |
| `summary[].items` | integer | no | Sum of `SwapItem.Count` for the category. |
| `summary[].swap_bytes` | integer | no | Raw process swap in the category. |
| `summary[].size_bytes` | integer | no | Raw tmpfs cache size in the category. |
| `summary[].cleanable_items` | integer | no | Count of cleanable items. |
| `summary[].cleanable_swap_bytes` | integer | no | Cleanable process swap. |
| `summary[].cleanable_size_bytes` | integer | no | Cleanable tmpfs bytes. |

## Item fields and nullability

| Field | Type | Null? | Rule |
| :--- | :--- | :---: | :--- |
| `items[].pid` | integer | yes | `null` for filesystem caches. |
| `items[].start_ticks` | integer | yes | `null` for filesystem caches; process identity is `(pid, start_ticks)`. |
| `items[].name` | string | no | Display name derived by the scan. |
| `items[].command` | string | no | Command line or path; `""` with `--no-command`. |
| `items[].category` | string | no | Exact category value. |
| `items[].swap_bytes` | integer | no | Process swap; `0` for caches. |
| `items[].size_bytes` | integer | no | Cache size; `0` for processes. |
| `items[].effective_size_bytes` | integer | no | `size_bytes` when positive, otherwise `swap_bytes`. |
| `items[].age_seconds` | integer | yes | Whole seconds; `null` when a process has no start ticks or a cache has no mtime. `0` is a valid age. |
| `items[].idle_seconds` | integer | yes | Whole seconds; `null` when `/proc/<pid>/sched` is unreadable. Never `-1`. |
| `items[].last_active_at` | string | yes | RFC3339 with offset; `null` exactly when `idle_seconds` is `null`. |
| `items[].path` | string | yes | Absolute cache path; `null` for processes. |
| `items[].cleanable` | boolean | no | Whether the item is actionable. |

Two rules matter for consumers:

1. **`effective_size_bytes`** avoids branching on item kind:
   `size_bytes` when it is positive, otherwise `swap_bytes`.
2. **`idle_seconds: null`** means the kernel did not expose scheduling data.
   It is not zero and not `-1`. `last_active_at` is null in exactly the same
   cases.

Arrays are never `null`. An empty scan produces `"summary": []`,
`"items": []`, and `"warnings": []` with zero totals.

## Canonical ordering

`--sort`, `--limit`, and `--all` do not affect the document. Items are ordered
by:

1. `category` ascending (byte order),
2. `effective_size_bytes` descending,
3. `name` ascending,
4. `pid` ascending with `null` last.

This order is part of the contract and is covered by a golden test.

## Privacy: `--no-command`

Command lines can contain tokens, credentials, project names, or file paths that
should not be logged. `--no-command` keeps the `command` key but sets every value
to `""`. It is valid only with `--json`:

```bash
swapaudit --json --no-command > ci-snapshot.json
```

## Hard failures

If `/proc/self/status` cannot be read, the process exits `1` and emits exactly:

```json
{
  "schema_version": 1,
  "error": {
    "code": "proc_unavailable",
    "message": "cannot read /proc/self/status"
  }
}
```

Partial scans do not fail: unreadable individual processes are skipped, exactly
as in the human report. Exit codes are `0` success, `1` hard failure or encode
failure, and `2` flag conflict.

## Recipes

### Shell statusline

```bash
swapaudit --json --no-command --no-update-check |
  python3 -c 'import json,sys; s=json.load(sys.stdin); t=s["totals"]; print(f"{t[\"cleanable_bytes\"]/2**20:.0f} MiB cleanable / {t[\"items\"]} items")'
```

With `jq`:

```bash
swapaudit --json | jq -r '"\(.totals.cleanable_bytes / 1048576 | floor) MiB cleanable, \(.totals.items) items"'
```

### CI disk-usage check

Fail a job when more than 1 GiB is reclaimable:

```bash
set -e
swapaudit --json --no-command --no-update-check > snapshot.json
python3 - <<'PY'
import json
limit = 1024 ** 3
cleanable = json.load(open("snapshot.json"))["totals"]["cleanable_bytes"]
if cleanable > limit:
    raise SystemExit(f"cleanable bytes {cleanable} exceed {limit}")
PY
```

## Complete example

```json
{
  "schema_version": 1,
  "tool": {
    "name": "swapaudit",
    "version": "dev",
    "commit": "none",
    "built": "unknown"
  },
  "generated_at": "2026-10-21T09:15:04+03:00",
  "uptime_seconds": 123456.78,
  "thresholds": {
    "max_age_seconds": 43200,
    "idle_threshold_seconds": 7200
  },
  "system": {
    "swap_total_bytes": 18321752064,
    "swap_used_bytes": 8971677696,
    "swap_free_bytes": 9350074368,
    "mem_total_bytes": 33549176832,
    "mem_used_bytes": 15874695168,
    "mem_available_bytes": 17674481664,
    "tmpfs_total_bytes": 16777216000,
    "tmpfs_used_bytes": 7838310400
  },
  "totals": {
    "items": 2,
    "process_swap_bytes": 94057267,
    "tmpfs_cache_bytes": 3145728,
    "cleanable_bytes": 97202995,
    "cleanable_process_swap_bytes": 94057267,
    "cleanable_tmpfs_bytes": 3145728
  },
  "summary": [
    {
      "category": "dormant-agents",
      "items": 1,
      "swap_bytes": 94057267,
      "size_bytes": 0,
      "cleanable_items": 1,
      "cleanable_swap_bytes": 94057267,
      "cleanable_size_bytes": 0
    },
    {
      "category": "untouched-caches",
      "items": 1,
      "swap_bytes": 0,
      "size_bytes": 3145728,
      "cleanable_items": 1,
      "cleanable_swap_bytes": 0,
      "cleanable_size_bytes": 3145728
    }
  ],
  "items": [
    {
      "pid": 1618347,
      "start_ticks": 123456789,
      "name": "chrome-devtools",
      "command": "chrome-devtools-mcp --port 9222",
      "category": "dormant-agents",
      "swap_bytes": 94057267,
      "size_bytes": 0,
      "effective_size_bytes": 94057267,
      "age_seconds": 104880,
      "idle_seconds": 43980,
      "last_active_at": "2026-10-20T21:02:04+03:00",
      "path": null,
      "cleanable": true
    },
    {
      "pid": null,
      "start_ticks": null,
      "name": "go-build123",
      "command": "/tmp/go-build123",
      "category": "untouched-caches",
      "swap_bytes": 0,
      "size_bytes": 3145728,
      "effective_size_bytes": 3145728,
      "age_seconds": 172800,
      "idle_seconds": 46800,
      "last_active_at": "2026-10-20T20:15:04+03:00",
      "path": "/tmp/go-build123",
      "cleanable": true
    }
  ],
  "warnings": []
}
```
