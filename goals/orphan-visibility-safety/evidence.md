# Evidence: orphan visibility and fail-closed termination

Verdict: PASS

Date: 2026-10-20 (implementation), baseline 2026-10-19.

## Before (goal facts, 2026-10-19)

- `CollectSwapItems` dropped every process with `VmSwap <= 0`, so zero-swap agents were invisible to display and cleanup.
- `abandoned-agents` rendered only when its count was nonzero.
- Orphan evidence was discarded (`_, isOrphaned := proc.ClassifyOrphan(...)`).
- Kill-time checks re-read UID and start ticks, but not parent liveness, orphan evidence, or idle evidence.
- Signal name was not printed.
- Non-TTY cleanup silently skipped every group and exited 0.

## After (live host, 2026-10-20)

Agent visibility and status lines are always printed:

```text
Agents: 8 detected · 8 with swap · 0 zero-swap · 0 orphaned · 8 dormant · 0 idle-unknown · 0 vanished
Orphan scan: 8 agents · 0 orphaned (evidence-gated) · 8 parent-alive · 0 idle-unknown · 0 active
```

`abandoned-agents` renders at zero:

```text
    abandoned-agents                      |        0 |        0 B | [..........]   0.0% |        0 B
```

A zero-swap orphan is retained with `0 B` and a `SIGTERM` action. `TestZeroSwapOrphanVisible` builds an orphaned `agy` snapshot with `VmSwap = 0`, a zero-swap non-agent snapshot, and asserts:

- the agent item survives (`1` item, non-agent dropped);
- category `abandoned-agents`, cleanable;
- `SwapBytes == 0`, `Signal == SIGTERM`;
- the item appears in the actionable table and the orphan category line;
- `TestZeroSwapOrphanCleanupVisible` previews it as `would-signal` with `0 B`;
- `TestZeroSwapOrphanReachesCleanupFlag` proves `--dry-run --clean-orphaned` lists it in the group line with `0 B`.

## Kill-time fail-closed behavior

Live dry-run skips dormant agents with a live parent instead of signaling them:

```text
Group: Dormant Agent Sessions (8 processes, 1.1 GB)
  [dry-run] ! Skipped PID 1430533 (agy): parent bash (PID 938403) is alive
  [dry-run] ! Skipped PID 1618347 (chrome-devtools): parent sh (PID 1618346) is alive
  ...
Cleanup summary (dry-run): 0 would prune, 0 would end, 8 skipped, (8 parent-alive)
```

The override is explicit and prints a warning:

```text
=== RUN   TestParentAliveSkipAndOverride --- PASS
  parent-alive by default; --allow-live-parent prints "WARNING: PID 99999 has a live parent"
```

Other rechecks:

```text
=== RUN   TestOrphanEvidenceChanged --- PASS  (skip reason orphan-evidence-changed)
=== RUN   TestIdleUnknownSkip        --- PASS  (skip reason idle-unknown)
=== RUN   TestSignalPrinted          --- PASS  (would-signal SIGTERM / signaled SIGTERM)
=== RUN   TestNoSIGKILL              --- PASS  (no SIGKILL in cleaner.go)
```

`SIGKILL` is absent from the implementation:

```text
$ grep -R "SIGKILL" internal || true
(no output)
```

## Non-interactive contract

```text
$ printf '' | ./swapaudit --clean-all
exit=2
error: cleanup requires --dry-run or --yes when stdin is not a terminal
```

```text
=== RUN   TestNonTTYCleanupRequiresDryRunOrYes       --- PASS
=== RUN   TestNonTTYAuditWithoutCleanupIsAllowed    --- PASS
=== RUN   TestDryRunWithoutCleanupWarns              --- PASS
=== RUN   TestYesWithoutCleanupWarns                 --- PASS
```

## Full process scan

```text
=== RUN   TestPermissionWarning              --- PASS  (PermissionDenied > 0 prints a WARN)
=== RUN   TestScanProcessesIncludesZeroSwap --- PASS  (zero-swap processes are retained)
=== RUN   TestScanProcessesIdleUnknown      --- PASS
=== RUN   TestReadParentInfoCurrent         --- PASS
=== RUN   TestParseProcStatusFull           --- PASS
=== RUN   TestAgentPopulation               --- PASS
=== RUN   TestClassifyAgentReasons          --- PASS
```

`TestScanProcessesIncludesZeroSwap` asserts the auditor's own PID is excluded, at least one zero-swap process is retained, and `IdleKnown == (Idle >= 0)` for every snapshot.

## Verification gate

```text
$ gofmt -l .            # empty
$ go vet ./...          # pass
$ go test -race ./...   # pass
$ ./scripts/check.sh --full
✔ All checks passed.
```
