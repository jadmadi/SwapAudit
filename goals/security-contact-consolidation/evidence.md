# Evidence: security-contact-consolidation

Date: 2026-10-19
Decision record: `docs/decisions/0002-security-contact.md`
Canonical email: `contact@jadmadi.net`
Canonical security URL: `https://github.com/jadmadi/SwapAudit/security/advisories/new`
Canonical repo: `jadmadi/SwapAudit`

## Contact model

| Purpose | Primary | Fallback |
| --- | --- | --- |
| Security vulnerability | GitHub Private Vulnerability Reporting | `contact@jadmadi.net` |
| General support | GitHub Issues with templates | `contact@jadmadi.net` |
| Conduct | `contact@jadmadi.net` | GitHub report form |

The retired `jad@madi.me` address is removed from the published security path. It may appear only in the decision record's migration note and the changelog's historical note.

## Contact sentry

```text
$ ./scripts/check-contacts.sh .
Contact sentry clean: contact@jadmadi.net; https://github.com/jadmadi/SwapAudit/security/advisories/new

$ ./scripts/check-contacts-test.sh
PASS: consistent fixture
PASS: inconsistent fixture rejected
PASS: missing decision fixture rejected
Contact sentry fixtures passed.
```

## Issue-form validation

A temporary PyYAML install in the session scratchpad was used only for validation; it is not a project dependency.

```text
valid: .github/ISSUE_TEMPLATE/config.yml keys= ['blank_issues_enabled', 'contact_links']
valid: .github/ISSUE_TEMPLATE/bug_report.yml keys= ['body', 'description', 'labels', 'name', 'title']
valid: .github/ISSUE_TEMPLATE/feature_request.yml keys= ['body', 'description', 'labels', 'name', 'title']
```

## Retired-address scan

```text
$ grep -RIn "jad@madi.me" --exclude-dir=.git --exclude-dir=goals \
    --exclude-dir=business-reviews --exclude="0002-security-contact.md" \
    --exclude="CHANGELOG.md" --exclude="check-contacts.sh" .
(no output)
```

The contact sentry enforces the same exclusion set and fails if the retired address reappears in the published surface.

## Quality gate

```text
$ ./scripts/check.sh --full
✔ Diff sentry clean.
✔ Claims sentry clean.
✔ Privacy sentry clean.
✔ Contact sentry clean.
✔ All Go files formatted.
✔ go vet passed cleanly.
✔ Race-detected tests passed.
✔ Built binary self-verified.
✔ All checks passed.
```

## GitHub URL status

The repository is private. The advisory URL is staged and valid for authorized collaborators; it is not publicly reachable until the repository is made public. The `standalone-v0-1-0-release` goal owns the post-publication URL check and the repository setting verification.

## Files changed

- `SECURITY.md`
- `MAINTAINERS.md`
- `CODE_OF_CONDUCT.md`
- `CONTRIBUTING.md`
- `README.md`
- `SUPPORT.md` (new)
- `docs/decisions/0002-security-contact.md` (new)
- `.github/ISSUE_TEMPLATE/bug_report.yml` (new)
- `.github/ISSUE_TEMPLATE/feature_request.yml` (new)
- `.github/ISSUE_TEMPLATE/config.yml` (new)
- `.github/PULL_REQUEST_TEMPLATE.md` (new)
- `scripts/check-contacts.sh` (new)
- `scripts/check-contacts-test.sh` (new)
- `scripts/check.sh` (contact sentry)
- `CHANGELOG.md`

## Deviations

- The GitHub advisory URL cannot be publicly verified while the repository is private. The release goal owns that check.
- The canonical email is the existing `contact@jadmadi.net`; no new mailbox was invented.
- The conduct channel uses the same canonical email because no dedicated conduct mailbox exists.
