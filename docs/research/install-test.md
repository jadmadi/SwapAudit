# Install test

Date: 2026-10-19
Release: private `v0.1.0` in `jadmadi/SwapAudit`

## Private release smoke test

The published amd64 binary was run in two clean containers:

```text
Debian bookworm-slim:
  swapaudit 0.1.0 (commit: faaa762, built: 2026-10-19T19:34:53Z)

Alpine 3.20:
  swapaudit 0.1.0 (commit: faaa762, built: 2026-10-19T19:34:53Z)

Debian bookworm-slim audit smoke:
  header rendered
  "OK nothing to reclaim" for the disposable container
```

The binary is static and runs on both glibc and musl environments.

## Checksums and signature

- Four architecture assets present.
- `sha256sum -c checksums.txt --ignore-missing` passes for all four.
- Cosign keyless signature verified in the release workflow and by `scripts/verify-release.sh` with cosign v2.4.1.
- Tamper test: appending one byte to `swapaudit-linux-amd64` makes `sha256sum -c` fail.

## Public install test

**Not performed.** The repository is private, and public visibility, a public announcement, and public download measurement require maintainer approval.

Blocked items:

- Public unauthenticated download of the four assets.
- Public `go install` and `swapaudit upgrade`.
- Announcement in a community channel.
- 14-day download/issue/reply measurement.
- Five target-user interviews.

## Next steps

1. Maintainer decides whether the repository becomes public.
2. If public, run `scripts/install-smoke.sh` from a clean environment and publish one appropriate announcement.
3. Record asset download counts at day 1, 7, and 14 (proxy metric).
4. Complete five interviews and write `decision.md`.
