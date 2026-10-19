# Consent record: standalone-v0-1-0-release

Date: 2026-10-19

## Approved

- Repository: `jadmadi/SwapAudit`
- Visibility: **private**
- Source import and push to `main`
- Annotated tag `v0.1.0`
- Private GitHub release with four Linux assets
- SHA256 checksums and cosign keyless signatures
- Local and authenticated verification (`gh release download`, `go install`, checksums)

## Not approved / not performed

- Changing repository visibility to public
- Public unauthenticated release downloads
- A public install test or public announcement
- Five user interviews
- Publishing internal business-review notes or changing the namespace

## Signing

- cosign keyless via GitHub Actions OIDC
- cosign v2.4.1 pinned in the release workflow
- Signature assets: `checksums.txt.sig`, `checksums.txt.pem`

## Rollback

- Prefer `v0.1.1` for a broken release.
- Do not overwrite assets.
- Delete the release/tag only within 24 hours, only if no external installs exist, and only with written approval.
