# Facts: Validate Demand with Five User Interviews and a Public Install Test

## Architectural Invariants & Constraints

- Evidence over opinion: no fabricated interviews, quotes, download counts, or willingness-to-pay.
- Five interviews are qualitative; never claim statistical significance.
- GitHub download counts are a proxy, not unique users.
- No PII in the repository; anonymize at capture.
- No telemetry or analytics SDK.
- Feature work is frozen until the decision record exists.
- The public install test cannot run before `standalone-v0-1-0-release` is certified.
- No spam, paid reviews, or undisclosed affiliation.

## Observed Facts

| Fact | Evidence | Consequence |
| --- | --- | --- |
| No public release exists | `curl https://github.com/jadmadi/swapaudit` → 404; `git tag` empty | No public install test can run yet |
| No public issue tracker | `gh repo list jadmadi` has no swapaudit; repo 404 | No issues/replies to measure |
| No user research exists | No `docs/research/` directory; no interview notes | Demand is unvalidated |
| No telemetry/analytics | `grep -R "telemetry\|analytics"` finds only governance anti-goals | Activation/retention cannot be measured from the product |
| The product works locally | `go test -race ./...` passes; `./swapaudit --dry-run` runs; audit timed at 81 ms on this host | The install test can focus on distribution, not code correctness |
| Local binary is 6.6 MB, statically linked | `ls -lh swapaudit`; `file swapaudit` | Download size is plausible for a CLI |
| The target profile is stated in the README | `README.md:29-47`; `GOVERNANCE.md:25-30` | Linux developers with AI agents, compilers, containers, browsers |
| Direct alternatives are free or OS-bundled | Business review §8: ps_mem, smem, earlyoom, systemd-oomd, systemd-tmpfiles, BleachBit, Stacer | Willingness to pay is likely low; adoption is the first test |
| The business review ranks demand unvalidated | `docs/business-reviews/2026-10-19-swapaudit-full-review.md` G-20 | This goal is the cheapest way to resolve it |
| The review proposes thresholds | §11 E-01/E-02: 10 installs, 3 pain reports, 5 interviews | Use as starting decision criteria, labeled as proposed |

## Target User Definition

A participant qualifies if they meet at least two of:

- Uses Linux as a primary development workstation.
- Runs at least one AI coding agent (agy, Claude Code, Codex, Cursor, Windsurf, Cline, Aider, OpenCode, Gemini, or similar).
- Runs compilers, test suites, containers, or browsers that create large `/tmp` caches.
- Has experienced swap or `/tmp` pressure in the last 90 days.
- Currently uses `ps_mem`, `smem`, `earlyoom`, `systemd-oomd`, `systemd-tmpfiles`, BleachBit, manual cleanup, or a reboot to recover.

Exclude:

- Users who only run servers and do not use a Linux workstation, unless they explicitly manage developer workstations.
- Vendor employees or competitors with a commercial interest, unless disclosed and treated as separate evidence.
- Anyone who cannot give consent.

## Interview Sample (Proposed)

| Label | Profile | Why included |
| --- | --- | --- |
| P1 | Go/backend developer, Linux workstation, heavy test/build caches | Tests the compiler-cache and swap use case |
| P2 | Platform/SRE engineer, Linux workstation + containers | Tests the operational/cleanup use case |
| P3 | AI-agent power user (multiple agent runtimes) | Tests the agent-aware wedge |
| P4 | OSS maintainer or tooling author | Tests trust, distribution, and support expectations |
| P5 | Developer on a lower-RAM laptop (8–16 GB) | Tests urgency and false-positive tolerance |

This is a convenience sample. Recruit from personal network and relevant communities; disclose the purpose; do not pay for participation.

## Interview Questions (Summary)

1. Tell me about the last time your workstation ran low on memory or swap.
2. What did you do? What tools did you use?
3. How often does this happen? What triggers it?
4. Which AI agents, compilers, containers, or browsers do you run?
5. What did the incident cost (time, lost work, reboot)?
6. What do you use today to find what is consuming swap or `/tmp`?
7. If a tool could show per-process swap + `/tmp` + safe cleanup in one pass, what would make you try it?
8. What would stop you from installing it (binary download, root, killing processes, privacy, trust)?
9. Which install channel do you prefer (raw binary, `go install`, AUR, Homebrew, Nix, deb/rpm)?
10. Would you pay for support or a team feature, or is this strictly a free/waqf utility?
11. What should the tool explicitly not do?
12. Who else should I talk to?

Do not describe swapaudit's features before questions 1–6. Capture the current workaround before presenting the concept.

## Decision Rules (Proposed)

| Outcome | Criteria | Consequence |
| --- | --- | --- |
| **Proceed** | ≥3/5 report frequent/urgent pain + current workaround; ≥10 external asset downloads (proxy); ≥3 issues/replies/direct reports; ≥3 willing to install | Lift the freeze for the highest-evidence wedge (likely agent-aware cleanup); create feature goals with evidence |
| **Narrow** | ≥3/5 pain but <10 installs, or install friction dominates | Keep the feature freeze; fix distribution/install/trust first |
| **Stop / personal utility** | <2/5 pain and <5 downloads | Keep the release as a waqf/personal utility; do not invest in feature breadth |
| **Inconclusive** | <5 interviews, or <14 days of public release | Extend once; do not lower the bar silently |

These thresholds are proposed decision criteria, not measured market data. They must be labeled as such in `decision.md`.

## Public Install Test Environment Matrix

| Environment | Architecture | Minimum checks |
| --- | --- | --- |
| Ubuntu 24.04 container/VM | amd64 | README raw download, `sha256sum -c`, `--version`, `--help`, `--dry-run` |
| Fedora 40 or Arch container/VM | amd64 | Same, plus `go install` in a temp `GOBIN` if Go is available |
| arm64 hardware/qemu (optional) | arm64 | Raw download and `--version`; if unavailable, mark as not executed |

Do not run destructive cleanup in the install test. Use `--dry-run` only.

## Measurement Definitions

- **External asset download**: a GitHub release asset `download_count` for a binary. This is a proxy; it may include mirrors, bots, and repeat downloads.
- **Pain report**: a participant describes a specific incident, frequency, and current workaround, not a hypothetical "that sounds useful."
- **Install intent**: participant says they will install and names a channel; separate from actual download.
- **Issue/reply**: a public GitHub issue, discussion, or direct message that references the release or asks a usage question.
- **Willingness to pay**: an explicit statement about support/team feature; do not infer from downloads.

## Assumptions

| Assumption | Why it is needed | How to test it |
| --- | --- | --- |
| Five interviews can be recruited in two weeks | Time-box | Track outreach; extend once if stalled |
| Target users will answer honestly about pain | Qualitative research | Ask for the last incident and current workaround, not hypothetical interest |
| GitHub download counts are available | Release measurement | Check the release API daily; record counts |
| One community announcement is enough for a first signal | Avoid spam | Post once; measure; do not spam multiple channels |
| The product's value is understandable without a demo | Interview script | Show a short terminal demo only after the pain questions |

## Unknowns

- Whether target users will install a raw binary versus a package manager. **Unknown.** Ask in interviews; measure channel preference.
- Whether AI-agent-specific cleanup is the wedge or a niche. **Unknown.** This is the core hypothesis.
- Whether users will trust a process-killing tool. **Unknown.** Ask about trust and false-positive tolerance.
- Whether anyone will pay for support. **Unknown.** Ask directly; do not infer.
- Whether the sample is representative. **Unknown.** Label as qualitative.
- Whether the release will be public for the full 14 days. **Unknown.** Keep the goal blocked until it is.

## Evidence Commands

```bash
gh release view v0.1.0 --json assets --jq '.assets[] | [.name, .download_count] | @tsv'
gh api repos/<owner>/swapaudit/releases/tags/v0.1.0 --jq '.assets[] | [.name, .download_count] | @tsv'
gh issue list --state all --limit 50
gh api repos/<owner>/swapaudit --jq '{issues: .has_issues, discussions: .has_discussions}'
./scripts/install-smoke.sh
```

## File & Interface Contracts

- `docs/research/README.md`: methodology and decision rules.
- `docs/research/interview-script.md`: exact questions and consent script.
- `docs/research/2026-10-19-interviews.md`: anonymized notes.
- `docs/research/install-test.md`: environments, commands, outputs, download counts.
- `docs/research/feature-backlog.md`: deferred requests.
- `docs/research/decision.md`: dated Proceed/Narrow/Stop/Inconclusive decision.
- `scripts/install-smoke.sh`: clean amd64 install and dry-run.
- `GOVERNANCE.md`: feature-freeze rule.
- `goals/README.md`: freeze banner and gate.
