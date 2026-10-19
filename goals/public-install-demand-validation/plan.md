# Plan: Validate Demand with Five User Interviews and a Public Install Test

## Execution Steps

### Phase 1 — Write the protocol and freeze the scope

- [ ] Create `docs/research/README.md` with purpose, target profile, recruitment, consent, bias, decision rules, and artifact locations.
- [ ] Create `docs/research/interview-script.md` with the 12 questions and the consent script.
- [ ] Add the feature-freeze section to `GOVERNANCE.md` with allowed/not-allowed work.
- [ ] Add a freeze banner to `goals/README.md` pointing to this goal.
- [ ] Add `docs/research/feature-backlog.md` with an empty table and "deferred until validation" status.
- [ ] Confirm `standalone-v0-1-0-release` is certified; if not, stop and keep this goal blocked.

### Phase 2 — Recruit five participants

- [ ] List target channels (personal network, Go/Linux communities, agent-user communities).
- [ ] Send a short recruitment note with purpose, time commitment, consent, and no compensation.
- [ ] Screen against the target profile; record qualification reasons without PII.
- [ ] Schedule 30–45 minute interviews.
- [ ] Track declines and no-shows; if fewer than five are scheduled within one week, extend recruitment once.

### Phase 3 — Conduct and synthesize interviews

- [ ] Conduct five interviews; capture consent yes/no before notes.
- [ ] Ask questions 1–6 before describing swapaudit.
- [ ] Record pain frequency, last incident, current workaround, agent runtimes, trust concerns, install channel, and willingness to pay.
- [ ] Anonymize notes as P1–P5; remove names, employers, emails, and identifying details.
- [ ] Write `docs/research/2026-10-19-interviews.md` with a synthesis:
  - [ ] Pain frequency table.
  - [ ] Current tools/workarounds table.
  - [ ] Agent runtime table.
  - [ ] Trust/install concerns.
  - [ ] Willingness to install/pay.
  - [ ] Strongest counter-evidence.
- [ ] Run the PII grep and fix any leak.

### Phase 4 — Run the public install test

- [ ] Confirm `v0.1.0` is public and the release assets are verifiable.
- [ ] Run `scripts/install-smoke.sh` on Ubuntu 24.04 amd64 (container or VM).
- [ ] Run the same on Fedora 40 or Arch amd64.
- [ ] If arm64 hardware/qemu is available, run the arm64 binary; otherwise mark it not executed.
- [ ] Capture `uname -a`, distro, exact commands, `--version`, `--help`, and `--dry-run`.
- [ ] Write `docs/research/install-test.md` with the environment matrix and outputs.
- [ ] Publish one announcement in a community where self-promotion is allowed; disclose affiliation.
- [ ] Record the announcement URL, date, and exact text.
- [ ] Do not run destructive cleanup in any test environment.

### Phase 5 — Measure for 14 days

- [ ] Record asset download counts at day 1, day 7, and day 14 via the release API.
- [ ] Record issues, discussions, and direct replies with links.
- [ ] Triage each issue/reply into: install blocker, usage question, bug, feature request, or praise.
- [ ] Add feature requests to `docs/research/feature-backlog.md`; do not implement them.
- [ ] Record the proxy limitation and any bot/mirror suspicion.
- [ ] If the release is not public for the full 14 days, extend the window and label the result inconclusive until complete.

### Phase 6 — Apply the decision rules

- [ ] Write `docs/research/decision.md` with the R5 criteria and the observed evidence.
- [ ] State Proceed / Narrow / Stop / Inconclusive with the exact counts.
- [ ] Include the strongest evidence for and against the verdict.
- [ ] Name the invalidated assumption, if any.
- [ ] List the next goals allowed or explicitly deferred.
- [ ] Date and sign the decision; name the decision owner.
- [ ] If inconclusive, extend once and record the new time-box; do not lower the threshold silently.

### Phase 7 — Resolve the feature freeze

- [ ] If Proceed: lift the freeze in `GOVERNANCE.md` and `goals/README.md`; create feature goals only for the highest-evidence wedge.
- [ ] If Narrow: keep the freeze; create distribution/trust goals only.
- [ ] If Stop: keep the release as a waqf/personal utility; archive the backlog; do not create feature goals.
- [ ] Update `goals/README.md` with the decision link and date.
- [ ] Update `README.md` or `CONTRIBUTING.md` with a link to the research/decision.
- [ ] Do not rewrite history to make the evidence look stronger than it is.

### Phase 8 — Verification and evidence

- [ ] `./scripts/install-smoke.sh` passes on both amd64 environments.
- [ ] `gh release view v0.1.0 --json assets` output is recorded.
- [ ] `gh issue list --state all` output is recorded.
- [ ] `grep -RInE '(@|employer|company|phone|address)' docs/research/` returns no PII.
- [ ] `docs/research/decision.md` is consistent with the raw counts.
- [ ] `GOVERNANCE.md` and `goals/README.md` show the correct freeze state.
- [ ] Record all outputs in `goals/public-install-demand-validation/evidence.md`.

### Phase 9 — Attribution commit

- [ ] Commit with `(goals/public-install-demand-validation/goal.md)` or `Goal: public-install-demand-validation`, using `docs:` or `chore:`. The goal tool ignores docs/test commits for auto-closing; if auto-close is required, use a `chore:` subject that includes the goal reference.
- [ ] Run `sila goals --project=/home/jad/scripts/active/swapaudit`.
- [ ] Do not push unless explicitly authorized.

## Artifacts to Create

| Artifact | Purpose |
| --- | --- |
| `docs/research/README.md` | Method, target profile, consent, bias, decision rules |
| `docs/research/interview-script.md` | Exact questions and consent script |
| `docs/research/2026-10-19-interviews.md` | Anonymized notes and synthesis |
| `docs/research/install-test.md` | Environments, commands, outputs, download counts |
| `docs/research/feature-backlog.md` | Deferred requests |
| `docs/research/decision.md` | Dated Proceed/Narrow/Stop/Inconclusive decision |
| `scripts/install-smoke.sh` | Repeatable clean install + dry-run |
| `goals/public-install-demand-validation/evidence.md` | Raw outputs and verification |

## Verification Commands

```bash
./scripts/install-smoke.sh
gh release view v0.1.0 --json assets --jq '.assets[] | [.name, .download_count] | @tsv'
gh issue list --state all --limit 50
grep -RInE '(@|employer|company|phone|address)' docs/research/ || true
```

## Rollback

- If the validation is inconclusive, keep the release public and the freeze in place; run a second sample.
- If the decision is Stop, keep the release as a utility and archive the backlog; do not delete the repo.
- If a participant withdraws consent, remove their notes and record the withdrawal without identifying them.
- If an announcement violates community rules, delete it, apologize, and do not repost without permission.

## Completion Evidence

Before certifying, `goals/public-install-demand-validation/evidence.md` must contain:

1. The five consent records (yes/no, no PII) and the anonymized interview synthesis.
2. The install smoke-test outputs for two environments.
3. The announcement URL/date/text.
4. Asset download counts at day 1, 7, and 14.
5. Issue/reply inventory with links.
6. The decision record with exact thresholds and counts.
7. The governance freeze update.
8. Any deviation from this plan and why.
