# Decision 0001: Update-check policy

- **Decision**: A — opt-in update checks
- **Approver**: Project maintainer
- **Date**: 2026-10-19
- **Status**: Accepted

## Context

`GOVERNANCE.md` says the project does not phone home. The previous default run started a background request to `api.github.com` unless `--no-update-check`, `SWAPAUDIT_NO_UPDATE_CHECK`, or `CI` was set. The request was also canceled after 150 ms, so the "daily" check often did not complete.

The project is private and pre-v0.1.0. This is the right moment to make the default behavior match the stated privacy contract rather than shipping a contradiction.

## Decision

The default audit performs **no network I/O**.

A user who wants an update check opts in explicitly with one of:

```bash
swapaudit --check-updates
SWAPAUDIT_CHECK_UPDATES=1 swapaudit
```

`swapaudit upgrade` is separate: it always uses the network because it downloads and verifies a release binary.

## User-facing privacy sentence

> SwapAudit performs no network I/O by default. `swapaudit --check-updates` or `SWAPAUDIT_CHECK_UPDATES=1` performs one request to the GitHub release API. `swapaudit upgrade` downloads a release. SwapAudit does not collect telemetry or require an account.

This sentence is the canonical summary. `PRIVACY.md` contains the full details.

## Network details

| Item | Value |
| --- | --- |
| Primary endpoint | `https://api.github.com/repos/jadmadi/SwapAudit/releases/latest` |
| Fallback endpoint | `https://github.com/jadmadi/SwapAudit/releases/latest` (redirect check) |
| Method | `GET` |
| Headers | `User-Agent: swapaudit`; `Accept: application/vnd.github.v3+json` |
| Data sent | The HTTP request itself; GitHub may log the requesting IP |
| Data not sent | Local version, hostname, username, file contents, usage data, identifiers |
| Timeout | 2 seconds |
| State file | `$XDG_CACHE_HOME/swapaudit/update_check.json`, `~/.cache/swapaudit/update_check.json`, or the UID-scoped temp fallback |
| Opt-in | `--check-updates` or `SWAPAUDIT_CHECK_UPDATES=1` |
| Opt-out | Omit the flag/env; the default is already silent |

## Consequences

- Users no longer receive automatic update notices. Release notes and the README must make the explicit check discoverable.
- The 150 ms background race and the 24-hour automatic cadence are removed.
- `--no-update-check` and `SWAPAUDIT_NO_UPDATE_CHECK` are removed because v0.1.0 is the first public release and the default is silent.
- On a failed explicit check, the previous known version/URL is preserved and `LastChecked` is updated; the audit still exits `0`.
- The privacy sentry in `scripts/check.sh` rejects references to the removed flags and requires `PRIVACY.md`.

## Alternatives considered

- **Disclosed default check**: keep one automatic request, print a disclosure line, and retain `--no-update-check`. This is acceptable but weaker: it still performs network I/O the user did not request. It remains the fallback if opt-in causes a support problem.
- **No update check at all**: simplest, but users would have no in-binary way to learn about a release. The explicit flag preserves that path at zero default cost.
