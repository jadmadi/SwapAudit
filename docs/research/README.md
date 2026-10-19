# Demand validation research

## Purpose

Answer one question before adding features: **do target users feel the swap/tmpfs pain strongly enough to install and use SwapAudit, and does the agent-aware cleanup wedge matter to them?**

This is qualitative research plus a public install test. It is not a statistically representative survey.

## Status

- Protocol and interview script: ready.
- Five interviews: **not started** — requires five consenting target users.
- Public install test: **blocked** — the repository is private; public visibility and a public announcement require maintainer approval.
- 14-day download/issue measurement: **not started** — requires a public release and elapsed time.

The private v0.1.0 release was smoke-tested in Debian bookworm and Alpine 3.20; see [`install-test.md`](install-test.md).

## Target profile

A participant qualifies if they meet at least two of:

- Uses Linux as a primary development workstation.
- Runs at least one AI coding agent (`agy`, Claude Code, Codex, Cursor, Windsurf, Cline, Aider, OpenCode, Gemini, or similar).
- Runs compilers, test suites, containers, or browsers that create large `/tmp` caches.
- Has experienced swap or `/tmp` pressure in the last 90 days.
- Currently uses `ps_mem`, `smem`, `earlyoom`, `systemd-oomd`, `systemd-tmpfiles`, BleachBit, manual cleanup, or a reboot to recover.

Exclude users who only run servers without a Linux workstation, vendor employees with a commercial interest (unless disclosed and treated separately), and anyone who cannot give consent.

## Recruitment

- Personal network and relevant Go/Linux/agent-user communities.
- Disclose the purpose; do not pay for participation; do not incentivize reviews.
- Screen against the target profile and record only a coarse qualification reason.
- If recruitment stalls, extend the time-box once and record the shortfall. Do not lower the evidence bar silently.

## Consent and privacy

- Record consent before notes.
- No names, emails, employer names, or verbatim quotes without explicit permission.
- Store anonymized labels (`P1`–`P5`) only.
- If recording, store the recording outside the repository and delete it after transcription.
- A participant may withdraw; remove their notes and record the withdrawal without identifying them.

## Bias

Five convenience interviews are not representative. Report the sample, the recruitment channel, and the bias. Separate stated intent ("I would try it") from observed action ("I installed it"). Do not claim statistical significance.

## Decision rules

| Outcome | Criteria | Consequence |
| --- | --- | --- |
| **Proceed** | ≥3/5 report frequent/urgent pain + current workaround; ≥10 external asset downloads (proxy); ≥3 issues/replies/direct reports; ≥3 willing to install | Lift the freeze for the highest-evidence wedge |
| **Narrow** | ≥3/5 pain but <10 installs, or install friction dominates | Keep the freeze; fix distribution/trust |
| **Stop / personal utility** | <2/5 pain and <5 downloads | Keep the release as a waqf/personal utility; no feature investment |
| **Inconclusive** | <5 interviews, or <14 days of public release | Extend once; do not lower the threshold |

These are proposed decision criteria, not measured market data.

## Artifacts

- [`interview-script.md`](interview-script.md) — consent, screener, questions.
- [`2026-10-19-interviews.md`](2026-10-19-interviews.md) — anonymized notes (to be created after interviews).
- [`install-test.md`](install-test.md) — install environments and results.
- [`feature-backlog.md`](feature-backlog.md) — deferred requests during the freeze.
- [`decision.md`](decision.md) — dated Proceed/Narrow/Stop/Inconclusive decision (to be created after data).

## Related

- Feature freeze: [`GOVERNANCE.md`](../../GOVERNANCE.md).
- Goal: [`goals/public-install-demand-validation/goal.md`](../../goals/public-install-demand-validation/goal.md).
- Business review: `docs/business-reviews/2026-10-19-swapaudit-full-review.md` (internal/private).
