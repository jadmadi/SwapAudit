# Evidence: public-install-demand-validation

Date: 2026-10-19
Release: public `v0.1.0` in `jadmadi/SwapAudit`

## Public install test

The public install path was verified without GitHub authentication:

```text
$ curl -s -o /dev/null -w '%{http_code}' https://github.com/jadmadi/SwapAudit
200

$ ./scripts/install-smoke.sh v0.1.0
Install smoke passed for v0.1.0.

$ ./scripts/verify-release.sh v0.1.0
PASS: four architecture assets are publicly reachable
PASS: checksums verified
PASS: amd64 binary reports 0.1.0
PASS: signature verified
PASS: go install @v0.1.0
Release verification passed for v0.1.0.

$ go install github.com/jadmadi/swapaudit/cmd/swapaudit@v0.1.0
swapaudit v0.1.0 (commit: release, built: unknown)

$ ./swapaudit-old upgrade   # v0.0.1 build
Upgrading swapaudit v0.0.1 -> v0.1.0...
Checksum verified successfully.
✔ Successfully upgraded swapaudit to v0.1.0
```

The published amd64 binary also ran in Debian bookworm-slim and Alpine 3.20 containers. Full details: `docs/research/install-test.md`.

## Still outstanding

- Five consenting target-user interviews (`2026-10-19-interviews.md` not created).
- One public announcement (draft in `docs/marketing/x-launch.md`; not posted).
- Asset download counts at day 1, 7, and 14.
- Issue/reply inventory.
- Dated `docs/research/decision.md` with Proceed/Narrow/Stop/Inconclusive.
- Lifting or keeping the feature freeze based on that decision.

## Notes

- The public install test satisfies the distribution half of this goal.
- Demand itself is still unvalidated; five interviews are required before the decision rules can be applied.
- The release asset download counts are not a unique-user metric and may include mirrors or repeated downloads.
