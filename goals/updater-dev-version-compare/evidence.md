# Evidence: Compare git-describe dev builds as newer than their base tag

## Symptom
Running a `git describe` build (`v0.1.0-59-g2c13a5e`) printed:

```
A new release of swapaudit is available: v0.1.0-59-g2c13a5e -> v0.1.0
```

`upgrade` on the installed release binary reported `already up to date
(0.1.0)`. The dev build is 59 commits **after** v0.1.0, so the notice was
false and upgrading would have downgraded the binary.

## Root Cause
`parseSemVer` split on the first `-` and treated `59-g2c13a5e` as a semver
prerelease. `CompareVersions` then ranked the release tag above the snapshot.

## Verification
- `go test -race ./internal/updater ./internal/app ./internal/term` — pass.
- `go test -race ./...` — pass.
- `go vet ./...` — pass.
- End-to-end, dev binary built with
  `-X .../version.Version=v0.1.0-59-g2c13a5e`:
  - cached `latest_version: v0.1.0` -> no update banner.
  - cached `latest_version: v0.2.0` -> banner shown.

Verdict: PASS
