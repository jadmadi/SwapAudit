# Plan: Consolidate Security, Support, and Vulnerability Reporting Contacts

## Execution Steps

### Phase 1 — Decide the canonical contact model

- [x] Ask the maintainer which address is monitored: `contact@jadmadi.net`, `jad@madi.me`, or a dedicated alias.
- [x] Confirm whether `jad@madi.me` is an alias, a forwarding address, or obsolete; set a migration window if it is being retired.
- [x] Confirm whether a dedicated `security@` or `conduct@` mailbox exists; do not invent one.
- [x] Write `docs/decisions/0002-security-contact.md` with the primary GitHub path, fallback email, conduct channel, approver, date, and monitoring expectation.
- [x] Add the machine-readable `contact-sentry` header with `canonical_email`, `canonical_repo`, and `canonical_security_url`.

### Phase 2 — Rewrite the policies

- [x] Rewrite `SECURITY.md` with supported versions, primary/fallback reporting, what to include, 48-hour acknowledgment, 5-business-day assessment target, disclosure terms, scope, and safe harbor.
- [x] Update `MAINTAINERS.md` to the canonical email and add the security-channel note.
- [x] Update `CODE_OF_CONDUCT.md` with the canonical conduct channel and 48-hour acknowledgment.
- [x] Add `SUPPORT.md` with bug/feature/security/question routing, response expectations, scope, and conduct link.
- [x] Update `README.md` with a "Support and security" section.
- [x] Update `CONTRIBUTING.md` with links to `SUPPORT.md`, `SECURITY.md`, and `CODE_OF_CONDUCT.md`.
- [x] Update `CHANGELOG.md` under `Changed`/`Security`.

### Phase 3 — Add GitHub templates

- [x] Add `.github/ISSUE_TEMPLATE/bug_report.yml` with the required fields and sanitization confirmation.
- [x] Add `.github/ISSUE_TEMPLATE/feature_request.yml` with the problem/alternatives/scope check and feature-freeze note.
- [x] Add `.github/ISSUE_TEMPLATE/config.yml` with `blank_issues_enabled: false` and contact links for security, support, and conduct.
- [x] Add `.github/PULL_REQUEST_TEMPLATE.md` with the verification checklist and goal reference.
- [x] Validate all YAML with a parser.
- [x] Render the templates locally or inspect the GitHub issue-form schema; do not rely on memory.

### Phase 4 — Add the contact sentry

- [x] Add `scripts/check-contacts.sh` with an optional root argument.
- [x] Parse the `contact-sentry` header from `docs/decisions/0002-security-contact.md` or `SECURITY.md`.
- [x] Fail on mismatched emails, old addresses outside the migration note, wrong repo URLs, missing support/conduct links, or missing security URL.
- [x] Print file and line numbers on failure.
- [x] Add `scripts/check-contacts-test.sh` with pass and fail fixtures under a temporary directory.
- [x] Wire `scripts/check-contacts.sh` into `scripts/check.sh --full` as "Contact sentry".
- [x] Run both scripts and record the output.

### Phase 5 — Verify the GitHub link staging

- [x] Confirm the exact GitHub private-reporting URL for the confirmed repo (`https://github.com/<owner>/<repo>/security/advisories/new`).
- [x] Add the URL to `SECURITY.md` and `SUPPORT.md` with the note "active after publication" if the repo is not yet public.
- [x] Record in `goals/security-contact-consolidation/evidence.md` that `standalone-v0-1-0-release` owns the post-publication URL check.
- [x] Do not claim the link works before the repo exists.

### Phase 6 — Full verification

- [x] `./scripts/check-contacts.sh` passes.
- [x] `./scripts/check-contacts-test.sh` passes both pass and fail cases.
- [x] `./scripts/check.sh --full` passes.
- [x] `grep -R "jad@madi.me" .` returns no matches outside the decision record's migration note.
- [x] `grep -Rn "contact@jadmadi.net" SECURITY.md MAINTAINERS.md SUPPORT.md CODE_OF_CONDUCT.md` shows consistent output (or the chosen canonical address).
- [x] Validate all `.github/ISSUE_TEMPLATE/*.yml` with `python3 -c 'import yaml...'`.
- [x] Check the GitHub URL status and record whether it is 404/pending or active.
- [x] Record all outputs in `goals/security-contact-consolidation/evidence.md`.

### Phase 7 — Attribution commit

- [x] Commit with `(goals/security-contact-consolidation/goal.md)` or `Goal: security-contact-consolidation`, using `docs:` or `chore:`? The goal tool ignores docs/test commits for auto-closing. Use a `chore:` subject if the change is documentation/ops only, or `fix:` if it fixes a broken reporting path. Prefer `fix:` because it repairs an unusable security channel.
- [x] Run `sila goals --project=/home/jad/scripts/active/swapaudit`.
- [x] Do not push unless explicitly authorized.

## Tests to Add

| Test | File | What it proves |
| --- | --- | --- |
| Contact sentry pass fixture | `scripts/check-contacts-test.sh` | Consistent files pass |
| Contact sentry fail fixture | `scripts/check-contacts-test.sh` | Mismatched emails fail with a useful message |
| Old-address fixture | `scripts/check-contacts-test.sh` | A retired address outside the migration note fails |
| Missing template fixture | `scripts/check-contacts-test.sh` | Missing SUPPORT/conduct link fails |
| YAML validation | `scripts/check-contacts-test.sh` or CI | Issue templates parse |
| Link status check | Manual/CI | The GitHub URL is pending before release and active after |

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
curl -s -o /dev/null -w '%{http_code}\n' https://github.com/jadmadi/swapaudit/security/advisories/new
```

## Rollback

- Revert the documentation commit.
- If the canonical email changes, set up forwarding first; do not leave reports going to an unmonitored address.
- If the contact sentry blocks a legitimate change, update the machine-readable header; do not disable the sentry without a replacement.
- If the GitHub private-reporting link must be removed before release, keep the email fallback and the "active after publication" note.

## Completion Evidence

Before certifying, `goals/security-contact-consolidation/evidence.md` must contain:

1. The decision record path and the canonical email/conduct channel.
2. The sentry pass/fail fixture outputs.
3. The YAML validation output.
4. The exact grep results for old and canonical addresses.
5. The GitHub URL status and the handoff note to the release goal.
6. The `scripts/check.sh --full` output.
7. Any deviation from this plan and why.
