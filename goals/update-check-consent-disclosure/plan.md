# Plan: Make the Update Check Consent-Based and Truthful

## Execution Steps

### Phase 1 — Record the policy decision

- [x] Write `docs/decisions/0001-update-check-policy.md`; choose A (opt-in, preferred) or B (disclosed default); record approver, date, rationale, and the exact privacy sentence.
- [x] Write `PRIVACY.md` with the endpoint, method, headers, data sent, data not sent, state-file path, and opt-in/opt-out mechanism.
- [x] Link `PRIVACY.md` from `README.md` and `GOVERNANCE.md`.
- [x] If Option A, rewrite `GOVERNANCE.md:36` to say the default performs no network I/O and an explicit opt-in contacts GitHub.
- [x] If Option B, remove the unqualified "does not phone home" claim and describe the disclosed check.

### Phase 2 — Refactor the updater API

- [x] Add `CheckForUpdate(ctx context.Context, currentVersion string) (*UpdateInfo, error)`.
- [x] Remove or rename `CheckDailyUpdate`/`CheckDailyUpdateWithContext` so no API promises a cadence.
- [x] Delete `GetCachedUpdate` and `CheckInterval` if they become unused; do not leave dead code.
- [x] Keep `FetchLatestReleaseContext`, `IsNewer`, `CompareVersions`, `LoadState`, and `SaveState`.
- [x] Preserve previous `LatestVersion`/`ReleaseURL` on failure; write `LastChecked` according to the policy.
- [x] Add tests for success, 404, 500, malformed JSON, empty tag, timeout, read-only cache, and commit-hash current version.
- [x] Run `go test -race ./internal/updater`.

### Phase 3 — Refactor the app update path

- [x] Add `var checkForUpdate = updater.CheckForUpdate` in `internal/app/app.go`.
- [x] Remove the `go func()` + `time.After(150 * time.Millisecond)` block.
- [x] Add `--check-updates` (default false) and `SWAPAUDIT_CHECK_UPDATES=1` opt-in (Option A).
- [x] Or add the disclosure line and synchronous bounded check on the default path (Option B).
- [x] Ensure `--version`, `--help`, and `upgrade --help` exit before any update check.
- [x] Ensure `--sort invalid --check-updates` exits `2` without a network call.
- [x] Ensure `upgrade` takes precedence over `--check-updates`.
- [x] Remove `--no-update-check` and `SWAPAUDIT_NO_UPDATE_CHECK` under Option A; update all call sites and tests. Under Option B, keep both and test them.
- [x] Print the notice exactly once; do not duplicate cached and live notices.
- [x] Bound the check to 2 s; on timeout print `warning: update check timed out after 2s` to stderr and exit `0`.
- [x] Run `go test -race ./internal/app`.

### Phase 4 — Add tests and close the coverage gap

- [x] `TestRunDefaultMakesNoNetworkRequest` with a `checkForUpdate` that fails the test if called.
- [x] `TestRunCheckUpdatesFlag` with a fake notice.
- [x] `TestRunCheckUpdatesEnv` with `SWAPAUDIT_CHECK_UPDATES=1`.
- [x] `TestRunVersionSkipsUpdateCheck`.
- [x] `TestRunUpgradeHelpSkipsUpdateCheck`.
- [x] `TestRunInvalidSortSkipsUpdateCheck`.
- [x] `TestCheckForUpdateTimeout` with a slow `httptest` server.
- [x] `TestCheckForUpdateSuccessWritesState`.
- [x] `TestCheckForUpdateFailurePreservesState`.
- [x] `TestCheckForUpdateCommitHashVersion`.
- [x] Raise `internal/updater` coverage to at least 70%; do not lower `internal/app` coverage.
- [x] Run `go test -race -cover ./internal/updater ./internal/app`.

### Phase 5 — Add the claims sentry

- [x] Extend `scripts/check.sh` (or add `scripts/check-privacy-claims.sh`) to enforce the chosen policy.
- [x] Option A: allow "does not phone home by default" only with a `PRIVACY.md` link; fail on an unqualified absolute claim.
- [x] Option B: fail if "does not phone home" appears anywhere; require the disclosure sentence in `README.md` and `PRIVACY.md`.
- [x] Ensure the sentry does not fail on the governance anti-goal word "telemetry" alone.
- [x] Wire the sentry into `scripts/check.sh --full`.

### Phase 6 — Documentation and help

- [x] Update `README.md` with an "Update checks and privacy" section.
- [x] Update `internal/app/display.go` help text with `--check-updates` and the endpoint.
- [x] Update `GOVERNANCE.md` anti-goal #1.
- [x] Update `CHANGELOG.md` under `Changed` and `Removed`.
- [x] Ensure `PRIVACY.md`, README, GOVERNANCE, and the decision record use the same sentence.

### Phase 7 — Verification and evidence

- [x] `gofmt -l .` returns empty.
- [x] `go vet ./...` passes.
- [x] `go test -race ./...` passes.
- [x] `go test -cover ./internal/updater ./internal/app` meets the coverage targets.
- [x] `./scripts/check.sh --full` passes.
- [x] `strace -f -e trace=network ./swapaudit --version` shows no network syscalls.
- [x] `strace -f -e trace=network ./swapaudit --no-color --ascii --limit 1` shows no network syscalls under Option A.
- [x] `./swapaudit --check-updates --no-color --ascii --limit 1` performs exactly one check and prints one notice.
- [x] `./swapaudit --sort invalid --check-updates` exits `2` with no network syscalls.
- [x] Record all outputs in `goals/update-check-consent-disclosure/evidence.md`.

### Phase 8 — Attribution commit

- [x] Commit with `(goals/update-check-consent-disclosure/goal.md)` or `Goal: update-check-consent-disclosure`, using `fix:` or `feat:`.
- [x] Run `sila goals --project=/home/jad/scripts/active/swapaudit`.
- [x] Do not push unless explicitly authorized.

## Tests to Add

| Test | File | What it proves |
| --- | --- | --- |
| `TestRunDefaultMakesNoNetworkRequest` | `internal/app/app_test.go` | Option A default is silent |
| `TestRunCheckUpdatesFlag` | `internal/app/app_test.go` | Explicit flag performs one check |
| `TestRunCheckUpdatesEnv` | `internal/app/app_test.go` | Env opt-in works |
| `TestRunVersionSkipsUpdateCheck` | `internal/app/app_test.go` | Version path is network-silent |
| `TestRunUpgradeHelpSkipsUpdateCheck` | `internal/app/app_test.go` | Upgrade help is network-silent |
| `TestRunInvalidSortSkipsUpdateCheck` | `internal/app/app_test.go` | Usage errors win over update checks |
| `TestCheckForUpdateTimeout` | `internal/updater/updater_test.go` | A slow server cannot hang the process |
| `TestCheckForUpdateSuccessWritesState` | `internal/updater/updater_test.go` | State fields are written atomically |
| `TestCheckForUpdateFailurePreservesState` | `internal/updater/updater_test.go` | Failure does not erase known version/URL |
| `TestCheckForUpdateCommitHashVersion` | `internal/updater/updater_test.go` | Commit-hash builds compare intentionally |
| Privacy sentry | `scripts/check.sh` | Docs cannot drift from the chosen policy |

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -cover ./internal/updater ./internal/app
./scripts/check.sh --full
strace -f -e trace=network ./swapaudit --version 2>&1 | grep -E 'connect|sendto|socket' || true
strace -f -e trace=network ./swapaudit --no-color --ascii --limit 1 2>&1 | grep -E 'connect|sendto|socket' || true
./swapaudit --check-updates --no-color --ascii --limit 1
./swapaudit --sort invalid --check-updates >/dev/null 2>&1; echo $?
```

## Rollback

- Revert the implementation commit.
- If Option A causes support confusion, add a one-time disclosed check in a follow-up; do not silently restore background I/O.
- If Option B is chosen and users object, switch to Option A in a follow-up; do not keep both default behaviors.
- Preserve the state-file format during rollback so a downgrade does not corrupt it.

## Completion Evidence

Before certifying, `goals/update-check-consent-disclosure/evidence.md` must contain:

1. The policy decision record path and the chosen option.
2. `strace` output for `--version`, default run, and `--check-updates`.
3. The `--sort invalid --check-updates` exit code and network trace.
4. `go test -race`, `go vet`, coverage, and `scripts/check.sh --full` outputs.
5. The exact README/GOVERNANCE/PRIVACY sentences and confirmation that they match.
6. Any deviation from this plan and why.
