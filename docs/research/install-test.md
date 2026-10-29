# Install test

Date: 2026-10-19
Release: public `v0.1.0` in `jadmadi/SwapAudit`

## Public install test

The release is public. These checks ran without GitHub authentication:

```text
$ curl -fsSL .../swapaudit-linux-amd64 -o swapaudit-linux-amd64
$ curl -fsSL .../checksums.txt -o checksums.txt
$ sha256sum -c checksums.txt --ignore-missing
swapaudit-linux-amd64: OK
swapaudit-linux-arm64: OK
swapaudit-linux-armv7: OK
swapaudit-linux-riscv64: OK

$ ./swapaudit-linux-amd64 --version
swapaudit 0.1.0 (commit: faaa762, built: 2026-10-19T19:34:53Z)

$ ./scripts/install-smoke.sh v0.1.0
Install smoke passed for v0.1.0.

$ go install github.com/jadmadi/swapaudit/cmd/swapaudit@v0.1.0
swapaudit v0.1.0 (commit: release, built: unknown)

$ ./swapaudit-old upgrade   # v0.0.1 build
Upgrading swapaudit v0.0.1 -> v0.1.0...
Checksum verified successfully.
✔ Successfully upgraded swapaudit to v0.1.0
```

The published amd64 binary was also run in two clean containers:

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

## Still outstanding

- Five target-user interviews.
- One public announcement (draft prepared; not posted).
- Asset download counts at day 1, 7, and 14 (proxy metric).
- A dated decision in `decision.md`.

## Next steps

1. Review and post the announcement using `docs/marketing/x-launch.md`.
2. Record asset download counts at day 1, 7, and 14.
3. Complete five interviews and write `decision.md`.
