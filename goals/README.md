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
- **Total Goals**: 14
- 🟡 **Ready to Execute (Pending)**: 0 (0 independent ⚡, 0 unblocked 🔗)
- 🟢 **Implemented**: 14 (13 verified 📜, 1 stale ⚠️, 0 unverified)
- 🎯 **By Tier**: 4 immediate (Tier 1), 8 roadmap (Tier 2), 2 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 330/330 completed (100%)

---

## 🟢 Implemented

| Goal Package | Mode | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :--- | :--- |
| [`one-shot-action-picker`](one-shot-action-picker/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `969bbde` |
| [`json-snapshot-output`](json-snapshot-output/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `040c948` |
| [`security-hardening-guards`](security-hardening-guards/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `fc6640d` |
| [`ansi-truncate-rendering`](ansi-truncate-rendering/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `d1d4336` |
| [`cache-mtime-propagation-tests`](cache-mtime-propagation-tests/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `003aaa9` |
| [`cleaner-interactive-tests`](cleaner-interactive-tests/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `3162132` |
| [`cleaner-safety-guards`](cleaner-safety-guards/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `3854d3b` |
| [`makefile-install-alignment`](makefile-install-alignment/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `9e578a7` |
| [`proc-stat-edge-case-tests`](proc-stat-edge-case-tests/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `ea9a60d` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `f4f1f93` |
| [`update-check-lifecycle`](update-check-lifecycle/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `602fa2f` |
| [`live-sampler-rates`](live-sampler-rates/goal.md) | `🔗 Dep` | 🚢 SHIP | stale ⚠️ | Git Commit | `b2746e0` |
| [`updater-boundary-tests`](updater-boundary-tests/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Git Commit | `9170277` |
| [`notification-watch`](notification-watch/goal.md) | `🔗 Dep` | 🚢 SHIP | verified 📜 | Git Commit | `225687e` |
