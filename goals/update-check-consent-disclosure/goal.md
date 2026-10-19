# Goal: Make the Update Check Consent-Based and Truthful

## Chain of Custody (Isnad)

- **Registered By**: Business review agent, 2026-10-19
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: Pending Mystery Audit

## Goal Description

Resolve the contradiction between `GOVERNANCE.md` ("does not phone home") and the default runtime behavior (a background request to `api.github.com`). The preferred policy is **opt-in**: the default audit performs no network I/O, and a user who wants an update check passes `--check-updates` or sets `SWAPAUDIT_CHECK_UPDATES=1`. If the maintainer rejects opt-in, the goal permits a **disclosure** policy, but only with an explicit pre-request notice, accurate documentation, and a fix for the 150 ms cancellation bug. The chosen policy must be recorded before implementation.

The goal also fixes the update-check lifecycle: no background goroutine that the process kills after 150 ms, no misleading "daily" cadence, no state corruption on failure, and no network call when the user runs `--version`, `--help`, or `upgrade --help`.

## Business Context

- The business review (`docs/business-reviews/2026-10-19-swapaudit-full-review.md`) ranks the privacy contradiction and unreliable update check as **High** trust gaps (G-07).
- The governance anti-goal says swapaudit "does not phone home." A default network call violates the stated contract and can surprise privacy-sensitive developers.
- The current 150 ms cutoff (`internal/app/app.go:252`) cancels a 2 s request, so the "daily background check" often never completes. Users either miss updates or see a stale cached notice.
- A truthful update policy is a prerequisite for the public v0.1.0 release. This goal is a prerequisite for `standalone-v0-1-0-release`.

## Dependencies & Execution Order

- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: 2
- **Shape**: ship
- **Tier**: immediate

## Files to Touch

- `internal/app/app.go`
- `internal/app/app_test.go`
- `internal/app/display.go`
- `internal/updater/updater.go`
- `internal/updater/updater_test.go`
- `README.md`
- `GOVERNANCE.md`
- `PRIVACY.md` (new)
- `CHANGELOG.md`
- `docs/decisions/0001-update-check-policy.md` (new)
- `scripts/check.sh` (claims sentry integration)
- `goals/update-check-consent-disclosure/*` (this package)

## Policy Decision Gate

Before changing code, write `docs/decisions/0001-update-check-policy.md` with:

- **Decision**: `A — opt-in` (preferred) or `B — disclosed default`.
- **Approver**, **date**, and a one-paragraph rationale.
- The exact user-facing sentence that appears in `PRIVACY.md`.
- The endpoint, HTTP method, headers, and data sent.
- The cache/state behavior.
- The opt-out or opt-in mechanism.

If no decision is recorded, the implementer must choose **A — opt-in** and record it. The rest of this goal describes both paths; the chosen path must satisfy its own acceptance criteria and the shared criteria.

## Requirements

### R1 — Record the policy and privacy contract

- Add `PRIVACY.md` with:
  - The exact default behavior (Option A: no network; Option B: one check per invocation with disclosure).
  - The endpoint (`https://api.github.com/repos/jadmadi/swapaudit/releases/latest`).
  - The fallback endpoint (`https://github.com/jadmadi/swapaudit/releases/latest` redirect check) if it remains.
  - Headers sent: `User-Agent: swapaudit`, `Accept: application/vnd.github.v3+json`.
  - Data **not** sent: local version, hostname, username, usage data, identifiers, file contents.
  - The state file path (`$XDG_CACHE_HOME/swapaudit/update_check.json`, `~/.cache/swapaudit/update_check.json`, or the UID-scoped temp fallback).
  - The exact opt-in/opt-out flag and environment variable.
- Link `PRIVACY.md` from `README.md` and `GOVERNANCE.md`.
- Replace the unqualified "does not phone home" sentence with a precise statement that matches the chosen policy. If Option B is chosen, remove "does not phone home" and describe the disclosure.

### R2 — Remove the 150 ms background cutoff

- Delete the `go func()` + `time.After(150 * time.Millisecond)` pattern in `internal/app/app.go`.
- No update check may outlive the audit process or be canceled after an arbitrary UI delay.
- If the chosen policy performs a check, it runs synchronously with a bounded context:
  - Option A: only when `--check-updates` or `SWAPAUDIT_CHECK_UPDATES=1` is set.
  - Option B: on the default path, after the audit output, with a disclosure line before the request.
- Use `context.WithTimeout(context.Background(), 2*time.Second)` and `defer cancel()`.
- A timeout prints `warning: update check timed out after 2s` to stderr and does not change the audit exit code.
- No goroutine leak; `go test -race ./...` and `go vet ./...` must pass.

### R3 — Option A (preferred): opt-in only

- Default `swapaudit` performs **zero network I/O**.
- Add `--check-updates` (bool, default false):
  `Check GitHub for a newer release and print a notice. This performs one network request; see PRIVACY.md.`
- Add `SWAPAUDIT_CHECK_UPDATES=1` as an equivalent explicit opt-in for scripts.
- Remove `--no-update-check` because v0.1.0 is the first public release and the default is already silent. Update all docs and tests. If the maintainer insists on compatibility, keep it as a deprecated no-op with a one-line warning; do not keep dead behavior.
- Remove `SWAPAUDIT_NO_UPDATE_CHECK`; the default is silent. Document the removal in `CHANGELOG.md`.
- Remove `GetCachedUpdate` and the 24-hour automatic cadence if they are unused. Keep `LoadState`/`SaveState` only for explicit-check observability.
- `--version`, `--help`, `upgrade --help`, and invalid-flag paths must not perform network I/O, even if `--check-updates` is also present. Define precedence: version/help win; `upgrade` wins over `--check-updates` because upgrade already checks.
- `CI=1` does not need to suppress anything by default; an explicit `--check-updates` still runs in CI (the user asked for it).

### R4 — Option B (fallback): disclosed default

- Keep one check on the default path, but print before the request:
  `Checking for updates at https://api.github.com/repos/jadmadi/swapaudit/releases/latest (see PRIVACY.md).`
- Print it to stderr, not stdout, so piped audit output remains clean.
- Keep `--no-update-check` and `SWAPAUDIT_NO_UPDATE_CHECK=1` as opt-outs.
- Use a bounded synchronous check after the audit, not a 150 ms background race. If the user wants the audit fast, the check waits up to 2 s; document that.
- Write the state file only after a successful fetch; on failure, preserve the previous `LatestVersion`/`ReleaseURL` and update `LastChecked` at most once per day to avoid retry storms.
- The disclosure line must appear exactly once per invocation, even if the check fails or times out.

### R5 — Refactor the updater API

Prefer a small, explicit API:

```go
// CheckForUpdate performs one release check with the provided context.
func CheckForUpdate(ctx context.Context, currentVersion string) (*UpdateInfo, error)
```

- Remove or rename `CheckDailyUpdate`/`CheckDailyUpdateWithContext` so the name does not promise a cadence the code does not enforce.
- Keep `FetchLatestReleaseContext`, `IsNewer`, `CompareVersions`, `LoadState`, and `SaveState`.
- If `GetCachedUpdate` remains, it must not trigger network I/O; it may only read a state file. If unused, delete it.
- The version comparison must not treat `dev`/`none`/commit-hash builds as newer than a real release in a way that causes an infinite upgrade prompt. Existing tests cover this; add a regression test for a commit-hash version (`1a8479e`).
- `Upgrade` behavior is unchanged except for the asset-name fix owned by `standalone-v0-1-0-release`. This goal does not change asset naming.

### R6 — State-file behavior

- `SaveState` remains atomic (temp file + rename) and uses the existing UID-scoped fallback.
- On a successful check:
  - `LastChecked` = now.
  - `LatestVersion` = fetched tag.
  - `ReleaseURL` = fetched URL.
- On a failed check:
  - If a previous state exists, preserve `LatestVersion` and `ReleaseURL`; update `LastChecked` (or leave it unchanged if the policy says no retry storm). Document the choice in `PRIVACY.md`.
  - If no previous state exists, write only `LastChecked` and empty version fields, or skip the write. Document the choice.
- Never write a partially fetched version or a checksum.
- Tests must cover success, HTTP 404, HTTP 500, malformed JSON, empty tag, timeout, and a read-only cache directory.

### R7 — Tests and seams

Add an app-level seam so the default path can be tested without network:

```go
var checkForUpdate = updater.CheckForUpdate
```

Tests:

- `TestRunDefaultMakesNoNetworkRequest`: override `checkForUpdate` with a function that fails the test if called; run `Run([]string{"--no-color", "--ascii", "--limit", "1"})`; assert not called. Use `t.Setenv("CI", "")` and a temp `XDG_CACHE_HOME`.
- `TestRunCheckUpdatesFlag`: fake returns `&UpdateInfo{Current:"v0.1.0", Latest:"v0.2.0"}`; run `--check-updates`; assert called once and the notice appears.
- `TestRunCheckUpdatesEnv`: `SWAPAUDIT_CHECK_UPDATES=1`; assert called once.
- `TestRunVersionSkipsUpdateCheck`: override the seam to fail if called; run `--version`; assert not called.
- `TestRunUpgradeHelpSkipsUpdateCheck`: run `upgrade --help`; assert not called.
- `TestCheckForUpdateTimeout`: `httptest` server sleeps 500 ms; context timeout 50 ms; assert a timeout error and no hang.
- `TestCheckForUpdateSuccessWritesState`: fake server returns a tag; assert state fields.
- `TestCheckForUpdateFailurePreservesState`: pre-write a state; fake server returns 500; assert previous version/URL preserved.
- `TestCheckForUpdateCommitHashVersion`: current `1a8479e`, target `v0.1.0`; assert `IsNewer` behavior is intentional and documented.
- `TestUpdateNoticePrintedOnce`: assert the notice is not duplicated by both cached and live paths.

Target: `internal/updater` coverage rises from 46.4% to at least 70%; `internal/app` coverage does not decrease.

### R8 — Claims sentry

- Extend the claims sentry from `cleanup-orphan-uid-safety` (or add a separate `scripts/check-privacy-claims.sh`) to enforce the chosen policy:
  - Option A: fail if `README.md` or `GOVERNANCE.md` contains an unqualified "does not phone home" sentence that is not the `PRIVACY.md` link/sentence. The default is silent, so the claim is allowed only with "by default" and a `PRIVACY.md` link.
  - Option B: fail if "does not phone home" appears anywhere; fail if the disclosure line is missing from `README.md` or `PRIVACY.md`.
- The sentry must not fail on the word "telemetry" in the governance anti-goal; match phrases, not single words.
- Wire the sentry into `scripts/check.sh --full`.

### R9 — Help and docs

- `--check-updates` help text must name the endpoint and the privacy document.
- `README.md` gets an "Update checks and privacy" section with the exact default behavior, the opt-in command, and the state-file path.
- `GOVERNANCE.md` anti-goal #1 is rewritten to match the chosen policy, not the old absolute claim.
- `CHANGELOG.md` records the policy change, the flag change, and the removal of the 150 ms background check.
- `docs/decisions/0001-update-check-policy.md` is the single source of truth for the policy; README and GOVERNANCE link to it.

### R10 — Preserve audit behavior

- The audit remains fast by default. With Option A, the default run should stay near the observed 81 ms (no network).
- With Option B, the audit may take up to 2 s on the default path; document the trade-off.
- An update-check failure never changes the audit exit code.
- `--check-updates` with `--sort invalid` still exits `2` for the invalid sort; the update check does not mask a usage error.
- `--check-updates` with `--clean-all` is allowed; the check runs after the cleanup summary.

## Non-Goals

- No telemetry, analytics, accounts, or background daemon.
- No new update channel (stable/beta), no auto-download, no auto-install.
- No change to the upgrade asset naming (owned by `standalone-v0-1-0-release`).
- No change to release signing (owned by `standalone-v0-1-0-release`).
- No change to the audit's core memory/swap logic.

## Acceptance Criteria

### Shared

- **AC-01**: `go test -race ./...` passes.
- **AC-02**: `go vet ./...` passes.
- **AC-03**: `go test -cover ./internal/updater ./internal/app` shows updater coverage at least 70% and no app coverage decrease.
- **AC-04**: `docs/decisions/0001-update-check-policy.md` exists and records A or B, the approver, the date, and the exact privacy sentence.
- **AC-05**: `PRIVACY.md` exists and states the endpoint, headers, data not sent, state-file path, and opt-in/opt-out mechanism.
- **AC-06**: `README.md` and `GOVERNANCE.md` link to `PRIVACY.md` and do not contradict it.
- **AC-07**: `scripts/check.sh --full` passes, including the privacy/claims sentry.
- **AC-08**: `./swapaudit --version` and `./swapaudit --help` perform no network I/O. Verify with `strace -f -e trace=network`.
- **AC-09**: `./swapaudit --sort invalid --check-updates` exits `2` and does not perform a network check before rejecting the flag.

### Option A (opt-in)

- **AC-A1**: `strace -f -e trace=network ./swapaudit --no-color --ascii --limit 1` shows no network syscalls to GitHub.
- **AC-A2**: `./swapaudit --check-updates --no-color --ascii --limit 1` prints one update notice or "already up to date" and exits `0`.
- **AC-A3**: `SWAPAUDIT_CHECK_UPDATES=1 ./swapaudit --no-color --ascii --limit 1` performs the check.
- **AC-A4**: `--no-update-check` and `SWAPAUDIT_NO_UPDATE_CHECK` are removed from help and docs (or explicitly deprecated with a warning; the default is silent either way).
- **AC-A5**: A `--check-updates` timeout prints the warning and exits `0`.
- **AC-A6**: The default run's elapsed time remains within 20% of the pre-change baseline on this host.

### Option B (disclosed default)

- **AC-B1**: The default run prints the exact disclosure line once to stderr before the request.
- **AC-B2**: `--no-update-check` suppresses both the line and the request.
- **AC-B3**: A 500 ms server response is not canceled at 150 ms; the state file is written after success.
- **AC-B4**: The default run completes the audit even if the check times out; exit code remains `0`.
- **AC-B5**: `PRIVACY.md` and `README.md` contain the same disclosure sentence.

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
go test -cover ./internal/updater ./internal/app
./scripts/check.sh --full
strace -f -e trace=network ./swapaudit --no-color --ascii --limit 1 2>&1 | grep -E 'connect|sendto|socket' || true
strace -f -e trace=network ./swapaudit --version 2>&1 | grep -E 'connect|sendto|socket' || true
./swapaudit --check-updates --no-color --ascii --limit 1
./swapaudit --sort invalid --check-updates >/dev/null 2>&1; echo $?
```

Add the exact outputs and the policy decision to `goals/update-check-consent-disclosure/evidence.md` before certifying.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Users expect automatic update notices and miss security releases | Opt-in is explicit; README explains how to check; release notes are public; `--check-updates` is one command |
| Removing `--no-update-check` breaks scripts | v0.1.0 is the first public release; document the removal; if compatibility is required, keep a deprecated no-op with a warning |
| GitHub rate limits explicit checks | The user controls invocation; no automatic cadence; document `SWAPAUDIT_REPO` for forks |
| State file becomes stale or misleading | Write only on success; preserve previous version/URL on failure; show `LastChecked` in `PRIVACY.md` |
| A synchronous check makes the audit slower under Option B | Bound to 2 s; document; Option A is preferred |
| A test accidentally hits the real GitHub API | Inject `checkForUpdate`; use `httptest`; set `SWAPAUDIT_REPO` only in tests; never hard-code a live call in unit tests |

## Rollback

- Revert the implementation commit. The old default check returns; the privacy contradiction returns with it.
- If Option A causes a support problem, publish a follow-up that adds a one-time, disclosed check with `--check-updates`; do not silently restore background network I/O.
- If Option B is chosen and users object, switch to Option A in a new goal; do not keep both default behaviors.

## References

- Business review: `docs/business-reviews/2026-10-19-swapaudit-full-review.md` (§5, §6.3, §7 G-07)
- `GOVERNANCE.md:36` (current no-phone-home claim)
- `internal/app/app.go:189-192`, `:252` (background check and 150 ms cutoff)
- `internal/updater/updater.go:24-28`, `:223`, `:317` (default repo, 24 h interval, cached notice)
- `README.md:259-281` (daily check and upgrade claims)
- Existing tests: `internal/app/app_test.go` (`TestRunCachedUpdateNotice`, `TestRunNoUpdateCheck`), `internal/updater/updater_test.go`
- GitHub REST API release endpoint: `https://api.github.com/repos/jadmadi/swapaudit/releases/latest`
