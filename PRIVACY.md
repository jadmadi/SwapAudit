# Privacy

SwapAudit is a local Linux CLI. It reads `/proc`, `/tmp`, and `/var/tmp` metadata to report memory and cache usage. It does not require an account, does not collect telemetry, and does not upload usage data.

## Default behavior

The default audit performs **no network I/O**.

```bash
swapaudit
```

## Explicit update check

If you ask for an update check, SwapAudit performs one request to GitHub:

```bash
swapaudit --check-updates
# or
SWAPAUDIT_CHECK_UPDATES=1 swapaudit
```

| Item | Detail |
| --- | --- |
| Primary endpoint | `https://api.github.com/repos/jadmadi/SwapAudit/releases/latest` |
| Fallback endpoint | `https://github.com/jadmadi/SwapAudit/releases/latest` (redirect check) |
| Method | `GET` |
| Headers | `User-Agent: swapaudit`, `Accept: application/vnd.github.v3+json` |
| Timeout | 2 seconds |
| Data sent | The HTTP request; GitHub may log the requesting IP as part of normal HTTP handling |
| Data **not** sent | Local version, hostname, username, file contents, usage data, identifiers |
| Opt-in | `--check-updates` or `SWAPAUDIT_CHECK_UPDATES=1` |
| Opt-out | Omit the flag and env; the default is silent |

No telemetry, analytics, crash reporting, accounts, or advertising SDKs are included.

## State file

An explicit check writes a small state file so the tool can remember the last check:

- `$XDG_CACHE_HOME/swapaudit/update_check.json`
- `~/.cache/swapaudit/update_check.json`
- If neither is available: `$TMPDIR/swapaudit-<uid>-update-check.json`

It contains:

```json
{
  "last_checked": "2026-10-19T00:00:00Z",
  "latest_version": "v0.1.0",
  "release_url": "https://github.com/jadmadi/SwapAudit/releases/tag/v0.1.0"
}
```

It does not contain local paths, usernames, or usage history. On a failed check, the previous version and URL are preserved and `last_checked` is updated.

## Upgrade

`swapaudit upgrade` is a separate, explicit action. It downloads release metadata, a binary, and a checksum file from GitHub, verifies the checksum, and replaces the executable. It uses the same GitHub origin and is not part of the audit path.

## Contact

For privacy or security questions, see [`SECURITY.md`](SECURITY.md) and [`SUPPORT.md`](SUPPORT.md) once the support policy is published. The security-contact consolidation goal tracks the canonical channel.
