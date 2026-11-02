# Facts & Invariants: Unified RAM & Swap Audit

1. Linux `/proc/[pid]/status` exposes memory statistics:
   - `VmSwap`: swap memory currently used by the process in kB.
   - `VmRSS`: total resident set size in physical RAM in kB.
   - `RssAnon`: anonymous memory resident in physical RAM.
   - `RssFile`: resident pages backed by disk files.
   - `RssShmem`: resident shared memory / tmpfs.
2. Invariant: `ParseProcStatus(r io.Reader) (int64, int)` must retain its exact function signature and return values `(swapBytes, ppid)` to avoid breaking existing callers.
3. Invariant: Snapshots (`internal/snapshot`) must maintain JSON backward compatibility; existing JSON consumers expecting `swap_bytes`, `size_bytes`, and `effective_size_bytes` must continue parsing without error.
4. Invariant: Processes with parent PID 1 or deceased parents (`isOrphaned == true`) belonging to developer agent/runtime categories remain safely cleanable via `--clean-orphaned`.
