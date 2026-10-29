# Release runbook

## Consent and visibility

The repository is **public** as of 2026-10-19. The release goal's consent record is `goals/standalone-v0-1-0-release/consent.md`.

Public release assets, `go install`, and `swapaudit upgrade` are verified without authentication. Changing visibility or publishing under a different namespace still requires maintainer approval.

## Preflight

```bash
gofmt -l .
go vet ./...
go test -race ./...
./scripts/check.sh --full
./scripts/repro-build.sh
goreleaser check
goreleaser release --snapshot --clean --skip=publish --skip=sign
```

Assert the four upload names in `dist/artifacts.json` (GoReleaser keeps raw binaries in per-target subdirectories and applies the upload name via `name_template`):

```text
swapaudit-linux-amd64
swapaudit-linux-arm64
swapaudit-linux-armv7
swapaudit-linux-riscv64
```

```bash
for name in swapaudit-linux-amd64 swapaudit-linux-arm64 swapaudit-linux-armv7 swapaudit-linux-riscv64; do
  grep -Fq "\"name\":\"$name\"" dist/artifacts.json || { echo "missing $name"; exit 1; }
done
```

Do not tag if any upload name is missing or if the snapshot fails.

## Tag and publish

```bash
git tag -a v0.1.0 -m "SwapAudit v0.1.0"
git push origin v0.1.0
```

The tag triggers `.github/workflows/release.yml`:

1. **test**: `go test -race ./...`.
2. **goreleaser**: builds four Linux binaries, creates checksums, and signs `checksums.txt` with cosign keyless.
3. **verify**: downloads the assets, verifies checksums, runs the amd64 binary, and verifies the cosign signature.

## Post-release verification

```bash
./scripts/verify-release.sh v0.1.0
```

If `cosign` is not installed locally, the script still checks the signature assets and prints a warning; install cosign to perform full verification. The script uses the public release URLs and unauthenticated `go install`.

## Rollback

- Prefer publishing `v0.1.1`; do not overwrite assets.
- Delete the release/tag only within 24 hours, only if no external installs are confirmed, and only with a written reason.
- If signing is compromised, revoke the certificate/key, publish an advisory, and release a patch.

## Repository settings

After publication:

- Enable Issues and Discussions (done).
- Enable Private Vulnerability Reporting and verify the URL in `SECURITY.md`.
- Enable secret scanning and Dependabot alerts.
- Protect `main` with required CI checks.
- Protect `v*` tags.

Record the settings and verification output in `goals/standalone-v0-1-0-release/evidence.md`.

## Asset contract

| Architecture | Asset |
| --- | --- |
| amd64 | `swapaudit-linux-amd64` |
| arm64 | `swapaudit-linux-arm64` |
| armv7 | `swapaudit-linux-armv7` |
| riscv64 | `swapaudit-linux-riscv64` |

`checksums.txt`, `checksums.txt.sig`, and `checksums.txt.pem` accompany the four binaries when signing is enabled.
