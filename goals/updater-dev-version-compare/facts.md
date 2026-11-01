# Facts: Compare git-describe dev builds as newer than their base tag

## Architectural Invariants & Constraints
- `git describe --tags --always --dirty` emits `<tag>-<commits>-g<sha>[-dirty]`.
- That form describes a build **after** the tag; it is not a semver prerelease.
- Real semver prereleases such as `v1.0.0-rc.1` remain older than their release.
- The `-dirty` marker must not change ordering against the base tag.
- Update checks must never advertise the base tag for a snapshot ahead of it.

## File & Interface Contracts
- Relevant files: `internal/updater/updater.go`, `internal/updater/updater_test.go`.
- `parseSemVer` returns `(major, minor, patch, prerelease, commitsAhead, ok)`.
- `CompareVersions` orders equal triples by prerelease, then commit distance.
- `IsNewer` keeps non-semver builds (`dev`, `none`, raw SHAs) upgradeable.
