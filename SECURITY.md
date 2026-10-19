# Security Policy

## Supported Versions

| Version | Supported |
| --- | --- |
| Latest tagged release (0.1.x when published) | :white_check_mark: |
| Older releases | Best-effort only |
| Unreleased source builds | Best-effort only |

## Reporting a Vulnerability

**Do not open a public issue for a vulnerability.**

Primary channel: [GitHub Private Vulnerability Reporting](https://github.com/jadmadi/SwapAudit/security/advisories/new).

Fallback: `contact@jadmadi.net`.

The GitHub advisory link is active for authorized collaborators while the repository is private and becomes the public primary path when the repository is published. The canonical contact decision is documented in [`docs/decisions/0002-security-contact.md`](docs/decisions/0002-security-contact.md).

Please include:

1. A description of the vulnerability and the affected component.
2. Steps to reproduce, or a proof of concept.
3. Your impact assessment.
4. `swapaudit --version`, OS/distro, kernel version, and the command used.
5. Sanitized logs. Do not include secrets, tokens, private file contents, or other users' data.

## Response Expectations

- We acknowledge receipt within **48 hours**.
- We aim to provide an initial assessment within **5 business days**. This is a target, not a contractual SLA.
- Fix timing depends on severity and maintainer availability. No specific fix date is promised.
- We will coordinate disclosure with you and credit you unless you request anonymity.

## Scope

**In scope:**

- `/proc` parsing, memory/swap attribution, and `/tmp` cache inspection.
- Process classification and cleanup safety, including UID, start-ticks, orphan, path, and symlink guards.
- The self-updater, release artifacts, checksums, and install scripts.
- Documentation claims that could cause unsafe use or data loss.

**Out of scope:**

- Vulnerabilities in third-party AI agents, language servers, or other processes that SwapAudit inspects.
- The Linux kernel, systemd, GitHub, or distribution packaging.
- A user's local configuration or shell environment.

Report out-of-scope issues to the relevant upstream project.

## Safe Harbor

Good-faith security research that follows this policy will not be pursued legally. Do not access other users' data, degrade service, or run destructive cleanup tests on systems you do not own. Use a disposable VM or container for tests that delete files or signal processes.

## Contact

- Security: GitHub Private Vulnerability Reporting (primary) or `contact@jadmadi.net` (fallback).
- General support: [`SUPPORT.md`](SUPPORT.md).
- Conduct: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) or `contact@jadmadi.net`.
