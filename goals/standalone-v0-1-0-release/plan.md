# Plan: Publish Standalone Repository and Signed v0.1.0 Release with Four Assets

## Execution Steps

### Phase 1 — Consent, namespace, and content decision

- [ ] Obtain explicit user approval for repo creation and push.
- [ ] Confirm the owner/namespace, repo name, visibility, license, and signing identity.
- [ ] Confirm whether `goals/`, `docs/business-reviews/`, `AGENTS.md`, and agent config files are public.
- [ ] Record the decisions in `goals/standalone-v0-1-0-release/consent.md` (signed/dated) and `docs/release.md`.
- [ ] Stop if any decision is unconfirmed. Do not run `gh repo create`, `git push`, or `gh release create`.

### Phase 2 — Build the clean standalone repo

- [ ] Create a staging directory outside `/home/jad/scripts` (`mktemp -d`).
- [ ] Copy the include list from `facts.md` using `tar` or `rsync` with explicit excludes.
- [ ] Exclude `swapaudit`, `dist/`, `build/`, `coverage.*`, `.sila/`, `goals/*/claim.json`, `.cursor/`, `.devin/`, `.windsurfrules`, `docs/business-reviews/`, and any unapproved internal file.
- [ ] `git init -b main` in the staging directory (never in the parent worktree).
- [ ] Add `.gitignore` entries for binaries, dist, coverage, claims, and local state.
- [ ] `git add` and commit as `chore: import swapaudit source`.
- [ ] Audit `git ls-files` against the include/exclude list; inspect the full file list once.
- [ ] Run `git grep -nE '(/home/jad|/Users/|BEGIN.*PRIVATE KEY|ghp_|sk-ant-|AKIA)'` and confirm no secrets/local paths.
- [ ] Verify `go test -race ./...` and `go build ./cmd/swapaudit` in the clean repo.

### Phase 3 — Fix asset naming and release metadata

- [ ] Add `assetNameFor(goarch string) string` in `internal/updater/updater.go`.
- [ ] Use `assetNameFor(runtime.GOARCH)` in `Upgrade`.
- [ ] Add a table test for amd64, arm64, arm, riscv64, and an unknown architecture.
- [ ] Update `.goreleaser.yaml` binary name template to `swapaudit-linux-{{ .Arch }}{{ if eq .Arch "arm" }}v7{{ end }}`.
- [ ] Update the tarball name template to include the ARMv7 suffix and avoid collisions.
- [ ] Remove the UPX block or set `enabled: false`.
- [ ] Replace `{{ .Date }}` with `{{ .CommitDate }}` or a `SOURCE_DATE_EPOCH`-driven value.
- [ ] Update `Makefile` and `scripts/build` to honor `SOURCE_DATE_EPOCH`.
- [ ] Update README download URLs and architecture table to the four exact names.
- [ ] Run `goreleaser check` and a snapshot build; inspect `dist/` names.

### Phase 4 — Add signing, checksums, and verification scripts

- [ ] Add cosign keyless signing to the release workflow (preferred) or minisign with a repository secret.
- [ ] Ensure `checksums.txt` covers all four binaries.
- [ ] Add `scripts/verify-release.sh <version>` with the checks from R9.
- [ ] Add `scripts/repro-build.sh` and run it for amd64.
- [ ] Add a tamper test to the runbook: flip one byte, verify the checksum/signature fails.
- [ ] Document the verification commands in README and `docs/release.md`.
- [ ] If signing is unavailable, document "signatures pending" and do not claim a signed release.

### Phase 5 — Harden CI and release workflows

- [ ] Pin third-party Actions to full commit SHAs.
- [ ] Add `.github/dependabot.yml` for GitHub Actions.
- [ ] Add a CI job for `goreleaser check` and a snapshot release that asserts the four asset names.
- [ ] Add a release `test` job (`gofmt`, `go vet`, `go test -race ./...`) that the GoReleaser job depends on.
- [ ] Add `id-token: write` and `contents: write` to the release workflow.
- [ ] Remove `go mod tidy` from the GoReleaser `before.hooks`.
- [ ] Add a post-release verification job for asset names, checksums, signature, amd64 `--version`, and `go install`.
- [ ] Pin golangci-lint in CI; do not use `latest`.
- [ ] Run the workflow in a fork or with `act` if possible; otherwise review the YAML and record the limitation.

### Phase 6 — Update docs and reconcile claims

- [ ] Update `CHANGELOG.md` `[0.1.0]` with Added/Changed/Fixed/Security/Known limitations.
- [ ] Write `docs/releases/v0.1.0.md` with install, verify, upgrade, and limitations.
- [ ] Write `docs/release.md` with preflight, tag, monitor, verify, settings, and rollback.
- [ ] Update `README.md`:
  - [ ] Four asset URLs and `v0.1.0`.
  - [ ] `git clone` URL.
  - [ ] `go install ...@v0.1.0` and `@latest`.
  - [ ] CI/release badge URLs.
  - [ ] `swapaudit upgrade` for all four arches.
  - [ ] Kernel 3.14+ or add a fallback.
  - [ ] `/var/tmp` claim fixed or removed.
  - [ ] Supported agent runtimes listed.
  - [ ] Privacy/update-check policy matches `PRIVACY.md`.
  - [ ] Safety link to `docs/safety.md`; no persistence claim.
  - [ ] "Verify the release" and "Known limitations" sections.
- [ ] Cross-check every claim against `docs/business-reviews/2026-10-19-swapaudit-full-review.md` §5 and record the mapping in `evidence.md`.
- [ ] Update `SUPPORT.md`, `SECURITY.md`, `MAINTAINERS.md`, and `CODE_OF_CONDUCT.md` from the security goal if not already merged.
- [ ] Update `PRIVACY.md` from the update-check goal if not already merged.

### Phase 7 — Pre-tag verification

- [ ] `gofmt -l .` returns empty.
- [ ] `go vet ./...` passes.
- [ ] `go test -race ./...` passes.
- [ ] `./scripts/check.sh --full` passes.
- [ ] `goreleaser check` passes.
- [ ] `goreleaser release --snapshot --clean --skip=publish` produces the four expected names.
- [ ] `scripts/repro-build.sh` reports reproducible or documents the cause.
- [ ] `scripts/verify-release.sh` is syntactically valid and its dry-run path works.
- [ ] The README commands are copied into a scratch script and executed against the local snapshot.
- [ ] The consent record is signed; the namespace and visibility are confirmed.

### Phase 8 — Create the remote repo and push (consent gate)

- [ ] `gh repo create <owner>/swapaudit --public --source=<staging> --remote=origin --push` only after approval.
- [ ] If the namespace is an organization, confirm write access; fail closed on auth errors.
- [ ] Verify the repo URL returns 200 and the default branch is `main`.
- [ ] Verify the CI workflow starts and passes.
- [ ] Verify the CI/release badge URLs return 200 after the first run.
- [ ] Enable Issues, Discussions, Private Vulnerability Reporting, secret scanning, Dependabot alerts, and branch/tag protection.
- [ ] Record the settings and URLs in `evidence.md`.

### Phase 9 — Tag and publish v0.1.0

- [ ] Create an annotated tag: `git tag -a v0.1.0 -m "swapaudit v0.1.0"`.
- [ ] Push the tag only after approval: `git push origin v0.1.0`.
- [ ] Watch the release workflow; do not edit assets manually unless the workflow fails.
- [ ] Confirm the release body matches `docs/releases/v0.1.0.md`.
- [ ] Confirm the four assets, checksums, and signatures are present.
- [ ] Run `scripts/verify-release.sh v0.1.0`.
- [ ] Verify `go install ...@v0.1.0` and `@latest` with a bounded retry.
- [ ] Verify `swapaudit upgrade` from a local v0.0.1 build.
- [ ] Verify the README install commands verbatim on amd64.
- [ ] Run the install smoke test on at least two distros/containers if available.

### Phase 10 — Post-release evidence and rollback readiness

- [ ] Record the release URL, asset list, checksums, signature verification, CI run, and install results in `evidence.md`.
- [ ] Record the download counts at 24 hours and 7 days (for the validation goal).
- [ ] Confirm the rollback procedure in `docs/release.md`.
- [ ] If broken, publish `v0.1.1`; do not overwrite assets.
- [ ] Commit the evidence and run `sila goals --project=/home/jad/scripts/active/swapaudit`.

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
