# Facts: Consolidate Security, Support, and Vulnerability Reporting Contacts

## Architectural Invariants & Constraints

- One canonical email per purpose; no two addresses for the same purpose.
- Security reports must never be routed to a public issue.
- Do not invent a mailbox; use an existing published address unless the maintainer confirms a new one.
- Do not promise a 24/7 response, bug bounty, or commercial SLA.
- Do not publish personal data beyond the confirmed contact address.
- The GitHub private-reporting URL is staged for the release goal; this goal does not create the public repo.
- The contact sentry must be deterministic, fixture-testable, and wired into `scripts/check.sh --full`.
- No product code changes.

## Observed Facts (Repository)

| Fact | Evidence | Consequence |
| --- | --- | --- |
| Security contact is `jad@madi.me` | `SECURITY.md:16` | Conflicts with the maintainer contact |
| Security policy points to GitHub Private Vulnerability Reporting "on the repository" | `SECURITY.md:16` | The repository is 404, so the primary path is unusable |
| Maintainer contact is `contact@jadmadi.net` | `MAINTAINERS.md:13` | Two different addresses for the same project |
| Conduct enforcement has no address | `CODE_OF_CONDUCT.md` "Enforcement" section | Reports have no concrete destination |
| No `SUPPORT.md` | Repository root listing; `README.md` links only CONTRIBUTING/GOVERNANCE/SECURITY | Users have no support policy |
| No issue templates | `.github/` contains workflows and `copilot-instructions.md` only | Bug reports lack version/OS/repro fields |
| No PR template | `.github/` listing | Contributors lack a verification checklist |
| `CONTRIBUTING.md` links governance and maintainers but not support/security | `CONTRIBUTING.md` | New contributors do not see the reporting paths |
| README links `SECURITY.md` and `GOVERNANCE.md` | `README.md:332-334` | Security is discoverable, but the contact is inconsistent |
| Secret scanning/diff sentry exists | `scripts/check.sh` "Diff Sentry" | A contact sentry can follow the same pattern |
| No public repository | `curl https://github.com/jadmadi/swapaudit` → 404; `gh repo list jadmadi` has no swapaudit | The GitHub private-reporting link cannot work yet |
| Goal audit SHAs do not resolve | `git cat-file -t` fails for `fc6640d…`, `3854d3b…`, `9170277…` | The repository is not a trustworthy public source yet; the contact goal must not imply otherwise |

## External Facts

- GitHub Private Vulnerability Reporting is available at `https://github.com/{owner}/{repo}/security/advisories/new` once the repository is public and the feature is enabled.
- GitHub issue forms use YAML under `.github/ISSUE_TEMPLATE/`; `config.yml` controls contact links and blank issues.
- GitHub's `security/advisories/new` URL returns 404 for a nonexistent or private repository without access.
- A 48-hour acknowledgment is the only response commitment currently published; no other SLA exists in the repo.
- Email forwarding/aliases are provider-specific and cannot be verified from the repository.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| `contact@jadmadi.net` is monitored | It is the published maintainer contact | Send a test message and confirm receipt; record the date |
| `jad@madi.me` is either an alias or obsolete | It is published in SECURITY.md but conflicts with MAINTAINERS.md | Ask the maintainer; if it is an alias, keep it only as a documented forwarding path |
| `contact@jadmadi.net` is acceptable for conduct reports | No dedicated conduct mailbox exists | Ask the maintainer; if not, create a dedicated alias or use a GitHub report form |
| GitHub private reporting will be enabled after release | It is the preferred primary channel | Release goal verifies the URL and repository setting |
| A 48-hour acknowledgment is sustainable | It is already published | Track the first three reports; adjust only with a documented decision |

## Unknowns

- Which address the maintainer actually monitors. **Unknown.** The decision record must resolve this.
- Whether `jad@madi.me` and `contact@jadmadi.net` are aliases of the same inbox. **Unknown.** Ask the maintainer.
- Whether a dedicated `security@` or `conduct@` alias exists. **Unknown.** Do not invent one.
- Whether GitHub Discussions will be enabled. **Unknown.** `SUPPORT.md` can say "if enabled."
- Whether the public repository will be under `jadmadi` or an organization. **Unknown.** The release goal confirms the namespace; the security link must use the confirmed repo.

## Current Contact Surface

| Surface | Current text | Problem |
| --- | --- | --- |
| `SECURITY.md` | `jad@madi.me` or GitHub private reporting | Address conflicts with MAINTAINERS; GitHub link 404s |
| `MAINTAINERS.md` | `contact@jadmadi.net` | Different address; no security role note |
| `CODE_OF_CONDUCT.md` | "report to the project maintainers" | No concrete channel |
| `CONTRIBUTING.md` | Links governance/maintainers | No support/security link |
| `README.md` | Links `SECURITY.md` and `GOVERNANCE.md` | Security contact is wrong at the destination |
| `.github/ISSUE_TEMPLATE/` | Does not exist | No structured bug/feature reports |
| `.github/PULL_REQUEST_TEMPLATE.md` | Does not exist | No verification checklist |

## Evidence Commands

```bash
grep -n "jad@madi.me\|contact@jadmadi.net\|security/advisories" SECURITY.md MAINTAINERS.md CODE_OF_CONDUCT.md CONTRIBUTING.md README.md
find .github -maxdepth 2 -type f -print
grep -n "Diff Sentry\|check.sh" scripts/check.sh | head
curl -s -o /dev/null -w '%{http_code}\n' https://github.com/jadmadi/swapaudit
curl -s -o /dev/null -w '%{http_code}\n' https://github.com/jadmadi/swapaudit/security/advisories/new
```

## File & Interface Contracts

- `docs/decisions/0002-security-contact.md`: human decision record plus a machine-readable `contact-sentry` header.
- `SECURITY.md`: canonical policy; contains the `contact-sentry` header if the decision record does not.
- `SUPPORT.md`: support routing and expectations.
- `CODE_OF_CONDUCT.md`: conduct reporting channel.
- `.github/ISSUE_TEMPLATE/*.yml`: valid YAML issue forms.
- `.github/PULL_REQUEST_TEMPLATE.md`: markdown checklist.
- `scripts/check-contacts.sh`: deterministic sentry; accepts a root argument for fixture tests.
- `scripts/check.sh`: invokes the sentry in `--full` mode.
