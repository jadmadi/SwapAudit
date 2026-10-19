# Plan: Make the Update Check Consent-Based and Truthful

## Execution Steps

### Phase 1 — Record the policy decision

- [ ] Write `docs/decisions/0001-update-check-policy.md`; choose A (opt-in, preferred) or B (disclosed default); record approver, date, rationale, and the exact privacy sentence.
- [ ] Write `PRIVACY.md` with the endpoint, method, headers, data sent, data not sent, state-file path, and opt-in/opt-out mechanism.
- [ ] Link `PRIVACY.md` from `README.md` and `GOVERNANCE.md`.
- [ ] If Option A, rewrite `GOVERNANCE.md:36` to say the default performs no network I/O and an explicit opt-in contacts GitHub.
- [ ] If Option B, remove the unqualified "does not phone home" claim and describe the disclosed check.

### Phase 2 — Refactor the updater API

- [ ] Add `CheckForUpdate(ctx context.Context, currentVersion string) (*UpdateInfo, error)`.
- [ ] Remove or rename `CheckDailyUpdate`/`CheckDailyUpdateWithContext` so no API promises a cadence.
- [ ] Delete `GetCachedUpdate` and `CheckInterval` if they become unused; do not leave dead code.
- [ ] Keep `FetchLatestReleaseContext`, `IsNewer`, `CompareVersions`, `LoadState`, and `SaveState`.
- [ ] Preserve previous `LatestVersion`/`ReleaseURL` on failure; write `LastChecked` according to the policy.
- [ ] Add tests for success, 404, 500, malformed JSON, empty tag, timeout, read-only cache, and commit-hash current version.
- [ ] Run `go test -race ./internal/updater`.

### Phase 3 — Refactor the app update path

- [ ] Add `var checkForUpdate = updater.CheckForUpdate` in `internal/app/app.go`.
- [ ] Remove the `go func()` + `time.After(150 * time.Millisecond)` block.
- [ ] Add `--check-updates` (default false) and `SWAPAUDIT_CHECK_UPDATES=1` opt-in (Option A).
- [ ] Or add the disclosure line and synchronous bounded check on the default path (Option B).
- [ ] Ensure `--version`, `--help`, and `upgrade --help` exit before any update check.
- [ ] Ensure `--sort invalid --check-updates` exits `2` without a network call.
- [ ] Ensure `upgrade` takes precedence over `--check-updates`.
- [ ] Remove `--no-update-check` and `SWAPAUDIT_NO_UPDATE_CHECK` under Option A; update all call sites and tests. Under Option B, keep both and test them.
- [ ] Print the notice exactly once; do not duplicate cached and live notices.
- [ ] Bound the check to 2 s; on timeout print `warning: update check timed out after 2s` to stderr and exit `0`.
- [ ] Run `go test -race ./internal/app`.

### Phase 4 — Add tests and close the coverage gap

- [ ] `TestRunDefaultMakesNoNetworkRequest` with a `checkForUpdate` that fails the test if called.
- [ ] `TestRunCheckUpdatesFlag` with a fake notice.
- [ ] `TestRunCheckUpdatesEnv` with `SWAPAUDIT_CHECK_UPDATES=1`.
- [ ] `TestRunVersionSkipsUpdateCheck`.
- [ ] `TestRunUpgradeHelpSkipsUpdateCheck`.
- [ ] `TestRunInvalidSortSkipsUpdateCheck`.
- [ ] `TestCheckForUpdateTimeout` with a slow `httptest` server.
- [ ] `TestCheckForUpdateSuccessWritesState`.
- [ ] `TestCheckForUpdateFailurePreservesState`.
- [ ] `TestCheckForUpdateCommitHashVersion`.
- [ ] Raise `internal/updater` coverage to at least 70%; do not lower `internal/app` coverage.
- [ ] Run `go test -race -cover ./internal/updater ./internal/app`.

### Phase 5 — Add the claims sentry

- [ ] Extend `scripts/check.sh` (or add `scripts/check-privacy-claims.sh`) to enforce the chosen policy.
- [ ] Option A: allow "does not phone home by default" only with a `PRIVACY.md` link; fail on an unqualified absolute claim.
- [ ] Option B: fail if "does not phone home" appears anywhere; require the disclosure sentence in `README.md` and `PRIVACY.md`.
- [ ] Ensure the sentry does not fail on the governance anti-goal word "telemetry" alone.
- [ ] Wire the sentry into `scripts/check.sh --full`.

### Phase 6 — Documentation and help

- [ ] Update `README.md` with an "Update checks and privacy" section.
- [ ] Update `internal/app/display.go` help text with `--check-updates` and the endpoint.
- [ ] Update `GOVERNANCE.md` anti-goal #1.
- [ ] Update `CHANGELOG.md` under `Changed` and `Removed`.
- [ ] Ensure `PRIVACY.md`, README, GOVERNANCE, and the decision record use the same sentence.

### Phase 7 — Verification and evidence

- [ ] `gofmt -l .` returns empty.
- [ ] `go vet ./...` passes.
- [ ] `go test -race ./...` passes.
- [ ] `go test -cover ./internal/updater ./internal/app` meets the coverage targets.
- [ ] `./scripts/check.sh --full` passes.
- [ ] `strace -f -e trace=network ./swapaudit --version` shows no network syscalls.
- [ ] `strace -f -e trace=network ./swapaudit --no-color --ascii --limit 1` shows no network syscalls under Option A.
- [ ] `./swapaudit --check-updates --no-color --ascii --limit 1` performs exactly one check and prints one notice.
- [ ] `./swapaudit --sort invalid --check-updates` exits `2` with no network syscalls.
- [ ] Record all outputs in `goals/update-check-consent-disclosure/evidence.md`.

### Phase 8 — Attribution commit

- [ ] Commit with `(goals/update-check-consent-disclosure/goal.md)` or `Goal: update-check-consent-disclosure`, using `fix:` or `feat:`.
- [ ] Run `sila goals --project=/home/jad/scripts/active/swapaudit`.
- [ ] Do not push unless explicitly authorized.

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
