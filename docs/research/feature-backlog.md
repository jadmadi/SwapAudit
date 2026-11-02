# Feature backlog (frozen)

Feature work is frozen until `public-install-demand-validation` is certified. Record requests here; do not create or claim a feature goal.

| Date | Request | Source | Evidence of demand | Status |
| --- | --- | --- | --- | --- |
| 2026-11-02 | Detect open file descriptors, process cwd, and file-backed mappings before pruning a `/tmp` cache, so the owning agent is stopped first | X reply, @sebuzdugan | 1 external report; matches the known gap in `docs/safety.md` | Deferred until validation |

Allowed during the freeze: bug fixes, safety fixes, documentation, distribution, release verification, and work required by the three prerequisite goals.

Not allowed: new scanners, agent runtimes, cleanup categories, JSON output, package-manager packages, GUI, daemon/watch mode, telemetry, pricing/paywall work, or scope expansion.
