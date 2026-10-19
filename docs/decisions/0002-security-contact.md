# Decision 0002: Security, support, and conduct contacts

<!-- contact-sentry
canonical_email: contact@jadmadi.net
canonical_repo: jadmadi/SwapAudit
canonical_security_url: https://github.com/jadmadi/SwapAudit/security/advisories/new
-->

- **Decision**: one canonical fallback email plus GitHub private advisories
- **Approver**: Project maintainer
- **Date**: 2026-10-19
- **Status**: Accepted

## Context

The repository published two different addresses:

- `SECURITY.md`: `jad@madi.me`
- `MAINTAINERS.md`: `contact@jadmadi.net`

`SECURITY.md` also pointed to GitHub Private Vulnerability Reporting on a repository that did not exist publicly at the time. `CODE_OF_CONDUCT.md` had no concrete reporting channel. There was no `SUPPORT.md` and no issue template.

## Decision

| Purpose | Primary | Fallback |
| --- | --- | --- |
| Security vulnerability | GitHub Private Vulnerability Reporting: `https://github.com/jadmadi/SwapAudit/security/advisories/new` | `contact@jadmadi.net` |
| General support | GitHub Issues using the templates | `contact@jadmadi.net` |
| Conduct report | `contact@jadmadi.net` | GitHub repository report form |
| Release/security coordination | Project Lead (`@jadmadi`) | `contact@jadmadi.net` |

`contact@jadmadi.net` is the canonical email because it is already published in `MAINTAINERS.md` and is the address the maintainer has designated for project contact. `jad@madi.me` is retired from the published security path. If it remains active as a forwarding alias, it must be documented here; it must not be republished as a primary channel.

The GitHub advisory URL is staged for the public repository. It is valid for authorized collaborators while the repository is private and becomes the public primary path when the repository is made public. The release goal verifies it after publication.

## Response expectations

- Acknowledge receipt within **48 hours**.
- Aim to provide an initial assessment within **5 business days** (target, not a contractual SLA).
- Fix timing depends on severity and maintainer availability. No fix date is promised.
- Supported versions: the latest tagged release (0.1.x when published). Pre-release/source builds are best-effort.

## Monitoring

The canonical email is checked at least weekly. Security reports are prioritized over general support. If the address changes, set up forwarding before removing the old address and update this record, `SECURITY.md`, `MAINTAINERS.md`, `SUPPORT.md`, and `CODE_OF_CONDUCT.md` in the same change.

## Alternatives considered

- **Keep `jad@madi.me`**: rejected because `MAINTAINERS.md` publishes a different contact and the security page should not introduce a second address.
- **Invent `security@jadmadi.net`**: rejected because there is no evidence that the mailbox exists or is monitored.
- **Use only GitHub advisories**: rejected because users may not have repository access while the repo is private, and a fallback is needed for disclosure.
