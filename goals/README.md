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
- **Total Goals**: 5
- 🟡 **Ready to Execute (Pending)**: 1 (1 independent ⚡, 0 unblocked 🔗)
- 🟢 **Implemented**: 4 (4 verified 📜, 0 unverified)
- 🎯 **By Tier**: 2 immediate (Tier 1), 2 roadmap (Tier 2), 1 nice-to-have (Tier 3), 0 wont-fix (Tier 4)
- 📋 **Execution Tasks Progress**: 20/20 completed (100%)

---

## 🟡 Ready to Execute (Pending Goals)

| Goal Package | Tier | Mode & Sequence | Dependencies | Focus & Description | Launch Command |
| :--- | :---: | :---: | :--- | :--- | :--- |
| [`makefile-install-alignment`](makefile-install-alignment/goal.md) | `✨ Nice-to-Have` | `🚢 SHIP ⚡ Independent` | - | Align Makefile Install Target with Hardened Build Script — Update Makefile install target to invoke scripts/build --install rather than pla... | `/goal goals/makefile-install-alignment/goal.md` |

---

## 🟢 Implemented

| Goal Package | Mode | Shape | Verification | Task / Ref | Commit |
| :--- | :---: | :--- | :---: | :--- | :--- |
| [`ansi-truncate-rendering`](ansi-truncate-rendering/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `d1d4336` |
| [`cleaner-safety-guards`](cleaner-safety-guards/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `3854d3b` |
| [`sila-onboarding`](sila-onboarding/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `f4f1f93` |
| [`update-check-lifecycle`](update-check-lifecycle/goal.md) | `⚡ Indep` | 🚢 SHIP | verified 📜 | Git Commit | `602fa2f` |
