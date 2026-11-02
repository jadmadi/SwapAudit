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
- **Total Goals**: 28
- 🟡 **Ready to Execute (Pending)**: 0 (0 independent ⚡, 0 unblocked 🔗)
- 🔵 **In Progress (Claimed)**: 1
- 🟢 **Implemented**: 27 (19 verified 📜, 8 stale ⚠️, 0 unverified)
- 🎯 **By Tier**: 13 immediate (Tier 1), 13 roadmap (Tier 2), 2 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 861/906 completed (95%)

---

## 🔵 In Progress (Claimed Goals)

| Goal Package | Tier | Mode | Agent | Status & Note | Started |
| :--- | :---: | :---: | :--- | :--- | :--- |
| [`public-install-demand-validation`](public-install-demand-validation/goal.md) | `🎯 Immediate` | `🔗 Dep` | `@opencode` | in progress | 3m0s ago |

---

## 🟢 Implemented

| Goal Package | Mode | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :--- | :--- |
| [`accounting-reconciliation`](accounting-reconciliation/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Evidence (evidence.md) | `evidence` |
| [`cleanup-orphan-uid-safety`](cleanup-orphan-uid-safety/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `b8c7abc` |
| [`one-shot-action-picker`](one-shot-action-picker/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `969bbde` |
| [`json-snapshot-output`](json-snapshot-output/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `040c948` |
| [`update-check-consent-disclosure`](update-check-consent-disclosure/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `31372b3` |
| [`security-contact-consolidation`](security-contact-consolidation/goal.md) | `⚡ Indep` | 🚢 SHIP | stale ⚠️ | Git Commit | `bf78d9f` |
| [`security-hardening-guards`](security-hardening-guards/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `fc6640d` |
| [`ai-cli-vps-coverage`](ai-cli-vps-coverage/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `9fedb41` |
| [`ansi-truncate-rendering`](ansi-truncate-rendering/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `d1d4336` |
| [`cache-mtime-propagation-tests`](cache-mtime-propagation-tests/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `003aaa9` |
| [`cleaner-interactive-tests`](cleaner-interactive-tests/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `3162132` |
| [`cleaner-safety-guards`](cleaner-safety-guards/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `3854d3b` |
| [`makefile-install-alignment`](makefile-install-alignment/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `9e578a7` |
| [`no-reboot-memory-reclaim`](no-reboot-memory-reclaim/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `64c451f` |
| [`proc-stat-edge-case-tests`](proc-stat-edge-case-tests/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `ea9a60d` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `f4f1f93` |
| [`unified-ram-swap-audit`](unified-ram-swap-audit/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `1db2778` |
| [`update-check-lifecycle`](update-check-lifecycle/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `602fa2f` |
| [`updater-dev-version-compare`](updater-dev-version-compare/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `0167195` |
| [`cache-liveness-classification`](cache-liveness-classification/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Evidence (evidence.md) | `evidence` |
| [`live-sampler-rates`](live-sampler-rates/goal.md) | `🔗 Dep` | 🚢 SHIP | stale ⚠️ | Git Commit | `7cde25f` |
| [`standalone-v0-1-0-release`](standalone-v0-1-0-release/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Git Commit | `5449c41` |
| [`updater-boundary-tests`](updater-boundary-tests/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Git Commit | `9170277` |
| [`orphan-visibility-safety`](orphan-visibility-safety/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Evidence (evidence.md) | `evidence` |
| [`notification-watch`](notification-watch/goal.md) | `🔗 Dep` | 🚢 SHIP | stale ⚠️ | Git Commit | `225687e` |
| [`output-information-architecture`](output-information-architecture/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Evidence (evidence.md) | `evidence` |
| [`machine-readable-audit`](machine-readable-audit/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Evidence (evidence.md) | `evidence` |
