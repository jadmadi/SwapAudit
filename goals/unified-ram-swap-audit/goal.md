# Goal: Unified RAM & Swap Audit and Surgical Cleanup

## Context
Standard Linux workstations experience heavy RAM and swap creep due to three key factors:
1. Physical RAM (VmRSS) accumulated by bloated or dormant background apps.
2. In-memory temporary filesystems (`/tmp` tmpfs) accumulating abandoned simulation and test runs.
3. Inactive anonymous pages swapped out to disk (VmSwap).

Previously, `swapaudit` only tracked processes with `VmSwap > 0`. When swap usage was low or 0, active RAM consumers were completely invisible in the audit report.

This goal extends `swapaudit` into a comprehensive, unified **RAM and Swap Auditor**:
- Simultaneously inspects and attributes process physical RAM (`VmRSS`), process swap (`VmSwap`), and tmpfs (`/tmp`).
- Introduces flexible ranking modes (`--sort size|swap|ram|age`).
- Supports targeting RAM-heavy dormant or orphaned developer processes.
- Maintains strict fail-closed safety guards and backwards compatibility with existing CLI flags, snapshots, and tests.

## Deliverables
- `internal/model`: Add `RSSBytes` to `SwapItem` and update `ItemSize`.
- `internal/proc`: Add `ReadProcMemory` and `ParseProcMemory` to extract both `VmSwap`, `VmRSS`, and `PPid` without breaking existing `ParseProcStatus`.
- `internal/app`: Update `CollectSwapItems` to discover both RAM and Swap occupants, support `--sort ram|swap|size|age`.
- `internal/app/display`: Display both RSS and Swap columns in detailed view, rank accurately.
- `internal/snapshot`: Update machine-readable snapshot JSON schema to include `rss_bytes`.
- Full table-driven unit tests and integration tests passing with `-race`.
