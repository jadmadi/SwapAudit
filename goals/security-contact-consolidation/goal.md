# Goal: Consolidate Security, Support, and Vulnerability Reporting Contacts

## Chain of Custody (Isnad)

- **Registered By**: Business review agent, 2026-10-19
- **Spec Audited By**: CommandCode, 2026-10-20 (acceptance criteria checked against tests and evidence)
- **Implemented By**: CommandCode, 2026-10-20 (see `evidence.md`)
- **Mystery Audited By**: Evidence certification, 2026-10-20 (see `certification.json`; independent mystery audit not run)

## Goal Description

Replace the inconsistent and partly unusable security/support surface with one canonical, tested contact model:

- `SECURITY.md` says `jad@madi.me` and points to GitHub private vulnerability reporting on `github.com/jadmadi/swapaudit`, which currently returns 404.
- `MAINTAINERS.md` says `contact@jadmadi.net`.
- `CODE_OF_CONDUCT.md` says "report to the project maintainers" without an address.
- There is no `SUPPORT.md`, no issue template, and no PR template.
- There is no automated check preventing the addresses from drifting again.

The goal establishes a primary GitHub private-reporting path (active once the standalone repository exists), one canonical fallback email, a public support policy, and a contact-consistency sentry. It does **not** create the public repository; that is `standalone-v0-1-0-release`.

## Business Context

- The business review (`docs/business-reviews/2026-10-19-swapaudit-full-review.md`) ranks broken security/support contacts as **High** trust gaps (G-08).
- A vulnerability report sent to an unmonitored address, or a private-reporting link that 404s, is a trust and disclosure failure.
- A public issue tracker and support policy are prerequisites for the demand-validation interviews and the v0.1.0 release.
- The repository currently has no public issue path, so users cannot report bugs or ask for help.

## Dependencies & Execution Order

- **Mode**: Independent (disjoint, parallelizable)
- **Depends On**: none
- **Sequence**: 3
- **Shape**: ship
- **Tier**: immediate

## Files to Touch

- `SECURITY.md`
- `MAINTAINERS.md`
- `CODE_OF_CONDUCT.md`
- `CONTRIBUTING.md`
- `README.md`
- `SUPPORT.md` (new)
- `.github/ISSUE_TEMPLATE/bug_report.yml` (new)
- `.github/ISSUE_TEMPLATE/feature_request.yml` (new)
- `.github/ISSUE_TEMPLATE/config.yml` (new)
- `.github/PULL_REQUEST_TEMPLATE.md` (new)
- `scripts/check-contacts.sh` (new)
- `scripts/check.sh` (wire in the sentry)
- `docs/decisions/0002-security-contact.md` (new)
- `CHANGELOG.md`
- `goals/security-contact-consolidation/*` (this package)

## Requirements

### R1 — Record the canonical contact decision

Add `docs/decisions/0002-security-contact.md` with:

- **Primary security channel**: GitHub Private Vulnerability Reporting at
  `https://github.com/jadmadi/swapaudit/security/advisories/new` (active after the public repository exists).
- **Fallback email**: one of the existing published addresses, confirmed by the maintainer:
  - `contact@jadmadi.net` (published in `MAINTAINERS.md`; default if no decision is recorded), or
  - `jad@madi.me` (published in `SECURITY.md`), or
  - a dedicated `security@`/`conduct@` mailbox if one exists.
- **Conduct reporting channel**: the same canonical email unless a dedicated conduct mailbox is confirmed.
- **Approver**, **date**, and **monitoring expectation** (for example, "checked at least weekly").
- **Migration note**: if the old address remains active, say for how long; if it is decommissioned, say when.

Do not invent a new mailbox. Do not publish two different addresses for the same purpose. If the maintainer cannot confirm which address is monitored, use `contact@jadmadi.net` and state that it is the fallback.

### R2 — Rewrite `SECURITY.md`

The file must include:

1. **Supported versions**: `0.1.x` (latest release). State that pre-release/source builds are best-effort.
2. **Primary reporting**: GitHub Private Vulnerability Reporting link, with a note that it becomes active when the public repo is published.
3. **Fallback reporting**: the canonical email from R1.
4. **What to include**:
   - Description and affected component.
   - Steps to reproduce or proof of concept.
   - Impact assessment.
   - `swapaudit --version`, OS/distro, kernel version, and command used.
   - Sanitized logs; no secrets, tokens, or private file contents.
5. **Response expectations**:
   - Acknowledge receipt within **48 hours** (existing commitment).
   - Aim to provide an initial assessment within **5 business days** (target, not a contractual SLA).
   - Fix timing depends on severity and maintainer availability; no guaranteed date.
6. **Disclosure**:
   - Coordinated disclosure; agree on a timeline before publishing.
   - Credit the reporter unless anonymity is requested.
   - Do not open a public issue for a vulnerability.
7. **Scope**:
   - In scope: `/proc` parsing, process classification, cleanup safety, updater, release artifacts, install scripts, and documentation claims.
   - Out of scope: vulnerabilities in third-party agent runtimes, the Linux kernel, GitHub, distro packaging, or a user's local configuration. Report those upstream.
8. **Safe harbor**: good-faith research that follows the policy will not be pursued legally; do not access other users' data, degrade service, or run destructive tests on systems you do not own.

### R3 — Align `MAINTAINERS.md`

- Replace the contact cell with the canonical email.
- Add a row or note: "Security reports: GitHub Private Vulnerability Reporting (primary); `<canonical email>` (fallback)."
- Keep the existing role/credential inventory; do not add unnecessary personal data.
- Ensure the email matches `SECURITY.md` exactly.

### R4 — Add `SUPPORT.md`

Include:

- **Where to get help**:
  - Bug reports: GitHub Issues (once public).
  - Feature requests: GitHub Issues using the feature template; note the governance anti-goals and the feature freeze until demand validation.
  - Security: `SECURITY.md` only; never a public issue.
  - Questions: GitHub Discussions if enabled, otherwise Issues.
- **What to include**: version, OS/distro, kernel, command, expected/actual, `--dry-run` output, sanitized logs.
- **Response expectations**: best-effort, no SLA; supported version is the latest release; older versions may not receive fixes.
- **Scope**: Linux-only, local CLI; no hosted service; no account support.
- **Conduct**: link to `CODE_OF_CONDUCT.md` and the canonical conduct contact.
- **Funding/support**: if the project has no paid support, say so plainly. Do not imply a commercial SLA.

### R5 — Add GitHub issue and PR templates

`.github/ISSUE_TEMPLATE/bug_report.yml`:

- `name: Bug report`
- `description: Report a reproducible defect in swapaudit.`
- `title: "[bug]: "`
- `labels: ["bug"]`
- Fields:
  - swapaudit version (`swapaudit --version`).
  - OS and kernel (`uname -a`).
  - Command and flags.
  - Expected behavior.
  - Actual behavior.
  - `--dry-run` output (sanitized).
  - Confirmation that logs contain no secrets.
  - Optional: agent runtime and version.
- Required: version, OS/kernel, command, expected, actual, sanitization confirmation.

`.github/ISSUE_TEMPLATE/feature_request.yml`:

- `name: Feature request`
- `description: Propose a capability or behavior change.`
- `title: "[feature]: "`
- `labels: ["enhancement"]`
- Fields:
  - Problem and current workaround.
  - Proposed behavior.
  - Alternatives considered.
  - Scope check: does it fit the Linux-only, local-CLI, no-telemetry governance?
  - Note that feature work is frozen until demand validation is complete.

`.github/ISSUE_TEMPLATE/config.yml`:

- `blank_issues_enabled: false`
- `contact_links`:
  - Security → canonical GitHub security advisory URL.
  - Support → `SUPPORT.md`.
  - Conduct → canonical conduct contact or `CODE_OF_CONDUCT.md`.

`.github/PULL_REQUEST_TEMPLATE.md`:

- Summary and linked goal.
- Verification checklist:
  - `gofmt -l .` empty.
  - `go vet ./...` passes.
  - `go test -race ./...` passes.
  - `./scripts/check.sh --full` passes.
  - Docs updated.
  - No secrets or local paths.
- Scope confirmation: no new features during the freeze unless the linked goal is a safety/distribution fix.

### R6 — Fix `CODE_OF_CONDUCT.md`

- Replace "report to the project maintainers" with the canonical conduct contact from R1.
- State the reporting expectations: acknowledged within 48 hours, reviewed confidentially, no retaliation.
- Do not publish a home address or phone number. Use the canonical email or a GitHub report form.
- If the maintainer prefers a separate conduct mailbox, record it in R1; otherwise use the canonical email.

### R7 — Add a contact-consistency sentry

Add `scripts/check-contacts.sh`:

- Accept an optional root directory argument (default: repository root) so it can be tested with fixtures.
- Read the canonical email and canonical repo URL from `docs/decisions/0002-security-contact.md` or a small header block in `SECURITY.md`. Prefer an explicit machine-readable block:

```text
<!-- contact-sentry
canonical_email: contact@jadmadi.net
canonical_repo: jadmadi/swapaudit
-->
```

- Fail if:
  - `SECURITY.md` and `MAINTAINERS.md` contain different email addresses.
  - A known old address (`jad@madi.me` or `contact@jadmadi.net`, whichever is not canonical) appears outside the decision record's migration note.
  - `SECURITY.md` points to a GitHub repo other than `canonical_repo`.
  - `SUPPORT.md` or `CODE_OF_CONDUCT.md` omits the canonical contact/link.
  - An issue template links to a different security URL.
- Print a clear error with the file and offending line.
- Exit `0` when consistent, `1` when inconsistent, `2` on usage error.
- Wire it into `scripts/check.sh --full` as "Contact sentry".
- Add a small negative-test fixture under `scripts/testdata/contacts/` or run the sentry against temporary directories in `scripts/check-contacts-test.sh`. The test must prove both pass and fail cases.

### R8 — Update `README.md` and `CONTRIBUTING.md`

- README: add a "Support and security" section linking `SUPPORT.md`, `SECURITY.md`, and the issue tracker; state that security reports must not be public issues.
- CONTRIBUTING: link `SUPPORT.md`, `SECURITY.md`, and `CODE_OF_CONDUCT.md`; add the DCO and verification commands that already exist.
- Do not duplicate the full policy; link to the canonical files.

### R9 — Keep the scope honest

- Do not claim a 24/7 security response, a bug bounty, or a commercial SLA.
- Do not promise a fix timeline.
- Do not publish a personal address that the maintainer has not confirmed as monitored.
- Do not create the public repository in this goal. The GitHub private-reporting link is staged for `standalone-v0-1-0-release` to verify after publication.

### R10 — Phase the GitHub link

Because the repository does not exist yet:

- **Phase 1 (this goal)**: publish the canonical email, SUPPORT.md, issue templates, PR template, and the exact GitHub private-reporting URL. The URL will 404 until the repo is public; label it "active after publication."
- **Phase 2 (release goal)**: after the repo is public, verify the URL, enable private vulnerability reporting, enable issues/discussions, and record the verification in the release goal.

Do not block this goal on the public repo. Do not link to a nonexistent URL without the "active after publication" note.

## Non-Goals

- No public repository creation or push.
- No bug bounty program.
- No legal advice or contractual SLA.
- No security review of third-party agent runtimes or the Linux kernel.
- No changes to product code.
- No collection of personal data beyond the published contact address.

## Acceptance Criteria

- **AC-01**: `docs/decisions/0002-security-contact.md` exists with the chosen canonical email, conduct channel, approver, date, and monitoring expectation.
- **AC-02**: `SECURITY.md` contains supported versions, primary GitHub reporting, fallback email, what to include, 48-hour acknowledgment, disclosure terms, scope, and safe harbor.
- **AC-03**: `MAINTAINERS.md` uses the same canonical email and describes the security channel.
- **AC-04**: `CODE_OF_CONDUCT.md` has a concrete conduct reporting channel and 48-hour acknowledgment.
- **AC-05**: `SUPPORT.md` exists with bug/feature/security/question paths, response expectations, and scope.
- **AC-06**: `.github/ISSUE_TEMPLATE/bug_report.yml`, `feature_request.yml`, and `config.yml` exist and are valid YAML.
- **AC-07**: `.github/PULL_REQUEST_TEMPLATE.md` exists with the verification checklist.
- **AC-08**: `scripts/check-contacts.sh` passes on the repository and fails on a fixture with a mismatched email.
- **AC-09**: `scripts/check.sh --full` includes the contact sentry and passes.
- **AC-10**: `grep -R "jad@madi.me"` returns no matches outside the decision record's migration note.
- **AC-11**: `grep -R "contact@jadmadi.net"` shows the same canonical address in `SECURITY.md`, `MAINTAINERS.md`, `SUPPORT.md`, and `CODE_OF_CONDUCT.md` (or the chosen canonical address consistently).
- **AC-12**: The GitHub private-reporting URL is present with an "active after publication" note; the release goal owns the post-publication check.
- **AC-13**: `README.md` and `CONTRIBUTING.md` link to `SUPPORT.md`, `SECURITY.md`, and `CODE_OF_CONDUCT.md`.
- **AC-14**: No secrets, home addresses, or unconfirmed personal data are added.

## Verification Commands

```bash
./scripts/check-contacts.sh
./scripts/check-contacts-test.sh
./scripts/check.sh --full
grep -R "jad@madi.me" . || true
grep -Rn "contact@jadmadi.net" SECURITY.md MAINTAINERS.md SUPPORT.md CODE_OF_CONDUCT.md
python3 - <<'PY'
import yaml, pathlib
for p in pathlib.Path('.github/ISSUE_TEMPLATE').glob('*.yml'):
    yaml.safe_load(p.read_text())
    print('valid:', p)
PY
```

Add the exact outputs to `goals/security-contact-consolidation/evidence.md` before certifying.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| The canonical email is unmonitored | Record the monitoring expectation; prefer an address already published and monitored; test with a real message |
| Removing `jad@madi.me` loses historical reports | Set up forwarding before removal; document the migration window |
| The GitHub private-reporting link 404s before release | Label it "active after publication"; the release goal verifies it |
| Issue templates become a support burden | Set best-effort expectations; link to SUPPORT.md; no SLA |
| The contact sentry is brittle | Use an explicit machine-readable header and fixture tests; allow the decision record to mention old addresses |
| Publishing a personal email invites spam | Use the already published maintainer address; consider a dedicated alias; do not publish additional personal data |

## Rollback

- Revert the documentation commit.
- If the old address is still monitored, keep it as a forwarding alias rather than republishing it as the primary channel.
- If the contact sentry blocks a legitimate change, fix the machine-readable header first; do not disable the sentry without a replacement.

## References

- Business review: `docs/business-reviews/2026-10-19-swapaudit-full-review.md` (§5, §6.3, §7 G-08)
- Current `SECURITY.md:16` (`jad@madi.me`, GitHub private reporting)
- Current `MAINTAINERS.md:13` (`contact@jadmadi.net`)
- Current `CODE_OF_CONDUCT.md` enforcement section (no address)
- GitHub private vulnerability reporting: `https://github.com/jadmadi/swapaudit/security/advisories/new` (active after publication)
- GitHub issue template syntax: `.github/ISSUE_TEMPLATE/*.yml`
