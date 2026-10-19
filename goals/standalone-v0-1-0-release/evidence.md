# Evidence: standalone-v0-1-0-release

Date: 2026-10-19
Repository: `github.com/jadmadi/SwapAudit` (public as of 2026-10-19)
Tag: `v0.1.0` at `faaa762`
Release: `SwapAudit 0.1.0`, published 2026-10-19T19:38:10Z
Release workflow: run `39312241427` — **success**
CI run before tag: `39311854804` — **success** (including Release Config & Asset Names)

## Consent and visibility

The user requested a private repository initially, then explicitly approved the public release on 2026-10-19. The repository is now **public**.

Public visibility, public unauthenticated install, `go install`, and live `swapaudit upgrade` were verified. Five user interviews and the 14-day public measurement remain separate work in `public-install-demand-validation`.

## Release assets

```text
checksums.txt                 760 B
checksums.txt.pem            3252 B
checksums.txt.sig              96 B
swapaudit-linux-amd64      5648536 B
swapaudit-linux-arm64      5439640 B
swapaudit-linux-armv7      5505176 B
swapaudit-linux-riscv64    5308568 B
SwapAudit_0.1.0_linux_amd64.tar.gz
SwapAudit_0.1.0_linux_arm64.tar.gz
SwapAudit_0.1.0_linux_armv7.tar.gz
SwapAudit_0.1.0_linux_riscv64.tar.gz
```

The four binary upload names match the updater contract:
`swapaudit-linux-amd64`, `swapaudit-linux-arm64`, `swapaudit-linux-armv7`, `swapaudit-linux-riscv64`.

## Release workflow

```text
$ gh run view 39312241427 -R jadmadi/SwapAudit
Jobs:
  Test before release        success
  Run GoReleaser             success (1m40s)
  Verify release assets      success (32s)
    - Install cosign         success
    - Download assets        success
    - Verify checksums       success
    - Verify signature       success
```

The workflow signs `checksums.txt` with cosign keyless (`cosign v2.4.1`) and verifies the signature in the post-release job.

## Public release verification

```text
$ curl -s -o /dev/null -w '%{http_code}' https://github.com/jadmadi/SwapAudit
200

$ curl -sL -o /dev/null -w '%{http_code}' .../releases/download/v0.1.0/swapaudit-linux-amd64
200

$ ./scripts/verify-release.sh v0.1.0
PASS: four architecture assets are publicly reachable
PASS: checksums verified
PASS: amd64 binary reports 0.1.0
PASS: signature verified
PASS: go install @v0.1.0
Release verification passed for v0.1.0.

$ ./scripts/install-smoke.sh v0.1.0
Install smoke passed for v0.1.0.

$ ./swapaudit-old upgrade   # v0.0.1 build
Upgrading swapaudit v0.0.1 -> v0.1.0...
Checksum verified successfully.
✔ Successfully upgraded swapaudit to v0.1.0
```

The published amd64 binary ran in Debian bookworm-slim and Alpine 3.20 containers.

## Independent release verification

```text
$ ./scripts/verify-release.sh v0.1.0
PASS: four architecture assets present
swapaudit-linux-amd64: OK
swapaudit-linux-arm64: OK
swapaudit-linux-armv7: OK
swapaudit-linux-riscv64: OK
PASS: checksums verified
PASS: amd64 binary reports 0.1.0
Verified OK
PASS: signature verified
swapaudit v0.1.0 (commit: release, built: unknown)
PASS: go install @v0.1.0
Release verification passed for v0.1.0.
```

The release binary reports `0.1.0` (GoReleaser strips the tag's `v`); the verifier accepts both `v0.1.0` and `0.1.0`. The tag itself is `v0.1.0`.

## Build preflight

```text
$ goreleaser check
1 configuration file(s) validated

$ goreleaser release --snapshot --clean --skip=publish --skip=sign
release succeeded
artifacts.json contains:
  swapaudit-linux-amd64
  swapaudit-linux-arm64
  swapaudit-linux-armv7
  swapaudit-linux-riscv64

$ ./scripts/repro-build.sh
reproducible: yes (8b902a6231d4a7ed74c4287f74aa2eb1f53febeeaf03b0d7188ec4030d29d234)
```

## Upgrade path

The live private release cannot be used by `swapaudit upgrade` because the updater uses unauthenticated GitHub URLs and the repository is private. The upgrade path is covered by `TestUpgradeEndToEnd`, which serves a fake release through a local HTTP server, verifies the checksum, replaces a temporary executable, and asserts the new binary content. Public live upgrade is blocked on the repository becoming public.

## Files changed

- `.goreleaser.yaml` — four assets, ARMv7 suffix, no UPX, commit-date metadata, checksums, cosign signing, release notes.
- `internal/updater/updater.go`, `internal/updater/updater_test.go` — `assetNameFor`, ARMv7 contract, end-to-end upgrade test.
- `.github/workflows/ci.yml` — release-config job and four-name assertion.
- `.github/workflows/release.yml` — test gate, cosign keyless signing, post-release verification.
- `.github/dependabot.yml` — weekly Actions updates.
- `Makefile`, `scripts/build` — `SOURCE_DATE_EPOCH` support.
- `scripts/repro-build.sh`, `scripts/verify-release.sh` — reproducible-build and release verification.
- `docs/release.md`, `docs/releases/v0.1.0.md`, `README.md`, `CHANGELOG.md`.

## Deviations

1. **Private release, not public.** The user explicitly requested a private repository. Public visibility and unauthenticated public install were not approved.
2. **Live private `swapaudit upgrade` is unsupported.** The updater does not send GitHub auth. The local release-server test covers the upgrade logic.
3. **`go install` requires authentication** and version ldflags; the verifier injects the release version and asserts `v0.1.0`.
4. **No public install test.** That work belongs to `public-install-demand-validation` and is blocked on the repository becoming public plus five interviews.
