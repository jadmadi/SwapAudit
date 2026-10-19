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
- **Total Goals**: 10
- 🟡 **Ready to Execute (Pending)**: 4 (3 independent ⚡, 1 unblocked 🔗)
- 🟢 **Implemented**: 6 (6 verified 📜, 0 unverified)
- 🎯 **By Tier**: 4 immediate (Tier 1), 4 roadmap (Tier 2), 2 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 24/37 completed (64%)

---

## 🟡 Ready to Execute (Pending Goals)

| Goal Package | Tier | Mode & Sequence | Dependencies | Focus & Description | Launch Command |
| :--- | :---: | :---: | :--- | :--- | :--- |
| [`cache-mtime-propagation-tests`](cache-mtime-propagation-tests/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⚡ Independent` | - | Expand Cache Nested Directory Mtime Propagation Tests — Add unit tests for ScanTmpDirectoriesIn verifying that recently modified nested ... | `/goal goals/cache-mtime-propagation-tests/goal.md` |
| [`cleaner-interactive-tests`](cleaner-interactive-tests/goal.md) | `🗺️ Roadmap` | `🚢 SHIP ⚡ Independent` | - | Expand Cleaner Interactive Input and Confirmation Tests — Add unit tests for AskConfirmationReader covering positive, negative, empty, and... | `/goal goals/cleaner-interactive-tests/goal.md` |
| [`proc-stat-edge-case-tests`](proc-stat-edge-case-tests/goal.md) | `✨ Nice-to-Have` | `🚢 SHIP ⚡ Independent` | - | Expand Proc Stat Parsing and Malformed Input Tests — Add unit tests for ReadStartTicks and ReadProcessLastActive covering malformed p... | `/goal goals/proc-stat-edge-case-tests/goal.md` |
| [`updater-boundary-tests`](updater-boundary-tests/goal.md) | `🎯 Immediate` | `🚢 SHIP 🔗 Ready` | Deps met: security-hardening-guards | Expand Updater Edge-Case and Download Boundary Tests — Add unit tests for HTTP error status handling, malformed checksums.txt lines, ov... | `/goal goals/updater-boundary-tests/goal.md` |

---

## 🟢 Implemented

| Goal Package | Mode | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :--- | :--- |
| [`security-hardening-guards`](security-hardening-guards/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `fc6640d` |
| [`ansi-truncate-rendering`](ansi-truncate-rendering/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `d1d4336` |
| [`cleaner-safety-guards`](cleaner-safety-guards/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `3854d3b` |
| [`makefile-install-alignment`](makefile-install-alignment/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `9e578a7` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `f4f1f93` |
| [`update-check-lifecycle`](update-check-lifecycle/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `602fa2f` |
