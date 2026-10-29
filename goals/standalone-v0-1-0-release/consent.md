# Consent record: standalone-v0-1-0-release

Date: 2026-10-19

## Approved

- Repository: `jadmadi/SwapAudit`
- Visibility: **public** (approved 2026-10-19)
- Source import and push to `main`
- Annotated tag `v0.1.0`
- Public GitHub release with four Linux assets
- SHA256 checksums and cosign keyless signatures
- Public install, `go install`, and live `swapaudit upgrade`
- Public announcement and tweet drafts (drafts only; posting requires review)

## Not approved / not performed

- Five user interviews
- A public announcement posted without review
- 14-day download/issue measurement
- Changing namespace or repository name
- Deleting the internal business review or goal ledger

## Signing

- cosign keyless via GitHub Actions OIDC
- cosign v2.4.1 pinned in the release workflow
- Signature assets: `checksums.txt.sig`, `checksums.txt.pem`

## Public verification

- Repository URL returns HTTP 200 without authentication.
- All four assets and checksum/signature files are publicly reachable.
- `scripts/verify-release.sh v0.1.0` passes, including cosign verification and `go install`.
- Live `swapaudit upgrade` from v0.0.1 reaches v0.1.0 and verifies the checksum.

## Rollback

- Prefer `v0.1.1` for a broken release.
- Do not overwrite assets.
- Delete the release/tag only within 24 hours, only if no external installs exist, and only with written approval.
