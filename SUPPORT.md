# Support

## Where to get help

| Need | Channel |
| --- | --- |
| Reproducible bug | [GitHub Issues](https://github.com/jadmadi/SwapAudit/issues/new?template=bug_report.yml) |
| Feature request | [GitHub Issues](https://github.com/jadmadi/SwapAudit/issues/new?template=feature_request.yml) |
| Security vulnerability | [Private advisory](https://github.com/jadmadi/SwapAudit/security/advisories/new) or `contact@jadmadi.net` — **never a public issue** |
| Question / usage help | GitHub Issues, or Discussions if enabled |
| Conduct report | `contact@jadmadi.net` or the repository report form |

## What to include

- `swapaudit --version`
- OS and kernel (`uname -a`)
- The exact command and flags
- Expected behavior and actual behavior
- `--dry-run` output when the issue involves cleanup
- Sanitized logs. Remove secrets, tokens, private paths, and other users' data.

## Response expectations

Support is best-effort. There is no 24/7 support, no bug bounty, and no contractual SLA. Security reports are prioritized over general support. The supported version is the latest tagged release; older versions may not receive fixes.

## Scope

SwapAudit is a Linux-only local CLI. It does not provide a hosted service, accounts, or remote support access. Do not ask maintainers to run destructive cleanup on your machine; use a disposable VM or container and share the `--dry-run` output.

## Feature requests

The project is under a feature freeze until demand validation is complete. Feature requests are welcome, but new scanners, agent runtimes, JSON output, package-manager packaging, GUI, daemon, telemetry, and pricing work are deferred. See [`goals/README.md`](goals/README.md) and [`docs/research/decision.md`](docs/research/decision.md) when the validation record exists.

## Conduct

See [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md). Report conduct issues to `contact@jadmadi.net`; reports are handled confidentially.
