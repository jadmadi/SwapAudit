# JSON schema (version 1)

`swapaudit --json` writes exactly one JSON object plus a newline to stdout. Diagnostics go to stderr. `--json` implies no color and no human banner.

## Top-level object

```json
{
  "schema_version": 1,
  "generated_at": "2026-10-19T22:30:00Z",
  "host": {"kernel_release": "6.8.0", "uptime_seconds": 123456},
  "thresholds": {"max_age_seconds": 43200, "idle_threshold_seconds": 7200, "orphan_cache_age_seconds": 3600},
  "system": {
    "swap": {"total_bytes": 0, "used_bytes": 0, "free_bytes": 0},
    "ram": {"total_bytes": 0, "used_bytes": 0, "available_bytes": 0, "definition": "used = total - available"},
    "tmpfs": {"path": "/tmp", "total_bytes": 0, "used_bytes": 0, "free_bytes": 0, "avail_bytes": 0}
  },
  "attribution": {
    "swap": {"total_bytes": 0, "attributed_bytes": 0, "unattributed_bytes": 0, "overlap_bytes": 0, "reasons": []},
    "tmpfs": {"total_bytes": 0, "attributed_bytes": 0, "unattributed_bytes": 0, "overlap_bytes": 0, "reasons": []}
  },
  "reclaim": {"tmpfs_bytes": 0, "swap_bytes": 0, "upper_bound": true, "note": "tmpfs and swap pools may overlap; tmpfs pages may be swapped", "cache_count": 0, "process_count": 0},
  "population": {},
  "scan": {"scanned_caches": 0, "in_use_caches": 0, "orphaned_caches": 0, "recent_caches": 0, "evidence_incomplete": 0, "open_index_truncated": false, "open_index_errors": 0, "open_index_pids": 0, "open_index_fds": 0, "fallback_count": 0},
  "categories": [],
  "groups": [],
  "items": [],
  "cleanup": null
}
```

The golden fixture is `internal/report/testdata/audit-v1.golden.json`.

## Field rules

- `schema_version` is an integer starting at `1`. A breaking change bumps it.
- Byte fields are JSON integers (`int64`), never floats or strings.
- Duration fields are integer seconds. Unknown idle is `null` with `idle_known: false`.
- Timestamps are RFC3339 UTC.
- `reasons` is a sorted array, not a map.
- `categories` follow the fixed category order; `groups` sort by `total_bytes desc, key asc`; `items` sort by the active sort mode, then ID/path.
- Full names and paths are always present; no ellipsis or truncation.
- Consumers must ignore unknown fields.
- `cleanup` is `null` for an audit-only run.

## Population fields

`raw_items`, `aggregated_shells`, `aggregated_rows`, `displayed`, `hidden_by_limit`, `hidden_by_size_filter`, `cleanable`, `protected`, `processes`, `processes_with_swap`, `agents`, `agents_with_swap`, `agents_zero_swap`, `agents_dormant`, `agents_orphaned`, `agents_parent_alive`, `agents_idle_unknown`, `agents_active`, `vanished`, `permission_denied`.

`displayed + hidden_by_limit + hidden_by_size_filter` equals the number of protected rows after shell aggregation. `aggregated_shells` is the number of shell processes folded into `aggregated_rows` rows.

## Scan fields

`scanned_caches`, `in_use_caches`, `orphaned_caches`, `recent_caches`, `evidence_incomplete`, `open_index_truncated`, `open_index_errors`, `open_index_pids`, `open_index_fds`, `fallback_count`. These make the liveness evidence auditable without parsing human output.

## Item fields

`kind` (`process`/`cache`), `id`, `pid`, `start_ticks`, `uid`, `name`, `path`, `category`, `cache_kind`, `cleanable`, `agent`, `swap_bytes`, `allocated_bytes`, `apparent_bytes`, `rss_bytes`, `age_seconds`, `idle_seconds`, `idle_known`, `open_handles`, `cwd_inside`, `mapped_inside`, `parent_pid`, `parent_comm`, `parent_alive`, `orphan_reason`, `tty_nr`, `cgroup`, `evidence`, `signal`, `group_key`, `group_count`.

## Cleanup object

`cleanup` is present when cleanup flags are set:

```json
{
  "dry_run": true,
  "groups": [
    {"group": "Untouched Build Caches", "targets": 9, "bytes": 820000000,
     "would_prune": 9, "would_end": 0, "pruned": 0, "ended": 0,
     "skipped": 0, "reasons": []}
  ],
  "summary": {"pruned": 0, "ended": 0, "skipped": 0, "reasons": []}
}
```

- `--json --dry-run --clean-*` previews and sets `dry_run: true`.
- `--json --clean-*` without `--dry-run` requires `--yes`; otherwise exit `2` and emit no JSON.
- Live JSON cleanup emits the same object after the actions complete. There is no streaming event protocol in v1.
- Reason strings match the human summary (`open-handle`, `parent-alive`, `idle-unknown`, `remove-failed`, …).

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Audit succeeded; cleanup (if requested) completed with no operational failures. Skips and declines are not failures. |
| `1` | Runtime/report failure: required `/proc`/`statfs` read failed or JSON encoding failed. |
| `2` | Usage error: invalid flag or value, invalid combination, `--json upgrade`, `--json` live cleanup without `--yes`, non-TTY cleanup without `--dry-run`/`--yes`. |
| `3` | Cleanup partial failure: at least one `remove-failed` or `kill-failed`; the rest of the cleanup continued. |

## `--dry-run` / `--yes` matrix

| Flags | Cleanup flags | TTY | Behavior | Exit |
| --- | --- | --- | --- | --- |
| `--dry-run` | yes | any | Preview; no prompt; no mutation | 0 |
| `--dry-run` | no | any | Warning; normal audit | 0 |
| `--yes` | yes | any | Execute; no prompt | 0/3 |
| `--yes` | no | any | Warning; normal audit | 0 |
| neither | yes | yes | Prompt per group | 0 |
| neither | yes | no | Usage error | 2 |
| `--json --dry-run` | yes | any | JSON with `cleanup.dry_run=true` | 0 |
| `--json` + cleanup | yes | any | Requires `--yes` or `--dry-run` | 2 |
| `--json --yes` | yes | any | Execute and emit `cleanup` | 0/3 |

Flag order does not matter. `--json --dry-run` never reads stdin.

## Examples

```bash
swapaudit --json | jq -e '.schema_version == 1'
swapaudit --json | jq -e '.attribution.swap.attributed_bytes + .attribution.swap.unattributed_bytes == .system.swap.used_bytes'
swapaudit --json --dry-run --clean-all | jq '.cleanup.summary'
```
