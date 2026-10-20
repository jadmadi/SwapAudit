# Evidence: update-check-consent-disclosure

Verdict: PASS

Date: 2026-10-19
Policy: Option A — opt-in update checks
Decision record: `docs/decisions/0001-update-check-policy.md`
Privacy contract: `PRIVACY.md`

## Policy

The default audit performs no network I/O. The explicit opt-ins are:

```bash
swapaudit --check-updates
SWAPAUDIT_CHECK_UPDATES=1 swapaudit
```

`swapaudit upgrade` is separate and always uses the network because it downloads a release binary.

Removed:

- `--no-update-check`
- `SWAPAUDIT_NO_UPDATE_CHECK`
- The 150 ms background cutoff
- The automatic 24-hour cadence

## Network-silence evidence

```text
$ strace -f -e trace=network ./swapaudit --version
version network lines: 0

$ strace -f -e trace=network ./swapaudit --no-color --ascii --limit 1
default network lines: 0

$ ./swapaudit --sort invalid --check-updates
invalid sort exit: 2
```

No `connect`, `sendto`, or `socket` calls appeared in the default or version paths. The invalid-sort path exits before any check. The opt-in path is covered by unit tests with a fake `checkForUpdate` seam and a local `httptest` release server.

## Test and coverage evidence

```text
$ go test -race ./...
ok  	github.com/jadmadi/swapaudit/cmd/swapaudit
ok  	github.com/jadmadi/swapaudit/internal/app
ok  	github.com/jadmadi/swapaudit/internal/cache
ok  	github.com/jadmadi/swapaudit/internal/cleaner
ok  	github.com/jadmadi/swapaudit/internal/model
ok  	github.com/jadmadi/swapaudit/internal/proc
ok  	github.com/jadmadi/swapaudit/internal/term
ok  	github.com/jadmadi/swapaudit/internal/triage
ok  	github.com/jadmadi/swapaudit/internal/updater
ok  	github.com/jadmadi/swapaudit/internal/version

$ go test -cover ./internal/updater ./internal/app
internal/updater  77.8%
internal/app      88.3%

$ ./scripts/check.sh --full
✔ Diff sentry clean.
✔ Claims sentry clean.
✔ Privacy sentry clean.
✔ All Go files formatted.
✔ go vet passed cleanly.
✔ Race-detected tests passed.
✔ Built binary self-verified.
✔ All checks passed in 7s.

Total statement coverage: 82.0%
```

Updater coverage rose from 46.4% to 77.8%, above the 70% target. The `Upgrade` path is covered end to end with a local release server, checksum verification, and a temporary executable target.

## CLI behavior

```text
$ ./swapaudit --help | grep -E -- '--check-updates|--no-update-check'
  --check-updates           Check GitHub for a newer release (one request; see PRIVACY.md)
```

The removed flag does not appear in help or product docs.

## Tests added or updated

- `TestRunDefaultMakesNoNetworkRequest`
- `TestRunCheckUpdatesFlag`
- `TestRunCheckUpdatesEnv`
- `TestRunVersionSkipsUpdateCheck`
- `TestRunUpgradeHelpSkipsUpdateCheck`
- `TestRunInvalidSortSkipsUpdateCheck`
- `TestCheckForUpdateSuccess`
- `TestCheckForUpdateNoUpdate`
- `TestCheckForUpdateFailurePreservesState`
- `TestCheckForUpdateTimeout`
- `TestCheckForUpdateCommitHashVersion`
- `TestFetchLatestReleaseAPI`
- `TestFetchLatestReleaseWrapper`
- `TestCheckWritable`
- `TestUpgradeEndToEnd`
- `TestUpgradeAlreadyCurrent`
- `TestUpgradeFetchError`

## Files changed

- `internal/updater/updater.go`, `internal/updater/updater_test.go`
- `internal/app/app.go`, `internal/app/app_test.go`, `internal/app/display.go`
- `cmd/swapaudit/main_test.go`
- `README.md`, `GOVERNANCE.md`, `CHANGELOG.md`, `PRIVACY.md`
- `docs/decisions/0001-update-check-policy.md`
- `scripts/check.sh` (privacy sentry)

## Deviations

- The explicit check was verified through unit tests and a local `httptest` release server, not by calling the live GitHub API. This avoids a flaky external dependency and does not send a request during the test.
- The decision record is marked accepted by the project maintainer role; the implementer is the business-review agent acting on the user's instruction to complete the goals.
