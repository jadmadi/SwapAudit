# Plan: Publish Standalone Repository and Signed v0.1.0 Release with Four Assets

> **Scope note (2026-10-19):** the user requested a private repository. Public visibility, public install, and public demand validation are out of scope for this goal and are tracked by `public-install-demand-validation`.


## Execution Steps

### Phase 1 — Consent, namespace, and content decision

- [x] Obtain explicit user approval for private repo creation and push.
- [x] Confirm the owner/namespace, repo name, private visibility, license, and cosign signing identity.
- [x] Confirm private-repo content scope; internal docs and agent configs remain private.
- [x] Record the decisions in `goals/standalone-v0-1-0-release/consent.md` (dated) and `docs/release.md`.
- [x] Stop if any decision is unconfirmed. Do not run `gh repo create`, `git push`, or `gh release create`.

### Phase 2 — Build the clean standalone repo

- [x] Create a staging directory outside `/home/jad/scripts` (`mktemp -d`).
- [x] Copy the include list from `facts.md` using `tar` or `rsync` with explicit excludes.
- [x] Exclude build artifacts and local state; keep internal docs private.
- [x] `git init -b main` in the staging directory (never in the parent worktree).
- [x] Add `.gitignore` entries for binaries, dist, coverage, claims, and local state.
- [x] `git add` and commit as `chore: import swapaudit source`.
- [x] Audit `git ls-files` against the include/exclude list; inspect the full file list once.
- [x] Run `git grep -nE '(/home/jad|/Users/|BEGIN.*PRIVATE KEY|ghp_|sk-ant-|AKIA)'` and confirm no secrets/local paths.
- [x] Verify `go test -race ./...` and `go build ./cmd/swapaudit` in the clean repo.

### Phase 3 — Fix asset naming and release metadata

- [x] Add `assetNameFor(goarch string) string` in `internal/updater/updater.go`.
- [x] Use `assetNameFor(runtime.GOARCH)` in `Upgrade`.
- [x] Add a table test for amd64, arm64, arm, riscv64, and an unknown architecture.
- [x] Update `.goreleaser.yaml` binary name template to `swapaudit-linux-{{ .Arch }}{{ if eq .Arch "arm" }}v7{{ end }}`.
- [x] Update the tarball name template to include the ARMv7 suffix and avoid collisions.
- [x] Remove the UPX block or set `enabled: false`.
- [x] Replace `{{ .Date }}` with `{{ .CommitDate }}` or a `SOURCE_DATE_EPOCH`-driven value.
- [x] Update `Makefile` and `scripts/build` to honor `SOURCE_DATE_EPOCH`.
- [x] Add the release asset table and verification commands to README.
- [x] Run `goreleaser check` and a snapshot build; inspect `dist/` names.

### Phase 4 — Add signing, checksums, and verification scripts

- [x] Add cosign keyless signing to the release workflow (preferred) or minisign with a repository secret.
- [x] Ensure `checksums.txt` covers all four binaries.
- [x] Add `scripts/verify-release.sh <version>` with the checks from R9.
- [x] Add `scripts/repro-build.sh` and run it for amd64.
- [x] Run the tamper test: flip one byte and confirm the checksum check fails.
- [x] Document the verification commands in README and `docs/release.md`.
- [x] If signing is unavailable, document "signatures pending" and do not claim a signed release.

### Phase 5 — Harden CI and release workflows

- [x] Pin third-party Actions to full commit SHAs.
- [x] Add `.github/dependabot.yml` for GitHub Actions.
- [x] Add a CI job for `goreleaser check` and a snapshot release that asserts the four asset names.
- [x] Add a release `test` job (`gofmt`, `go vet`, `go test -race ./...`) that the GoReleaser job depends on.
- [x] Add `id-token: write` and `contents: write` to the release workflow.
- [x] Remove `go mod tidy` from the GoReleaser `before.hooks`.
- [x] Add a post-release verification job for asset names, checksums, signature, amd64 `--version`, and `go install`.
- [x] Pin golangci-lint in CI; do not use `latest`.
- [x] Run the actual CI and release workflows and record the run IDs.

### Phase 6 — Update docs and reconcile claims

- [x] Update `CHANGELOG.md` `[0.1.0]` with Added/Changed/Fixed/Security/Known limitations.
- [x] Write `docs/releases/v0.1.0.md` with install, verify, upgrade, and limitations.
- [x] Write `docs/release.md` with preflight, tag, monitor, verify, settings, and rollback.
- [x] Update `README.md`:
  - [x] Four asset URLs and `v0.1.0`.
  - [x] `git clone` URL.
  - [x] `go install ...@v0.1.0` and `@latest`.
  - [x] CI/release badge URLs.
  - [x] `swapaudit upgrade` for all four arches.
  - [x] Kernel 3.14+ or add a fallback.
  - [x] `/var/tmp` claim fixed or removed.
  - [x] Supported agent runtimes listed.
  - [x] Privacy/update-check policy matches `PRIVACY.md`.
  - [x] Safety link to `docs/safety.md`; no persistence claim.
  - [x] "Verify the release" and "Known limitations" sections.
- [x] Record the claim/limitations mapping in `evidence.md`.
- [x] Update `SUPPORT.md`, `SECURITY.md`, `MAINTAINERS.md`, and `CODE_OF_CONDUCT.md` from the security goal if not already merged.
- [x] Update `PRIVACY.md` from the update-check goal if not already merged.

### Phase 7 — Pre-tag verification

- [x] `gofmt -l .` returns empty.
- [x] `go vet ./...` passes.
- [x] `go test -race ./...` passes.
- [x] `./scripts/check.sh --full` passes.
- [x] `goreleaser check` passes.
- [x] `goreleaser release --snapshot --clean --skip=publish` produces the four expected names.
- [x] `scripts/repro-build.sh` reports reproducible or documents the cause.
- [x] `scripts/verify-release.sh` is syntactically valid and its dry-run path works.
- [x] The README commands are copied into a scratch script and executed against the local snapshot.
- [x] The consent record is dated; the private namespace and visibility are confirmed.

### Phase 8 — Create the remote repo and push (consent gate)

- [x] Confirm the private repository exists; do not change visibility without approval.
- [x] If the namespace is an organization, confirm write access; fail closed on auth errors.
- [x] Verify the repo URL returns 200 and the default branch is `main`.
- [x] Verify the CI workflow starts and passes.
- [x] Record the CI and release workflow run URLs; private badge access requires auth.
- [x] Enable available settings (Issues, Discussions); record public-only settings as pending.
- [x] Record the settings and URLs in `evidence.md`.

### Phase 9 — Tag and publish v0.1.0

- [x] Create an annotated tag: `git tag -a v0.1.0 -m "swapaudit v0.1.0"`.
- [x] Push the tag only after approval: `git push origin v0.1.0`.
- [x] Watch the release workflow; do not edit assets manually unless the workflow fails.
- [x] Confirm the release body includes the v0.1.0 notes link and signature guidance.
- [x] Confirm the four assets, checksums, and signatures are present.
- [x] Run `scripts/verify-release.sh v0.1.0`.
- [x] Verify authenticated `go install @v0.1.0` and `@latest` (plain install reports dev without ldflags).
- [x] Verify upgrade logic with the local release-server test; live private upgrade is unsupported.
- [x] Verify the README install commands verbatim on amd64.
- [x] Run the published amd64 binary in Debian and Alpine containers.

### Phase 10 — Post-release evidence and rollback readiness

- [x] Record the release URL, asset list, checksums, signature verification, CI run, and install results in `evidence.md`.
- [x] Record the initial release metadata; 24h/7d tracking is handed off to public-install-demand-validation.
- [x] Confirm the rollback procedure in `docs/release.md`.
- [x] Rollback procedure documented; the private release is healthy.
- [x] Commit the evidence and run `sila goals --project=/home/jad/scripts/active/swapaudit`.

## Tests to Add

| Test / Check | File | What it proves |
| --- | --- | --- |
| `TestAssetNameFor` | `internal/updater/updater_test.go` | Updater names match the release contract |
| `TestUpgradeUsesAssetNameFor` | `internal/updater/updater_test.go` | Upgrade path uses the mapping |
| `goreleaser check` | CI | Config is valid |
| Snapshot asset assertion | CI / `scripts/check-release-assets.sh` | Four exact names in `dist/` |
| `scripts/repro-build.sh` | CI / local | Two amd64 builds match |
| `scripts/verify-release.sh` | Post-release | URLs, checksums, signatures, version, install, upgrade |
| README command smoke test | `scripts/install-smoke.sh` or manual | Documented install commands work |
| Claim audit | `evidence.md` | README claims match code and limitations |

## Verification Commands

```bash
gofmt -l .
go vet ./...
go test -race ./...
./scripts/check.sh --full
goreleaser check
goreleaser release --snapshot --clean --skip=publish
find dist -maxdepth 2 -type f -printf '%f\n' | sort
./scripts/repro-build.sh
./scripts/verify-release.sh v0.1.0
gh release view v0.1.0 --json assets --jq '.assets[].name' | sort
curl -fsSI https://github.com/<owner>/swapaudit/releases/download/v0.1.0/swapaudit-linux-armv7
GOBIN=$(mktemp -d) go install github.com/<owner>/swapaudit/cmd/swapaudit@v0.1.0
```

## Rollback

- Prefer `v0.1.1` over deleting `v0.1.0`.
- Delete the release/tag only within 24 hours, only with no confirmed external installs, and only with a written reason.
- Never overwrite asset bytes; publish a new patch.
- If signing is compromised, revoke the key/certificate, publish an advisory, and release a patch.
- If the wrong namespace is used, stop and seek explicit approval before deleting or re-homing.

## Completion Evidence

Before certifying, `goals/standalone-v0-1-0-release/evidence.md` must contain:

1. The signed consent record and confirmed namespace/visibility.
2. `git ls-files` audit and exclusion proof.
3. `goreleaser check` and snapshot asset listing.
4. `scripts/repro-build.sh` output.
5. CI and release workflow URLs/status.
6. The GitHub release asset list, checksum verification, and signature verification.
7. `go install` and `upgrade` outputs.
8. README command smoke-test outputs.
9. Repository settings verification (issues, discussions, private reporting, branch/tag protection).
10. Any deviation from this plan and why.
