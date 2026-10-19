# Goal: Validate Demand with Five User Interviews and a Public Install Test

## Chain of Custody (Isnad)

- **Registered By**: Business review agent, 2026-10-19
- **Spec Audited By**: Pending Spec Audit
- **Implemented By**: Pending Implementation
- **Mystery Audited By**: Pending Mystery Audit

## Goal Description

Run the cheapest credible demand test before adding any feature:

1. **Five structured interviews** with target users (Linux developers/sysadmins running AI coding agents or heavy build workloads).
2. **A public install test** of the released v0.1.0: execute the README commands on at least two clean distros/containers, announce the release in one appropriate channel, and measure external installs, issues, and replies for 14 days.
3. **A written go/narrow/stop decision** with evidence, dated and recorded.
4. **A feature freeze** that prevents new scanners, agent runtimes, JSON output, package-manager work, GUI, daemon, or other features until this goal is certified.

This goal is deliberately about evidence, not building. It exists to answer: *do target users feel this pain, will they install the tool, and does the agent-aware wedge matter to them?*

## Business Context

- The business review (`docs/business-reviews/2026-10-19-swapaudit-full-review.md`) finds demand and ICP unvalidated (G-20), no usage measurement (G-17), and a feature roadmap that could outrun evidence.
- The project has no public release, no issues, no telemetry, and no interviews. The "why now" thesis (AI agents and build caches pressure developer workstations) is plausible but unproven.
- Every direct alternative is free or OS-bundled. Before investing in agent coverage or package managers, the project needs evidence that users will install and use swapaudit.
- The release goal (`standalone-v0-1-0-release`) is a precondition: the public install test cannot run without a public `v0.1.0`.

## Dependencies & Execution Order

- **Mode**: Dependent (requires prerequisite goals)
- **Depends On**: `standalone-v0-1-0-release`
- **Sequence**: 5
- **Shape**: ship
- **Tier**: immediate

## Feature Freeze (Mandatory)

Until this goal is certified:

- **Allowed**: bug fixes, safety fixes, documentation, distribution fixes, release verification, and work required by the three prerequisite goals.
- **Not allowed**: new scanners, new agent runtimes, new cleanup categories, JSON output, package-manager packages, GUI, daemon/watch mode, telemetry, pricing/paywall work, or any feature that expands scope.
- New feature goals must not be created or claimed. If a user requests a feature, record it in `docs/research/feature-backlog.md` and defer it until the decision record exists.
- The freeze is recorded in `GOVERNANCE.md` and `goals/README.md`; it is lifted only by the decision record in this goal.

## Research Questions

The interviews and install test must answer:

1. How often do target users hit swap/RAM/tmpfs pressure, and what triggers it?
2. What do they use today (`ps_mem`, `smem`, `earlyoom`, `systemd-oomd`, `systemd-tmpfiles`, BleachBit, manual `go clean`/Docker prune, reboot)?
3. Which agent runtimes do they actually run (`agy`, Claude Code, Codex, Cursor, Windsurf, Cline, Aider, OpenCode, Gemini, other)?
4. What is the last incident, and what did it cost (time, lost work, reboot)?
5. Which part of swapaudit is valuable: per-process swap attribution, tmpfs cache discovery, agent-aware cleanup, or the unified view?
6. What would prevent them from installing or trusting it (binary downloads, unsigned releases, root prompts, killing processes, privacy)?
7. Which install channel do they prefer (raw binary, `go install`, AUR, Homebrew, Nix, deb/rpm, distro repo)?
8. Would they pay for support or a team feature, or is this strictly a free/waqf utility?
9. What would make them recommend it to a teammate?
10. What should be explicitly out of scope?

## Requirements

### R1 — Write the research protocol before recruiting

Add `docs/research/README.md` with:

- Purpose, research questions, and non-goals.
- Target profile: Linux developer/sysadmin, runs at least one AI coding agent or heavy compiler/container workload, has experienced swap or `/tmp` pressure in the last 90 days.
- Recruitment channels: personal network, Go/Linux communities, agent-user communities. Avoid incentivized or compensated reviews.
- Consent and privacy: record only with consent; no names, emails, employer names, or verbatim quotes without permission; store anonymized notes.
- Bias acknowledgment: five interviews are qualitative, not statistically representative; the sample is convenience-based.
- Decision rules (R5).
- Artifact locations.

Add `docs/research/interview-script.md` with the 10 questions above, plus prompts for "tell me about the last time" and "what did you do instead?" Do not lead the participant toward swapaudit's features before they describe the pain.

### R2 — Conduct five interviews

- Recruit five participants matching the target profile.
- Schedule 30–45 minute interviews; remote or in person.
- Record consent (yes/no) without storing personal identifiers.
- Capture notes in `docs/research/2026-10-19-interviews.md` under anonymized labels (`P1`–`P5`), including:
  - Role and distro (coarse, no employer).
  - Agent runtimes and workload type.
  - Pain frequency and last incident.
  - Current tools/workarounds.
  - Reaction to the swapaudit concept (after describing it neutrally).
  - Trust/install concerns.
  - Preferred distribution channel.
  - Willingness to try, recommend, or pay for support.
- Do not fabricate quotes, participants, or pain. If fewer than five interviews are completed, record the shortfall and extend the time-box; do not claim the threshold was met.
- If a participant declines consent, do not record their data; count them as declined in the recruitment log.

### R3 — Run the public install test

- Precondition: `standalone-v0-1-0-release` is certified and `v0.1.0` is public.
- Execute the README install commands verbatim on at least two clean environments:
  - Ubuntu 24.04 amd64 (container or VM).
  - Fedora 40 amd64 or Arch amd64 (container or VM).
  - If arm64 hardware/qemu is available, run the arm64 binary and record it; otherwise mark arm64 as cross-compiled but not executed.
- For each environment, capture:
  - `uname -a`, distro version, architecture.
  - The exact command run.
  - `swapaudit --version`, `--help`, and `--dry-run --no-color --ascii`.
  - Any missing dependency or permission error.
- Add `scripts/install-smoke.sh` to automate the amd64 install and first run in a clean container or temp `HOME`/`GOBIN`; do not require root.
- Publish the release announcement:
  - GitHub release notes (already part of the release goal).
  - One community channel where self-promotion is allowed (for example a relevant subreddit, Go/Linux forum, or personal blog). Follow the channel's rules; disclose affiliation; do not spam multiple channels on the same day.
  - A short demo (asciinema/GIF or terminal output) showing the audit and a dry-run cleanup. Do not show a real destructive cleanup without a disposable environment.
- Track for 14 days:
  - GitHub release asset download counts at day 1, day 7, and day 14.
  - Issues, discussions, and direct replies.
  - Qualitative feedback and feature requests (record in the backlog).
- Acknowledge the proxy limitation: GitHub download counts are not unique users and may include mirrors, bots, and repeated downloads. Do not claim "10 users" from raw download counts; say "10 asset downloads (proxy)."

### R4 — Measure and record

Add `docs/research/install-test.md` with:

- Release URL, announcement date, channels, and exact post text.
- Asset download counts at day 1, 7, and 14, broken down by architecture.
- Issues/discussions/replies with links and a short summary.
- Install smoke-test results on each environment.
- Failures, blockers, and fixes.
- A clear statement that download counts are a proxy.

Add `docs/research/feature-backlog.md` with requested features and a "deferred until validation" status. Do not implement them in this goal.

### R5 — Apply the decision rules

Write `docs/research/decision.md` with a dated decision:

- **Proceed**: at least 3 of 5 interviewees report frequent/urgent pain and a current workaround; at least 10 external asset downloads (proxy); at least 3 issues/replies or direct user reports; at least 3 interviewees willing to install. → Lift the freeze for the highest-evidence wedge (likely agent-aware cleanup) and create feature goals with evidence.
- **Narrow**: pain is real (≥3/5) but installs are below 10, or the install test reveals distribution friction. → Keep the freeze on features; fix distribution/install/trust first.
- **Stop / personal utility**: fewer than 2 of 5 report the pain and fewer than 5 downloads. → Keep the release as a waqf/personal utility; do not invest in feature breadth; document the decision.
- **Inconclusive**: fewer than 5 interviews, or the release was not public for 14 days. → Extend the time-box once; do not pretend the threshold was met.

The decision must include:

- The strongest evidence for and against the verdict.
- Which assumption was invalidated, if any.
- The next goal(s) allowed or explicitly deferred.
- The date and the decision owner.

### R6 — Enforce the freeze and update governance

- Add a "Feature freeze until demand validation" section to `GOVERNANCE.md` with the allowed/not-allowed list.
- Update `goals/README.md` with a freeze banner and the validation goal as the gate.
- Add a `docs/research/README.md` link from `README.md` or `CONTRIBUTING.md` so contributors see the freeze.
- If a feature request arrives during the freeze, record it in `docs/research/feature-backlog.md`; do not create a goal or start implementation.
- After certification, update the freeze state in `GOVERNANCE.md` and `goals/README.md` with the decision and date.

### R7 — Ethics and data handling

- No PII in the repository; use anonymized labels.
- No secret, token, employer name, or customer name in notes.
- No recording without consent; if recording, store it outside the repo and delete after transcription.
- No paid reviews, incentivized downloads, or astroturfing.
- Disclose the maintainer's affiliation in any public post.
- Do not claim statistical significance from five interviews.
- Do not publish a participant's quote without explicit permission.

### R8 — Time-box and fallback

- Interviews: 2 weeks from the first outreach.
- Public install test: 14 days from the announcement.
- If recruitment stalls, extend once by 1 week and record the shortfall; do not lower the evidence bar silently.
- If the release is delayed, this goal remains blocked; do not run the interview synthesis as if the install test happened.

## Non-Goals

- No feature implementation, no new agent runtimes, no JSON, no package managers, no GUI, no daemon.
- No telemetry or analytics SDK.
- No pricing, paywall, or commercial plan.
- No market-size estimate or statistically representative survey.
- No public repository creation (the release goal owns it).
- No changes to the product beyond bug/safety/distribution fixes.

## Acceptance Criteria

- **AC-01**: `docs/research/README.md`, `interview-script.md`, `2026-10-19-interviews.md`, `install-test.md`, `feature-backlog.md`, and `decision.md` exist.
- **AC-02**: Five interviews are completed with consent recorded; if fewer, the shortfall and extension are documented, and the decision is labeled inconclusive.
- **AC-03**: The interview notes are anonymized and contain no PII.
- **AC-04**: The install smoke test passes on at least two clean amd64 environments; exact commands and outputs are recorded.
- **AC-05**: The release announcement and channels are recorded; no spam or undisclosed affiliation.
- **AC-06**: Asset download counts are recorded at day 1, 7, and 14, labeled as a proxy.
- **AC-07**: Issues/discussions/replies are counted and linked.
- **AC-08**: `docs/research/decision.md` applies R5, is dated, and states Proceed/Narrow/Stop/Inconclusive with evidence.
- **AC-09**: `GOVERNANCE.md` and `goals/README.md` contain the feature freeze and its resolution.
- **AC-10**: No feature goal is created or claimed during the freeze.
- **AC-11**: The strongest counter-evidence is recorded; no fabricated quotes or numbers.
- **AC-12**: `docs/research/feature-backlog.md` records deferred requests without implementing them.

## Verification Commands

```bash
./scripts/install-smoke.sh
gh release view v0.1.0 --json assets --jq '.assets[] | [.name, .download_count] | @tsv'
gh issue list --state all --limit 50
gh api repos/<owner>/swapaudit/releases/tags/v0.1.0 --jq '.assets[] | [.name, .download_count] | @tsv'
grep -RInE '(@|employer|company|phone|address)' docs/research/ || true
```

Record the exact outputs in `goals/public-install-demand-validation/evidence.md`.

## Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Five interviews are not representative | Label as qualitative; record the sample and bias; do not claim statistical significance |
| Download counts include bots/mirrors | Label as a proxy; triangulate with issues/replies and interview intent |
| Recruitment stalls | Extend once; record the shortfall; do not lower the bar silently |
| Participants are friends and give polite answers | Ask about the last incident and current workaround, not hypothetical interest; separate "would use" from "will install" |
| The release is delayed | Keep the goal blocked; do not fabricate the install test |
| Feature requests during the freeze create pressure | Record in the backlog; point to the decision rule; no goal creation |
| Public announcement is seen as spam | Follow community rules; one channel; disclose affiliation; provide value in the post |
| PII leaks into notes | Anonymize at capture; run the grep check; delete recordings after transcription |
| The decision is ambiguous | Use the R5 thresholds; if inconclusive, extend once and say so |

## Rollback

- If the validation is inconclusive, keep the release public and the freeze in place; run a second, larger sample.
- If the decision is Stop, do not delete the release; keep it as a waqf/personal utility and archive the feature backlog.
- If a participant withdraws consent, remove their notes and record the withdrawal without identifying them.
- If the public post violates a community rule, delete it, apologize, and do not repost without permission.

## References

- Business review: `docs/business-reviews/2026-10-19-swapaudit-full-review.md` (§7 G-17/G-20, §11 experiments E-01/E-02/E-03)
- Release goal: `goals/standalone-v0-1-0-release/goal.md`
- Target-user profile: `README.md:29-47`; `GOVERNANCE.md:25-30`
- Feature freeze: `GOVERNANCE.md`; `goals/README.md`
- GitHub release API: `https://api.github.com/repos/<owner>/swapaudit/releases/tags/v0.1.0`
