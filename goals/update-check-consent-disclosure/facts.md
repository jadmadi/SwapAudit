# Facts: Make the Update Check Consent-Based and Truthful

## Architectural Invariants & Constraints

- The default audit must be predictable: no hidden network I/O under Option A; no hidden check under Option B.
- No update check may be canceled by an arbitrary UI delay. Use a bounded context and wait for the result.
- An update-check failure never fails the audit.
- `--version`, `--help`, and `upgrade --help` never perform network I/O.
- No telemetry, accounts, or background daemon.
- No new external dependencies.
- The chosen policy is recorded in `docs/decisions/0001-update-check-policy.md`; README, GOVERNANCE, and PRIVACY must agree with it.
- Tests must never hit the live GitHub API. Inject the checker or use `httptest`.

## Observed Facts (Repository)

| Fact | Evidence | Consequence |
| --- | --- | --- |
| Governance claims "does not phone home" | `GOVERNANCE.md:36` | Default network I/O contradicts the stated contract |
| Default run starts an update check unless `--no-update-check`, `SWAPAUDIT_NO_UPDATE_CHECK`, or `CI` is set | `internal/app/app.go:186-204` | The first audit can contact GitHub without explicit consent |
| The check runs in a goroutine with a 2 s context | `internal/app/app.go:189-192` | The code intends a bounded background check |
| The main path cancels after 150 ms | `internal/app/app.go:252` | A 2 s request is usually canceled before completion; "daily background check" is unreliable |
| A cached notice is printed synchronously | `internal/app/app.go:208-211`, `:248-250` | Users may see a stale notice or no notice |
| The updater has a 24 h interval constant | `internal/updater/updater.go:27-28` | The cadence is enforced by the updater, but the app's 150 ms cutoff undermines it |
| The default repo is `jadmadi/swapaudit` | `internal/updater/updater.go:24-25` | The endpoint is public; there is no private service |
| The updater sends `User-Agent: swapaudit` and GitHub's v3 Accept header | `internal/updater/updater.go:250-255` | The request data is minimal but should be disclosed |
| The updater falls back to a GitHub redirect check | `internal/updater/updater.go:274-300` | A second endpoint can be contacted if the API fails |
| State is written atomically | `internal/updater/updater.go:79-105` | The state file is safe to update; failure behavior still needs tests |
| The update-check tests cover cached notices and context cancellation but not the 150 ms app path | `internal/app/app_test.go`, `internal/updater/updater_test.go` | The actual user-visible race is untested |
| Updater package coverage is 46.4% | `go test -cover ./...` on 2026-10-19 | The network/update path is under-tested |
| `strace` is available on this host | `command -v strace` → `/usr/bin/strace` | The default-run network test can be executed and recorded |

## External Facts

- GitHub REST API `GET /repos/{owner}/{repo}/releases/latest` returns the latest non-draft, non-prerelease release. It requires no authentication for public repositories, subject to rate limits.
- A `User-Agent` header is required by the GitHub API. The current code sends `swapaudit`.
- The API response includes `tag_name` and `html_url`; the current code decodes only those fields.
- The fallback `GET https://github.com/{owner}/{repo}/releases/latest` returns a redirect whose `Location` contains `/tag/<tag>`.
- GitHub may log the requesting IP address as part of normal HTTP handling. This is the privacy-relevant data.
- A 2 s context is a bounded network timeout, not a guarantee that the remote server has stopped processing the request.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| Users prefer a silent default over an automatic check | The governance contract and privacy posture imply it | Ask in the 5 validation interviews; note the answer in `public-install-demand-validation` |
| An explicit `--check-updates` is enough for users who want update notices | Avoids a background service and respects consent | Release notes and README; observe whether users run it |
| The GitHub API is acceptable for update metadata | It is the only release host in the project | Document the endpoint; provide `SWAPAUDIT_REPO` for forks |
| A 2 s synchronous check is tolerable when explicitly requested | The default path stays fast | Measure `--check-updates` on a slow network; document the worst case |
| The state file is not sensitive | It contains a timestamp, version, and URL | Confirm no local paths, usernames, or tokens are written |

## Unknowns

- Whether the maintainer prefers opt-in or disclosure. **Decision required.** The goal defaults to opt-in if no decision is recorded.
- Whether GitHub release download counts or API rate limits will be a problem at the expected adoption level. **Unknown.** The project has no releases yet.
- Whether users expect automatic update notices from a CLI. **Unknown.** The validation interviews should ask.
- Whether a future package manager (AUR/Homebrew/Nix) should own update notifications instead of the binary. **Unknown.** Out of scope for this goal.

## Current Behavior Trace

```text
app.Run
  ├─ parse flags
  ├─ if --version: print and exit (no network)
  ├─ if update check enabled:
  │    ├─ read cached state
  │    ├─ if fresh and newer: cachedNotice
  │    └─ else: start goroutine with 2s context
  ├─ run audit and print tables
  ├─ execute cleanup if requested
  └─ wait for goroutine:
       ├─ done within 150ms: print notice
       └─ else: cancel context, wait, print nothing
```

The goal changes the last branch to either no default goroutine (Option A) or a synchronous, disclosed, bounded check (Option B).

## Evidence Commands

```bash
grep -n "no-update-check\|SWAPAUDIT_NO_UPDATE_CHECK\|CI\|CheckDailyUpdate\|GetCachedUpdate\|time.After" internal/app/app.go
grep -n "CheckInterval\|DefaultRepo\|User-Agent\|api.github.com\|releases/latest" internal/updater/updater.go
grep -n "does not phone home\|offline" GOVERNANCE.md README.md
go test -cover ./internal/updater ./internal/app
strace -f -e trace=network ./swapaudit --version 2>&1 | tail -20
strace -f -e trace=network ./swapaudit --no-color --ascii --limit 1 2>&1 | tail -20
```

## File & Interface Contracts

- `internal/app`: add `var checkForUpdate = updater.CheckForUpdate`; add `--check-updates`; remove the background goroutine and 150 ms cutoff; add env opt-in.
- `internal/updater`: add `CheckForUpdate(ctx, currentVersion)`; remove or rename `CheckDailyUpdate*`; keep `FetchLatestReleaseContext`, `IsNewer`, `CompareVersions`, `LoadState`, `SaveState`.
- `PRIVACY.md`: single source of truth for network behavior.
- `docs/decisions/0001-update-check-policy.md`: recorded decision.
- `scripts/check.sh`: privacy/claims sentry.

## Test Seams

- App-level `checkForUpdate` variable so the default path can be tested without network.
- `httptest.Server` for updater tests; never the live GitHub API.
- `t.Setenv("SWAPAUDIT_CHECK_UPDATES", "1")`, `t.Setenv("XDG_CACHE_HOME", t.TempDir())`, `t.Setenv("CI", "")`.
- A fake `http.Client` only if the updater API accepts one; otherwise use `SWAPAUDIT_REPO` with a test server only if the URL is injectable. Prefer the app-level seam for default-path tests and `httptest` for updater unit tests.
