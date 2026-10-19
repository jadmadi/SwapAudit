# Facts: Publish Standalone Repository and Signed v0.1.0 Release with Four Assets

## Architectural Invariants & Constraints

- No remote repository creation, push, or release without explicit user approval and a confirmed namespace.
- The public repo must not contain local binaries, `dist/`, `.sila/`, agent configuration, business-review notes, secrets, or unverifiable audit receipts.
- Exactly four Linux binary assets: `amd64`, `arm64`, `armv7`, `riscv64`.
- `checksums.txt` is required; signatures are required if the release claims to be signed.
- The updater and the release must agree on asset names. No 404 on any advertised architecture.
- `CGO_ENABLED=0`; Linux-only; no new runtime dependencies.
- The release must not include new features. It ships the three prerequisite fixes and release plumbing only.
- The release must be reproducible or the non-reproducible input must be documented.
- Every README install/upgrade command must be executed and verified.

## Observed Facts (Repository and Environment)

| Fact | Evidence | Consequence |
| --- | --- | --- |
| Public repo URL returns 404 | `curl https://github.com/jadmadi/swapaudit` → 404; `gh repo list jadmadi` has no swapaudit | No public source, release, badge, or `go install` path |
| Configured git remote is a different repo | `git remote -v` → `https://github.com/jadmadi/scripts.git`; that URL is also 404 publicly | The project is not a standalone repo |
| Project directory is untracked | `git status --short .` → `?? ./`; `git ls-tree -r HEAD` has no swapaudit path; `git log -- active/swapaudit` empty | No standalone history, backup, or review |
| No tags | `git tag` → empty; `git describe --tags --always` → `b89ed95` | No release version; local binary reports a commit hash |
| Local binary is built | `./swapaudit --version` → `swapaudit 1a8479e (commit: 1a8479e, built: 2026-10-19T10:53:08Z)`; `file` → statically linked x86-64; size 6.6 MB | The build works locally; version metadata is commit-based, not release-based |
| Go module path | `go.mod:1` → `module github.com/jadmadi/swapaudit` | The module path matches the intended public repo; `go install` would work once the repo/tag exists |
| GoReleaser builds four arches | `.goreleaser.yaml:13-21` → amd64, arm64, arm, riscv64; `goarm: 7` | The build matrix is already present |
| GoReleaser binary name uses `.Arch` | `.goreleaser.yaml:51` → `swapaudit-{{ .Os }}-{{ .Arch }}` | ARM is named `swapaudit-linux-arm`, not `armv7` |
| Updater expects `armv7` | `internal/updater/updater.go:409-413` | ARMv7 upgrade would 404 even after a release |
| UPX is enabled | `.goreleaser.yaml:30-36` | Packed binaries complicate reproducibility and may trigger endpoint tooling |
| Checksums are generated | `.goreleaser.yaml:53-54` | SHA256 exists; no signing step |
| No signing step | `grep -R "cosign\|minisign\|gpg" .goreleaser.yaml .github/workflows` → no matches | Release cannot be called signed |
| Release workflow triggers on `v*` tags | `.github/workflows/release.yml:1-4` | The pipeline exists; it has never run publicly |
| CI runs lint, race, and cross-compile | `.github/workflows/ci.yml` | Local `go test -race ./...` passes; no public CI badge |
| Local toolchain is Go 1.27.2 | `go version` | Release CI pins Go 1.23; local results may differ slightly |
| `strace` is available | `command -v strace` → `/usr/bin/strace` | Network/privacy verification is executable on this host |
| README install paths are 404 | `README.md:124`, `:139`, `:148` | First install step fails for every documented method |
| README claims kernel 3.10+ | `README.md:151`; `internal/proc/proc.go:50` uses `MemAvailable` (3.14+) | The claim is wrong on 3.10–3.13 |
| README implies `/var/tmp` cleanup | `README.md:112`; `internal/cache/cache.go:37` scans only `/tmp` | The claim is broader than the implementation |
| Goal audit SHAs do not resolve | `git cat-file -t` fails for `fc6640d…`, `3854d3b…`, `9170277…` | Do not publish the goal receipts unless re-grounded in the new repo |
| Business review exists locally | `docs/business-reviews/2026-10-19-swapaudit-full-review.md` | Contains internal commercial analysis; exclude unless approved |
| `.gitignore` excludes binary/dist/coverage | `.gitignore` | Reuse and extend for the standalone repo |
| Existing CI actions use version tags | `.github/workflows/*.yml` → `actions/checkout@v4`, etc. | Pin to commit SHAs for supply-chain hardening |

## External Facts

- GoReleaser `.Arch` is `GOARCH`; `.Arm` is `GOARM`. `.Arch` alone does not encode `armv7`.
- GoReleaser supports Go template conditionals in `name_template`, so `{{ if eq .Arch "arm" }}v7{{ end }}` produces the ARMv7 suffix.
- Go module proxy indexing is asynchronous after a tag is pushed; `@latest` may take a short time to resolve.
- GitHub Private Vulnerability Reporting is available at `https://github.com/{owner}/{repo}/security/advisories/new` after the repo is public and the feature is enabled.
- Cosign keyless signing requires an OIDC identity and `id-token: write`; the release workflow already has `id-token: write`.
- GitHub Actions pinned to major tags can change underneath the workflow; full commit SHAs are the reproducible choice.
- `SOURCE_DATE_EPOCH` is the conventional reproducible-build timestamp variable.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| The intended public repo is `github.com/jadmadi/swapaudit` | It is the module path, README URL, updater default, and badge URL | Confirm with the maintainer before any remote action |
| The public repo will be under the maintainer's personal namespace | The current module path uses `jadmadi` | Confirm; if an organization is required, change the module path and all imports |
| The four raw binaries are the only architecture assets | The updater downloads raw binaries by name | Check the GoReleaser config and release asset list |
| Cosign keyless is acceptable | It avoids storing a long-lived signing key | Confirm the signing identity and OIDC provider; fall back to minisign if needed |
| The maintainer can monitor the release and security channels | A release without support is a trust risk | Confirm the contact model in `security-contact-consolidation` |
| No external users exist yet | The repo has never been public | Confirm via analytics/issue history; if users exist, treat the first release as a migration |

## Unknowns

- The confirmed owner/namespace and repository visibility. **Unknown.** Consent gate required.
- Whether the maintainer wants `goals/`, `docs/business-reviews/`, `AGENTS.md`, or agent configs public. **Unknown.** Explicit decision required.
- Whether cosign keyless or minisign will be used. **Unknown.** Signing choice required.
- Whether the Go module proxy will index the tag immediately. **Unknown.** Use a bounded retry.
- Whether the armv7 binary runs on the target devices. **Unknown.** Cross-compile is not execution; test on real hardware or qemu if available.
- Whether `riscv64` is a real target for the audience. **Unknown.** Keep it because the build matrix already supports it; document it as best-effort.
- Whether GitHub Actions minutes/billing are available for the public repo. **Unknown.** Confirm before tagging.

## Asset-Name Trace

```text
Current:
  runtime.GOARCH == "arm" → updater expects "swapaudit-linux-armv7"
  GoReleaser .Arch == "arm" → emits "swapaudit-linux-arm"
  Result: 404 on ARMv7 upgrade

Required:
  runtime.GOARCH == "arm" → assetNameFor("arm") → "swapaudit-linux-armv7"
  GoReleaser .Arch == "arm" + template suffix "v7" → "swapaudit-linux-armv7"
  Result: updater and release agree
```

## Release Contents Decision Matrix

| Content | Publish? | Reason |
| --- | --- | --- |
| `cmd/`, `internal/`, `go.mod` | Yes | Product source |
| `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md` | Yes | User/contributor docs |
| `GOVERNANCE.md`, `MAINTAINERS.md`, `SECURITY.md`, `SUPPORT.md`, `PRIVACY.md`, `CODE_OF_CONDUCT.md` | Yes | Trust and community policy |
| `LICENSE`, `LICENSE-AGPL.txt`, `WaqfDPL-1.0.md` | Yes | Legal terms |
| `Makefile`, `build.sh`, `scripts/` | Yes | Build and verification |
| `.github/workflows/`, `.github/ISSUE_TEMPLATE/`, `.github/PULL_REQUEST_TEMPLATE.md` | Yes | CI, release, support |
| `.goreleaser.yaml` | Yes | Release configuration |
| `docs/safety.md`, `docs/release.md`, `docs/releases/v0.1.0.md`, `docs/decisions/` | Yes | Operator/trust docs |
| `goals/` | Conditional | Only if audit receipts reference commits that exist in the new repo; otherwise exclude |
| `docs/business-reviews/` | No (default) | Internal commercial analysis; publish only with explicit approval |
| `.sila/`, `goals/*/claim.json` | No | Local tool state |
| `.cursor/`, `.devin/`, `.windsurfrules`, `.github/copilot-instructions.md` | No (default) | Local agent configuration; sanitize/exclude unless approved |
| `swapaudit` binary, `dist/`, `build/`, `coverage.*` | No | Build artifacts |

## Evidence Commands

```bash
git remote -v
git tag
git describe --tags --always
git status --short .
git ls-tree -r HEAD --name-only | grep -i swapaudit || true
curl -s -o /dev/null -w '%{http_code}\n' https://github.com/jadmadi/swapaudit
curl -s -o /dev/null -w '%{http_code}\n' https://proxy.golang.org/github.com/jadmadi/swapaudit/@latest
grep -n "goarch\|goarm\|name_template\|upx\|checksum" .goreleaser.yaml
grep -n "assetName\|GOARCH\|armv7\|releases/download" internal/updater/updater.go
grep -n "releases/latest\|go install\|git clone\|kernel 3.10" README.md
go version
command -v strace
go test -race ./...
```

## File & Interface Contracts

- `.goreleaser.yaml`: four builds; ARMv7 name suffix; no UPX; checksums; signing.
- `internal/updater/updater.go`: `assetNameFor(goarch string) string`; `Upgrade` uses it.
- `internal/version/version.go`: version/commit/date ldflags.
- `.github/workflows/ci.yml`: lint, vet, race, cross-compile, goreleaser check, snapshot asset assertion.
- `.github/workflows/release.yml`: test gate, release, post-release verification.
- `scripts/verify-release.sh`: end-to-end release verification.
- `scripts/repro-build.sh`: deterministic build check.
- `docs/release.md`: runbook, settings, rollback.
- `docs/releases/v0.1.0.md`: release body.
- `README.md`: install/verify/upgrade/known limitations.
