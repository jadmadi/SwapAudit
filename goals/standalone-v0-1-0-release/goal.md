# Goal: Publish Standalone Repository and v0.1.0 Release with Four Assets

## Chain of Custody (Isnad)

- **Registered By**: Business review agent, 2026-10-19
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: Pending Mystery Audit

## Goal Description

Turn the local project into a **private**, installable, verifiable standalone release. Public publication is a separate decision; the user requested a private repository.

- Create a clean standalone repository at the maintainer-confirmed URL (currently intended: `github.com/jadmadi/swapaudit`).
- Import the source without the parent worktree's history, local build artifacts, internal agent configuration, business-review notes, or unverifiable goal receipts.
- Fix the ARM asset-name mismatch so the updater and the release produce the same four names.
- Publish `v0.1.0` with exactly four Linux binaries: `amd64`, `arm64`, `armv7`, `riscv64`.
- Publish checksums, and signatures when signing is provisioned; verify a tampered binary is rejected when signatures are claimed.
- Make every README install and upgrade command work end to end.
- Prove the release with a repeatable verification script, not a manual click-through.

This goal does **not** add product features. It is the distribution foundation that makes the existing product reachable.

## Business Context

- The business review (`docs/business-reviews/2026-10-19-swapaudit-full-review.md`) ranks the missing public repository/release as **Critical** (G-01) and the ARM asset mismatch as **High** (G-06).
- Every documented install path currently returns 404: raw download, `git clone`, `go install`, and `swapaudit upgrade`.
- The project directory is untracked inside the parent `jadmadi/scripts` worktree, so there is no standalone history, backup, or CI.
- The local build works (race tests pass; 81 ms audit) and the four-architecture GoReleaser config exists; the release is a packaging and trust problem, not a code-rewrite problem.
- This goal depends on `cleanup-orphan-uid-safety`, `update-check-consent-disclosure`, and `security-contact-consolidation`. Do not tag a public release that still contains a false-positive kill path, a privacy contradiction, or a broken security contact.

## Dependencies & Execution Order

- **Mode**: Dependent (requires prerequisite goals)
- **Depends On**: `cleanup-orphan-uid-safety`, `update-check-consent-disclosure`, `security-contact-consolidation`
- **Sequence**: 4
- **Shape**: ship
- **Tier**: immediate

## Consent and Namespace Gate (Mandatory)

This goal touches shared state and is not locally reversible once published. Before any remote action:

- [ ] Obtain explicit user approval for repository creation and push.
- [ ] Confirm the canonical owner/namespace (personal `jadmadi` vs an organization) and the repository name.
- [ ] Confirm visibility (public), license (WaqfDPL + AGPL), and whether `goals/`, `docs/business-reviews/`, `AGENTS.md`, and agent configuration files are public.
- [ ] Confirm the release owner and the signing identity (cosign keyless OIDC vs minisign key).
- [ ] If any of the above is unconfirmed, stop. Do not run `gh repo create`, `git push`, or create a release.
- [ ] Follow `AGENTS.md` safety rails: no unauthorized repo creation, no namespace re-homing, fail closed on auth/namespace errors.

The local extraction, build, and verification steps may run before this gate. Remote creation/push/release may not.

## Files to Touch

### New standalone repository contents

- `cmd/`, `internal/`, `go.mod`
- `Makefile`, `build.sh`, `scripts/`
- `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/dependabot.yml`
- `.goreleaser.yaml`
- `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `GOVERNANCE.md`, `MAINTAINERS.md`, `SECURITY.md`, `SUPPORT.md`, `PRIVACY.md`
- `LICENSE`, `LICENSE-AGPL.txt`, `WaqfDPL-1.0.md`
- `docs/safety.md`, `docs/release.md`, `docs/releases/v0.1.0.md`, `docs/decisions/*.md`
- `.gitignore`
- Optional: `goals/` only if every audit receipt references a commit that exists in the new repo; otherwise exclude it.
- Optional: `AGENTS.md` only after sanitizing internal-only instructions; otherwise exclude it.

### Explicitly excluded from the public repo

- `/swapaudit` binary, `dist/`, `build/`, `coverage.*`, `coverage.html`.
- `.sila/` local state and `goals/*/claim.json`.
- `.cursor/`, `.devin/`, `.windsurfrules`, and any local agent configuration.
- `docs/business-reviews/` unless the maintainer explicitly approves publishing the commercial analysis.
- Any audit receipt with a commit SHA that does not resolve in the new repository.
- Any secret, `.env`, credential, local absolute path, or machine-specific state.

## Requirements

### R1 — Build the standalone repository cleanly

- Create a staging directory outside `/home/jad/scripts` (for example `mktemp -d`), then copy the project with an explicit include/exclude list.
- Do not run `git init` inside the existing parent worktree; do not create a nested repository in `/home/jad/scripts/active/swapaudit`.
- Initialize `main`, add all intended files, and commit as `chore: import swapaudit source`.
- Verify:
  - `git ls-files` includes `cmd/`, `internal/`, `go.mod`, `.goreleaser.yaml`, `.github/workflows/`, licenses, and the updated docs.
  - `git ls-files | grep -E '(^|/)(swapaudit|dist|build|coverage|\.sila|\.cursor|\.devin|business-reviews)'` returns no intended exclusions.
  - `git log --oneline` has a real history for the new repo.
- The new repo's `go.mod` module path remains `github.com/jadmadi/swapaudit`; if the confirmed namespace differs, update the module path and every import consistently.

### R2 — Fix the release asset-name contract

Current state: `internal/updater/updater.go:409-413` maps `runtime.GOARCH == "arm"` to `swapaudit-linux-armv7`, but `.goreleaser.yaml:43,51` uses `{{ .Arch }}` (= `GOARCH` = `arm`), producing `swapaudit-linux-arm`. The updater would 404 on ARMv7.

Required exact names:

| Architecture | Asset name |
| --- | --- |
| amd64 | `swapaudit-linux-amd64` |
| arm64 | `swapaudit-linux-arm64` |
| armv7 | `swapaudit-linux-armv7` |
| riscv64 | `swapaudit-linux-riscv64` |

Implementation:

- Add `func assetNameFor(goarch string) string` in `internal/updater/updater.go`; return `swapaudit-linux-armv7` for `arm`, otherwise `swapaudit-linux-<goarch>`.
- Use it in `Upgrade` instead of inline string formatting.
- Add a table test for the four names and an unknown architecture (return a deterministic fallback or an error; document the choice).
- Update `.goreleaser.yaml` binary name template to produce the same names, for example:
  `swapaudit-linux-{{ .Arch }}{{ if eq .Arch "arm" }}v7{{ end }}`
- Update tarball names to avoid collisions using the same ARMv7 suffix.
- Update README download URLs and the architecture table.
- Add a CI `goreleaser check` plus a snapshot build that asserts the four names. If GoReleaser snapshot naming differs, inspect `dist/` and record the exact output.

### R3 — Disable UPX for v0.1.0

- Remove the `upx` block from `.goreleaser.yaml` or set `enabled: false`.
- Ship uncompressed static binaries for v0.1.0 to improve reproducibility and reduce endpoint-scanner friction.
- Document the decision in `docs/release.md`; if binary size becomes a complaint, evaluate UPX in a follow-up with endpoint testing.
- Keep `CGO_ENABLED=0`, `-trimpath`, and `-s -w` where appropriate.

### R4 — Make the build reproducible

- Replace the wall-clock build date with a deterministic value:
  - GoReleaser: use `{{ .CommitDate }}` for `internal/version.Date`, or set `SOURCE_DATE_EPOCH`.
  - `Makefile` and `scripts/build`: honor `SOURCE_DATE_EPOCH`; default to the commit date or a documented fixed value.
- Add `scripts/repro-build.sh`:
  - Build `linux/amd64` twice in separate temp directories with the same flags.
  - Compare SHA256; print `reproducible: yes` or the differing hashes.
  - Do not require network access.
- Run the script in CI on `amd64` and record the result. If Go's build ID makes the binaries differ, document the exact cause and do not claim reproducibility.
- Document the toolchain (Go 1.23 for release; local Go 1.27 must not change release flags).

### R5 — Add signing and checksums

- GoReleaser must produce `checksums.txt` for all four binaries.
- Preferred signing: cosign keyless in GitHub Actions:
  - Add `id-token: write` to the release workflow.
  - Install cosign with `sigstore/cosign-installer`.
  - Sign `checksums.txt`; publish the signature and certificate/bundle.
  - Verify with `cosign verify-blob` in `scripts/verify-release.sh`.
- Fallback: minisign with a key stored in repository secrets; publish the public key and signature.
- If signing cannot be provisioned, publish checksums only and explicitly document "signatures pending." Do **not** label the release "signed" unless `scripts/verify-release.sh` verifies it.
- Add a tamper test to the verification runbook: modify one byte of a downloaded asset, run the checksum/signature check, and confirm failure.

### R6 — Harden CI and the release workflow

`.github/workflows/ci.yml`:

- Trigger on `push` and `pull_request` to `main`.
- `permissions: contents: read`.
- Jobs: formatting, `go vet`, golangci-lint, `go test -race ./...`, cross-compile matrix for amd64/arm64/arm/riscv64, `goreleaser check`, and a snapshot release that asserts the four asset names.
- Pin third-party Actions to full commit SHAs; add `.github/dependabot.yml` for `github-actions` updates.
- Do not use `latest` for golangci-lint in a release-blocking job; pin a version.

`.github/workflows/release.yml`:

- Trigger on `v*` tags.
- `permissions: contents: write`, `id-token: write`.
- `fetch-depth: 0`.
- Run a `test` job (`go test -race ./...`, `go vet`, `gofmt -l`) before GoReleaser; the release job depends on it.
- Install Go 1.23, cosign (if signing), and GoReleaser v2 (pinned).
- Run `goreleaser release --clean`.
- Add a post-release `verify` job that downloads the assets, checks names/checksums/signature, runs the amd64 `--version`, and runs `go install ...@v0.1.0` with a bounded retry.
- Do not run `go mod tidy` as a release hook; it can mutate the worktree and complicate reproducibility.

### R7 — Update version metadata and release notes

- `--version` must print `swapaudit v0.1.0 (commit: <short>, built: <date>)`.
- `git describe --tags --exact-match` must return `v0.1.0` at the tagged commit.
- `CHANGELOG.md` `[0.1.0]` entry must include:
  - Added: standalone release, four assets, checksums, signing, install verification.
  - Changed: update-check policy; `--clean-all` orphan semantics; security contact; UPX removal.
  - Fixed: orphan heuristic, UID enforcement, unverified conversation claim, ARMv7 asset name.
  - Security: signing/checksum verification, private reporting path.
  - Known limitations: Linux-only; kernel 3.14+ (or the documented fallback); supported agent runtimes; no JSON/package-manager distribution yet.
- Add `docs/releases/v0.1.0.md` with the release body, install commands, verification commands, and known limitations.
- The GitHub release body must include the same content; do not rely on an auto-generated changelog alone.

### R8 — Reconcile every README claim

Before tagging, audit `README.md` against the business review's claim table:

- Install URLs must use the four asset names and `v0.1.0`.
- `git clone` URL must match the confirmed repo.
- `go install` must be `github.com/<owner>/swapaudit/cmd/swapaudit@v0.1.0` and `@latest`.
- CI/release badges must match the confirmed repo.
- `swapaudit upgrade` must work on all four architectures.
- Kernel requirement: either require Linux 3.14+ (because `MemAvailable` is used) or implement a fallback. Do not ship the 3.10 claim unless it is true.
- `/var/tmp`: either scan it or remove the claim that it is cleaned.
- Agent detection: list the supported runtimes (`agy`, `devin`, `chrome-devtools`, `antigravity`) and state that others are not classified.
- Privacy/update check: match `PRIVACY.md` and the chosen policy.
- Safety: link `docs/safety.md`; do not claim conversation persistence.
- Add a "Known limitations" section.
- Add a "Verify the release" section with checksums/signature commands.

### R9 — Add a release verification script

Add `scripts/verify-release.sh <version>`:

- Requires `gh`, `curl`, `sha256sum` (or `shasum`), `go`, and `cosign` if signatures are claimed.
- Checks:
  1. `gh release view <version> --json assets` contains exactly the four binary names plus `checksums.txt` and signature files.
  2. `curl -fsSL` each asset URL returns 200.
  3. `sha256sum -c checksums.txt` passes for all downloaded assets.
  4. `cosign verify-blob` (or minisign) passes; tampering one byte fails.
  5. `./swapaudit-linux-amd64 --version` contains `<version>`.
  6. `GOBIN=$(mktemp -d) go install github.com/<owner>/swapaudit/cmd/swapaudit@<version>` succeeds and `--version` works.
  7. Upgrade path: build a local `v0.0.1` test binary, run `SWAPAUDIT_REPO=<owner>/swapaudit <binary> upgrade`, and assert it reports `<version>`.
  8. The README install commands are copied verbatim into the script and executed.
- Exit non-zero on any mismatch. Print a concise pass/fail table.

### R10 — Repository settings and post-publication checks

After the repo is public (requires the consent gate):

- Enable Issues and Discussions; disable blank issues via the templates.
- Enable Private Vulnerability Reporting; verify the URL from `SECURITY.md`.
- Enable secret scanning, Dependabot alerts, and branch/tag protection for `main` and `v*`.
- Require CI status checks before merge.
- Verify the CI and release badges return 200.
- Verify `go install ...@latest` resolves after the module proxy indexes the tag (allow a bounded retry).
- Record all settings and outputs in `goals/standalone-v0-1-0-release/evidence.md`.

### R11 — Keep the release scope frozen

- No new scanners, agent runtimes, JSON output, package-manager packages, GUI, daemon, or telemetry in v0.1.0.
- The only code changes allowed are the safety/update/contact prerequisites, the asset-name fix, release metadata, signing, CI, and docs.
- If a prerequisite goal is incomplete, do not tag; publish a release candidate or wait.

### R12 — Rollback and incident handling

- Prefer publishing `v0.1.1` over deleting `v0.1.0`; deleting a tag can break module-proxy caches and users.
- If `v0.1.0` is broken and no external installs exist, deleting the release/tag within 24 hours is acceptable with a written note.
- Never overwrite an existing asset with different bytes; publish a new patch version.
- If signing fails, do not publish unsigned assets as "signed"; publish checksums and document the limitation.
- Document the rollback procedure in `docs/release.md`.

## Non-Goals

- No new features or agent coverage.
- No package-manager distribution (AUR, Homebrew, Nix, deb/rpm) in v0.1.0; this is a separate distribution goal.
- No JSON output or dashboard integration.
- No telemetry, accounts, or hosted service.
- No Windows/macOS support.
- No remote repository creation or push without the consent gate.
- No publishing internal business reviews, local agent configs, or unverifiable goal receipts.

## Acceptance Criteria

- **AC-01**: The consent gate is recorded and signed by the user before any remote action.
- **AC-02**: A clean standalone repo exists at the confirmed URL; `git ls-files` contains the intended source/docs and no binaries, `dist/`, `.sila/`, `.cursor/`, `.devin/`, or business-review notes.
- **AC-03**: `git tag -l` shows `v0.1.0`; `git describe --tags --exact-match` returns `v0.1.0` at the release commit.
- **AC-04**: The GitHub release lists exactly `swapaudit-linux-amd64`, `swapaudit-linux-arm64`, `swapaudit-linux-armv7`, `swapaudit-linux-riscv64`, `checksums.txt`, and the signature/attestation files (if claimed).
- **AC-05**: `curl -fsSL` for each of the four asset URLs returns 200.
- **AC-06**: `sha256sum -c checksums.txt` passes for all four assets.
- **AC-07**: If signatures are claimed, `cosign verify-blob` (or minisign) passes; a one-byte tamper fails.
- **AC-08**: `swapaudit-linux-amd64 --version` reports `v0.1.0` and the expected commit/date.
- **AC-09**: `go install github.com/<owner>/swapaudit/cmd/swapaudit@v0.1.0` succeeds in a clean `GOBIN`; `@latest` resolves after proxy indexing.
- **AC-10**: `swapaudit upgrade` from a local `v0.0.1` test build reaches `v0.1.0` on amd64; the asset-name unit test covers armv7.
- **AC-11**: The CI workflow runs on the public repo and the badge returns 200; the release workflow runs on the tag and publishes the assets.
- **AC-12**: `scripts/verify-release.sh v0.1.0` passes end to end and is documented in `docs/release.md`.
- **AC-13**: `scripts/repro-build.sh` reports reproducible or documents the exact non-reproducible input; the release date ldflag is deterministic.
- **AC-14**: README install, verify, and upgrade commands work verbatim; the claim audit has no known false statement.
- **AC-15**: `SECURITY.md` private-reporting URL is active after publication and verified by the release goal.
- **AC-16**: `CHANGELOG.md`, `docs/releases/v0.1.0.md`, and the GitHub release body agree.
- **AC-17**: No feature work is included; the release contains only the prerequisite fixes and release plumbing.
- **AC-18**: All verification outputs are recorded in `goals/standalone-v0-1-0-release/evidence.md`.

## Verification Commands

```bash
# Local pre-tag
gofmt -l .
go vet ./...
go test -race ./...
./scripts/check.sh --full
goreleaser check
goreleaser release --snapshot --clean --skip=publish
find dist -maxdepth 2 -type f -printf '%f\n' | sort

# After publication
gh release view v0.1.0 --json assets --jq '.assets[].name' | sort
curl -fsSI https://github.com/<owner>/swapaudit/releases/download/v0.1.0/swapaudit-linux-amd64
curl -fsSI https://github.com/<owner>/swapaudit/releases/download/v0.1.0/swapaudit-linux-arm64
curl -fsSI https://github.com/<owner>/swapaudit/releases/download/v0.1.0/swapaudit-linux-armv7
curl -fsSI https://github.com/<owner>/swapaudit/releases/download/v0.1.0/swapaudit-linux-riscv64
./scripts/verify-release.sh v0.1.0
GOBIN=$(mktemp -d) go install github.com/<owner>/swapaudit/cmd/swapaudit@v0.1.0
```

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Publishing under the wrong namespace or leaking internal files | Mandatory consent gate; explicit include/exclude list; `git ls-files` audit; no remote action until confirmed |
| ARMv7 asset still 404s | `assetNameFor` unit test; GoReleaser snapshot assertion; `verify-release.sh` covers all four URLs |
| Unsigned release is mistaken for signed | Only claim signing after `verify-release.sh` passes; otherwise document "signatures pending" |
| UPX or build date breaks reproducibility | Disable UPX; use commit date/SOURCE_DATE_EPOCH; `repro-build.sh` |
| Tag publishes broken code | Release workflow depends on a test job; pre-tag `scripts/check.sh --full`; branch/tag protection |
| Module proxy has not indexed the tag when verification runs | Bounded retry with backoff; use `GOPROXY=direct` as a fallback for the verification step |
| README still contains a false claim | Line-by-line claim audit against the business review; known-limitations section |
| Deleting a bad release breaks users | Prefer `v0.1.1`; delete only within 24 hours with no external installs |
| Internal business analysis or agent config leaks | Explicit exclusion list; manual diff review before push |

## Rollback

- Prefer `v0.1.1` for any broken asset; never overwrite bytes in place.
- If `v0.1.0` must be removed, delete the GitHub release and tag only within 24 hours and only if no external installs are confirmed; document the reason.
- If signing is compromised, revoke the key/certificate, publish an advisory, and release a new patch.
- If the repo is created under the wrong namespace, stop and delete only with explicit approval; do not re-home silently.

## References

- Business review: `docs/business-reviews/2026-10-19-swapaudit-full-review.md` (§5, §6.1, §6.3, §7 G-01/G-06/G-09/G-15)
- `.goreleaser.yaml:13-54` (build matrix, UPX, names, checksums)
- `.github/workflows/ci.yml` (lint, race, cross-compile)
- `.github/workflows/release.yml:1-4` (tag trigger)
- `internal/updater/updater.go:368-470` (upgrade flow, asset name, checksum)
- `README.md:118-148` (install methods), `:259-281` (update/upgrade)
- GoReleaser templates: `.Arch` = GOARCH, `.Arm` = GOARM
- Go module proxy: `https://proxy.golang.org/github.com/<owner>/swapaudit/@v/list`
- GitHub release API: `https://api.github.com/repos/<owner>/swapaudit/releases/latest`
