# Goals Status & Pragmatic Execution Roadmap

This index tracks all goal packages under `goals/`, their execution mode (Independent ⚡ vs Dependent 🔗), dependencies, and progress status.

## 🤖 Agent Guide: How to Create, Claim & Execute Goals

1. **Create a Goal**: Scaffold a new goal package conforming to standards:
   ```bash
   sila goals create <slug> --title="..." [--depends-on="..."] [--independent]
   ```
2. **Claim a Goal**: Lock the goal so no other agent duplicates work:
   ```bash
   sila goals claim <slug> --agent=<name> [--note="evaluating..."]
   ```
3. **Launch & Implement**: Follow `facts.md` and `plan.md` until all tests pass:
   ```bash
   /goal goals/<slug>/goal.md
   ```
4. **Progress Updates**: Report status updates during execution:
   ```bash
   sila goals report <slug> "Running e2e test suite"
   ```
5. **Commit with Goal Reference**: Include the goal path in your commit message:
   ```bash
   git commit -m "feat(scope): implement description (goals/<slug>/goal.md)"
   ```
6. **Auto-Reconciliation**: Run `sila goals` to scan commits, mark 🟢 **Implemented**, and clear the claim lock.

## Summary
- **Total Goals**: 15
- 🟡 **Ready to Execute (Pending)**: 3 (3 independent ⚡, 0 unblocked 🔗)
- ⛔ **Blocked on Prerequisites**: 2
- 🟢 **Implemented**: 10 (7 verified 📜, 3 stale ⚠️, 0 unverified)
- 🎯 **By Tier**: 9 immediate (Tier 1), 4 roadmap (Tier 2), 2 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 37/338 completed (10%)

---

> 🧊 **Feature freeze until [`public-install-demand-validation`](public-install-demand-validation/goal.md) is certified.**
>
> **Allowed:** bug fixes, safety fixes, documentation, distribution, release verification, and the three prerequisite goals.
>
> **Not allowed:** new scanners, new agent runtimes, new cleanup categories, JSON output, package-manager packages, GUI, daemon/watch mode, telemetry, pricing/paywall work, or any scope expansion.
>
> New feature goals must not be created or claimed. Record requests in `docs/research/feature-backlog.md` and wait for the dated decision in `docs/research/decision.md`.

---

## 🟡 Ready to Execute (Pending Goals)

| Goal Package | Tier | Mode & Sequence | Dependencies | Focus & Description | Launch Command |
| :--- | :---: | :---: | :--- | :--- | :--- |
| [`cleanup-orphan-uid-safety`](cleanup-orphan-uid-safety/goal.md) | `🎯 Immediate` | `🚢 SHIP ⚡ Independent (#1)` | - | Harden Orphan Detection, Process UID Safety, and Destructive Cleanup Claims — Fix the three highest-risk trust defects in swapaudit's destructive path: 1. `Is... | `/goal goals/cleanup-orphan-uid-safety/goal.md` |
| [`update-check-consent-disclosure`](update-check-consent-disclosure/goal.md) | `🎯 Immediate` | `🚢 SHIP ⚡ Independent (#2)` | - | Make the Update Check Consent-Based and Truthful — Resolve the contradiction between `GOVERNANCE.md` ("does not phone home") and th... | `/goal goals/update-check-consent-disclosure/goal.md` |
| [`security-contact-consolidation`](security-contact-consolidation/goal.md) | `🎯 Immediate` | `🚢 SHIP ⚡ Independent (#3)` | - | Consolidate Security, Support, and Vulnerability Reporting Contacts — Replace the inconsistent and partly unusable security/support surface with one c... | `/goal goals/security-contact-consolidation/goal.md` |
| [`standalone-v0-1-0-release`](standalone-v0-1-0-release/goal.md) | `🎯 Immediate` | `🚢 SHIP ⛔ Blocked (#4)` | Prereqs: cleanup-orphan-uid-safety, update-check-consent-disclosure, security-contact-consolidation | Publish Standalone Repository and v0.1.0 Release with Four Assets — Turn the untracked local project into a public, installable, verifiable standalo... | `/goal goals/standalone-v0-1-0-release/goal.md` |
| [`public-install-demand-validation`](public-install-demand-validation/goal.md) | `🎯 Immediate` | `🚢 SHIP ⛔ Blocked (#5)` | Prereqs: standalone-v0-1-0-release | Validate Demand with Five User Interviews and a Public Install Test — Run the cheapest credible demand test before adding any feature: 1. **Five struc... | `/goal goals/public-install-demand-validation/goal.md` |

---

## 🟢 Implemented

| Goal Package | Mode | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :--- | :--- |
| [`security-hardening-guards`](security-hardening-guards/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Audit Receipt | `fc6640d55580576547bc841728758df1d5b62bf5` |
| [`ansi-truncate-rendering`](ansi-truncate-rendering/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `f01f0cb` |
| [`cache-mtime-propagation-tests`](cache-mtime-propagation-tests/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Audit Receipt | `003aaa91687d3a3b57a76bc4632ff6ccbf488ab1` |
| [`cleaner-interactive-tests`](cleaner-interactive-tests/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Audit Receipt | `3162132ccf129911b3e9ecd53b05244b775c57ca` |
| [`cleaner-safety-guards`](cleaner-safety-guards/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `af4ce6f` |
| [`makefile-install-alignment`](makefile-install-alignment/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `5790fea` |
| [`proc-stat-edge-case-tests`](proc-stat-edge-case-tests/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Audit Receipt | `ea9a60d5e17ef35d1b5c57cd4a8050bc4980dbc5` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Audit Receipt | `f4f1f9339befb56ef5400c6db09a3f354e7109a8` |
| [`update-check-lifecycle`](update-check-lifecycle/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Audit Receipt | `602fa2f57a75f47311499cd7a498f9532641ab33` |
| [`updater-boundary-tests`](updater-boundary-tests/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Audit Receipt | `9170277c74e51b21d35df4dd82757a2cd0f5df39` |
