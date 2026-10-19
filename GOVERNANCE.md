# Project Governance — swapaudit

swapaudit is an open-source swap, RAM, and in-memory cache auditor for Linux developer workstations, created and maintained by **Jad Madi** ([@jadmadi](https://github.com/jadmadi)).

This document defines swapaudit's governance model, core values, architectural scope boundaries, decision-making mechanisms, and review timers.

---

## 1. Governance Model: Written Solo Authority

swapaudit operates under a **Written Solo Authority** model:

- **Focused Utility Architecture**: swapaudit serves developers running Linux workstations with demanding AI coding agents, background daemons, and compiler workloads, while maintaining high architectural coherence and low maintainer overhead.
- **Anti-Pattern Rejection ("No Governance Theatre")**: swapaudit explicitly rejects complex multi-member elected steering committees. For single-maintainer or high-focus developer utilities, bureaucratic governance introduces friction and stalls execution without improving software quality.
- **Transparency & Predictability**: swapaudit adopts a formal, written governance contract specifying explicit decision classes, review timeframes, justified veto rules, and defined project scope.

---

## 2. Mission, Scope & Anti-Goals

The primary responsibility of governance in swapaudit is to defend maintainer attention and protect the software's architectural integrity from scope creep.

### In-Scope

swapaudit focuses strictly on high-performance, terminal-first swap and cache telemetry and safe cleanup:

1. **Linux Kernel Metrics**: Reading /proc/*/status, /proc/*/stat, and /proc/meminfo to accurately attribute VmSwap and memory consumption.
2. **Safe Artifact Cleanup**: Scanning and categorizing stale /tmp compiler caches and abandoned test suites.
3. **Agent Session Attribution**: Distinguishing active, dormant, and orphaned AI agent processes and associated caches.
4. **Zero-Overhead Invocation**: CLI inspection completes in under 200 milliseconds without background polling or daemon overhead.

### Explicit Anti-Goals (Out of Scope)

Proposals introducing any of the following are considered out of scope:

1. **Cloud-Hosted Telemetry**: swapaudit is 100% offline-capable and privacy-respecting. It does not phone home, report anonymous usage statistics, or require external accounts.
2. **Persistent Background Services**: swapaudit is an on-demand CLI tool. It does not install recurring kernel hooks or background listeners.
3. **Web or GUI Frameworks**: swapaudit is a terminal-native tool. Embedding Electron, Tauri, or browser runtimes into the core binary is out of scope.
4. **Unsafe Automated Process Killing**: swapaudit never terminates system daemons, active user shells, systemd services, or processes owned by another UID unless `--all-users` is explicitly set. Orphan cleanup is never implicit: `--clean-all` excludes it unless `--include-orphaned` is set. Cleanups require explicit flags and confirmation prompts.

---

## 3. Decision Classes & Review Timers

To prevent unbounded stalls or silent abandonment, every decision class operates on an explicit clock:

| Decision Class | Scope & Criteria | Review Window | Mechanism |
| :--- | :--- | :--- | :--- |
| **Class 1: Routine Fixes & Docs** | Bug fixes, typo corrections, documentation updates, and CI improvements. | **72 hours** | **Lazy Consensus**: 1 maintainer approval or no objection within 72 hours. |
| **Class 2: New Scanners & Categories** | Adding identification logic for new agent frameworks or build caches. | **7 days** | Must satisfy test fixtures, safe categorization rules, and maintain sub-200ms scan speed. |
| **Class 3: Architectural Changes** | Changes affecting CLI flags, JSON output schemas, or destructive cleanup operations. | **7 days** | Requires an RFC issue with rationale and alternatives; requires Project Lead approval. |
| **Class 4: Licensing & Governance** | Amendments to `LICENSE`, `WaqfDPL-1.0.md`, `GOVERNANCE.md`, or the CLA. | — | Sole authority of the Project Lead (**Jad Madi**). |

---

## 4. Conflict Resolution & Justified Veto

Disagreements are resolved through technical discussion against the in-scope criteria. If consensus cannot be reached, the Project Lead provides a written justification detailing the rationale, constraints, or security considerations for the final decision.
