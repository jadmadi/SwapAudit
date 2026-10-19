# Project Maintainers — swapaudit

This document lists the active maintainers, project roles, and stewardship for **swapaudit**.

swapaudit operates under a **Written Solo Authority** model led by Jad Madi. Governance mechanisms, review windows, and maintainer responsibilities are codified in [GOVERNANCE.md](GOVERNANCE.md).

---

## 1. Project Lead & Primary Maintainer

| Name | GitHub | Role | Affiliation | Contact |
| :--- | :--- | :--- | :--- | :--- |
| **Jad Madi** | [@jadmadi](https://github.com/jadmadi) | Project Lead, Founder & Primary Maintainer | Independent | `contact@jadmadi.net` |

**Responsibilities**:
- Final architectural direction, roadmap decisions, and release tagging.
- Sole steward of dual-licensing (WaqfDPL-Mulzim 1.0 and GNU AGPLv3).
- Final tiebreaker for technical proposals and pull requests.
- Security vulnerability coordination and CVE disclosure management.

---

## 2. Active Maintainers & Reviewers

Maintainers have triage and review permissions. They review PRs, evaluate new scan categories or cleaner integrations against [AGENTS.md](AGENTS.md), and help shepherd community contributions.

*Currently, all core review and merge operations are managed directly by Project Lead Jad Madi.*

Community contributors demonstrating sustained, high-quality contributions and alignment with [GOVERNANCE.md](GOVERNANCE.md) may be nominated for maintainer status by the Project Lead.

---

## 3. Privileged Access & Credential Inventory

Repository credentials and distribution secrets are strictly managed:

| Subsystem / Resource | Access Level | Authorized Roles | Current Key Holders |
| :--- | :--- | :--- | :--- |
| **GitHub Repository (`main`)** | Push / Merge | Project Lead | `@jadmadi` |
| **GitHub Release Secrets** | GoReleaser Token | Project Lead | `@jadmadi` |
| **Domain & Hosting DNS** | Administrative | Project Lead | `@jadmadi` |

Any maintainer transition requires secret rotation within 30 days.
