# Evidence: Clean Swap and RAM Without Reboot via Guarded Swap Drain and Cache Reclaim

## Goal Attribution
- Goal: `no-reboot-memory-reclaim`
- Repository: `/mnt/Jad/github/projects/jadmadi/swapaudit`
- Branch: `main`
- Verdict: PASS

## 1. Audit Findings

### Objective-blocking gaps
1. No swap-drain path. A grep for `swapoff`, `swapon`, `drop_caches`,
   `memory.reclaim`, and `compact_memory` returned zero production hits. The
   tool could free swapped pages only by killing their process.
2. No page-cache accounting or reclaim. `/proc/meminfo` parsed only
   `MemTotal`, `MemAvailable`, `SwapTotal`, and `SwapFree`; the header had no
   `Cached`, `SReclaimable`, `Shmem`, `Dirty`, or `Writeback`.
3. No per-device swap data. `/proc/swaps` was never parsed, so the tool could
   not report which area held the pages, its priority, or its type.
4. No feasibility check. A naive `swapoff -a` can OOM when used swap exceeds
   available RAM. There was no reserve or headroom calculation.
5. No pressure signal. `/proc/pressure/memory` was never read.

### Safety regressions found during the audit
6. `docs/safety.md` promised UID checks, non-TTY exit `2`, fail-closed orphan
   evidence, and cache liveness checks. `git diff a078f96 HEAD -- internal/`
   shows `internal/cache/liveness.go` deleted (-183 lines),
   `internal/cleaner/cleaner.go` reduced from 556 to 266 lines, and the orphan
   classifier removed. The 303e102 merge dropped the safety branch.
7. Live false positive: PID 328044 `ollama serve` had UID 999, PPID 1,
   `/system.slice/ollama.service`, and `tty_nr=0`. Current triage classified it
   as `abandoned-agents` because `argv` contained `serve` and
   `IsProcessOrphaned(1)` returned true. `--clean-orphaned` could signal a
   system service owned by another UID.
8. `--clean-all` implicitly enabled orphaned kills, contradicting the
   certified contract that orphan cleanup is never implicit.
9. `/tmp` reclaim accounting used `info.Size()` (apparent size), while
   `docs/metrics.md` requires `Stat_t.Blocks * 512` (allocated blocks).

## 2. Implementation

### New reclaim engine
- `internal/reclaim/plan.go`: pure planner, default reserve
  `max(512 MiB, MemTotal/10)`, per-device decisions, zram skip, and partial
  selection. Budget is `MemAvailable - reserve`; selected used swap must fit.
  Ascending and descending greedy passes are compared, so the planner can pick
  one large device or several small ones.
- `internal/reclaim/run.go`: root-gated executor. Per device:
  `swapoff -> verify absent -> swapon [-p priority] -> verify active`.
  Every disabled device is restored on error, cancellation, or signal. A
  restore failure exits `3` and prints the exact `sudo swapon` command.
  `--drop-caches` runs `sync()` then writes `3` to
  `/proc/sys/vm/drop_caches`.
- `internal/proc/proc.go`: `/proc/swaps`, `/proc/pressure/memory`, real UID,
  `tty_nr`, unified cgroup, and fail-closed `ClassifyOrphan`.

### CLI and report
- `swapaudit reclaim` with `--drain-swap`, `--drop-caches`, `--dry-run`,
  `--yes`, `--json`, `--include-zram`, `--reserve`, and `--timeout`.
- Exit codes: `0` success/dry-run/already-clean, `2` precondition failure,
  `3` reclaim failure that may leave a device disabled, `1` unreadable procfs.
- Main report now includes `No-Reboot Reclaim Readiness` with feasibility,
  selected headroom, per-device list, page-cache estimate, and PSI.
- `--clean-all` now expands to untouched + abandoned + dormant only. Orphaned
  kills require `--clean-orphaned` explicitly.

### Safety fixes
- `KillProcessTargets` re-reads the target's real UID and skips other-user
  processes before `SIGTERM`.
- Triage no longer treats `serve`, `--service`, `--stdio`, or generic `mcp` as
  agent markers. Known agent binaries and `-mcp`/`--conversation` markers remain.
- `ClassifyOrphan` fails closed on `.service` cgroups without `.scope` and on
  `tty_nr == 0`.
- Cleanup without `--dry-run` or `--yes` on a non-TTY exits `2` before scanning.
- `/tmp` sizes now use allocated blocks for files and directories.

## 3. Verification Receipts

### Full race suite
```text
$ go test -count=1 -race ./...    # exit 0
ok  github.com/jadmadi/swapaudit/cmd/swapaudit        1.492s
ok  github.com/jadmadi/swapaudit/internal/app         3.671s
ok  github.com/jadmadi/swapaudit/internal/cache       1.023s
ok  github.com/jadmadi/swapaudit/internal/cleaner     1.019s
ok  github.com/jadmadi/swapaudit/internal/live        1.407s
ok  github.com/jadmadi/swapaudit/internal/model       1.015s
ok  github.com/jadmadi/swapaudit/internal/notify      1.022s
ok  github.com/jadmadi/swapaudit/internal/picker      1.019s
ok  github.com/jadmadi/swapaudit/internal/proc        1.008s
ok  github.com/jadmadi/swapaudit/internal/reclaim     1.014s
ok  github.com/jadmadi/swapaudit/internal/snapshot    1.033s
ok  github.com/jadmadi/swapaudit/internal/term        1.008s
ok  github.com/jadmadi/swapaudit/internal/triage      1.009s
ok  github.com/jadmadi/swapaudit/internal/updater     1.134s
ok  github.com/jadmadi/swapaudit/internal/version     1.005s
ok  github.com/jadmadi/swapaudit/internal/watch       1.067s
```

### Project gate
```text
$ ./scripts/check.sh --full       # exit 0
1. Diff Sentry ........................ clean
2. gofmt .............................. clean
3. go vet ............................. clean
4. race tests + coverage .............. pass, total 74.5%
5. release build + self-verification .. pass
All checks passed in 7s.
```

### Live audit: Ollama false positive removed
```text
$ ./swapaudit --no-update-check --no-color --ascii
...
+------------------------------------------------------------------+
| *  No-Reboot Reclaim Readiness                                   |
+------------------------------------------------------------------+
  Swap drain  already clean (0 B used); no swapoff/swapon churn needed
  Devices     /dev/nvme0n1p3 (partition)
  Page cache  4.6 GB reclaimable with: sudo swapaudit reclaim --drop-caches
  Pressure    some 0.00 / full 0.00 (avg10)
...
$ ./swapaudit --no-update-check --no-color --ascii | grep -i ollama
ollama absent from actionable output
```

### Live reclaim plan
```text
$ ./swapaudit reclaim --dry-run --no-color
SYSTEM RECLAIM
------------------------------------------------------------------
  Swap         0 B used / 17.1 GB total across 1 device(s)
  Device       /dev/nvme0n1p3         partition 0 B used / 17.1 GB  already empty
  RAM          19.8 GB available / 31.2 GB total
  Reserve      3.1 GB (10% of RAM, floor 512.0 MB)
  Pressure     some 0.00 / full 0.00 (avg10)
  Page cache   4.6 GB reclaimable with --drop-caches (root)
  Plan         swap is already clean; no swapoff/swapon churn
```

### JSON receipt
```json
{
  "schema_version": 1,
  "action": "drain-swap",
  "dry_run": true,
  "root": false,
  "feasible": false,
  "partial": false,
  "blocked": "already-clean",
  "reserve_bytes": 3354918502,
  "headroom_bytes": 17918517658,
  "all_headroom_bytes": 17918517658,
  "skipped_for_headroom_bytes": 0,
  "result": "dry-run"
}
```

### Precondition refusals
```text
$ ./swapaudit reclaim --drop-caches --yes </dev/null
error: swapoff, swapon, and drop_caches require root
hint: sudo swapaudit reclaim --drop-caches --yes
exit 2

$ ./swapaudit --clean-all </dev/null
error: cleanup requires --dry-run or --yes when stdin is not a terminal
exit 2
```

## 4. Acceptance Assertions

| Assertion | Evidence | Result |
| --- | --- | --- |
| AC-01 infeasible plan performs no swapoff | `TestBuildPlanInsufficient`, `TestRunInfeasibleExecutesNothing` | PASS |
| AC-02 feasible plan with reserve | `TestBuildPlanFeasible`, `TestBuildPlanBoundaryIsFeasible` | PASS |
| AC-03 restore before next device | `TestExecuteDrainRestoresBeforeNextDevice` | PASS |
| AC-04 `swapon` failure exits 3 with recovery | `TestRunSwaponFailureLeavesCriticalAndExit3` | PASS |
| AC-05 dry-run is pure | `TestRunDryRunIsPure`, `TestRunDropCachesDryRunIsPure` | PASS |
| AC-06 non-root mutation refused | `TestRunRequiresRoot`, live refusal above | PASS |
| AC-07 non-TTY mutation refused | `TestRunRequiresYesWithoutTTY`, live refusal above | PASS |
| AC-08 zram skipped unless explicit | `TestBuildPlanZramOnlyIsSkipped`, `TestBuildPlanIncludeZram` | PASS |
| AC-09 zero used swap does no churn | `TestBuildPlanAlreadyClean`, `TestRunAlreadyCleanDoesNoChurn` | PASS |
| AC-10 Ollama is not an orphaned agent | `TestClassifyOrphanOllamaLiveShape`, `TestCategorizeProcess/ollama_serve_is_not_an_agent`, live audit | PASS |
| AC-11 other-UID process is skipped | `TestKillProcessTargetsRejectsOtherUID` | PASS |
| AC-12 snapshot remains valid JSON | `TestRunJSONStreamPurity`, `TestRunJSONDryRun`, `TestE2E_ReclaimDryRunJSON` | PASS |
| Partial drain selects a safe subset | `TestBuildPlanPartialDrain`, `TestBuildPlanPartialPicksBetterSubset`, `TestRunPartialDrainExecutesSelectedOnly` | PASS |
| Headroom re-check aborts before swapoff | `TestExecuteDrainAbortsWhenHeadroomVanishes` | PASS |
| Allocated-block accounting | `TestAllocatedBytesUsesBlocks`, `TestScanTmpDirectoriesUsesAllocatedBlocks` | PASS |
| `--clean-all` excludes orphaned | `TestSelectCleanup/all_excludes_orphaned` | PASS |

## 5. Live Limitation (resolved)

At certification time this host had `0 B` used swap and no passwordless root:

```text
$ sudo -n true
sudo: a password is required
exit 1
```

A live `swapoff`/`swapon` mutation was therefore not executed. The executor
contract is proven by the injected fake tests above, including the failure and
restore paths. The read-only live run proves the planner reads the real
`/proc/swaps` and `/proc/meminfo`. The disposable swap-file protocol in
`docs/reclaim.md#verification-protocol` is the live root test for an operator.

Resolved on 2026-11-02 with a user-approved `pkexec` run. Section 7 records the
live `swapoff`/`swapon` drain of a disposable swap file.

## 6. Known Follow-Ups

- Cache open-handle, cwd, and mapped-file liveness is still a design follow-up.
  `docs/cache-liveness.md` carries an implementation-status banner and
  `docs/safety.md` records the actual checks.
- Explicit per-device selection (`--device`) is not implemented; partial
  selection is automatic.
- `memory.reclaim` and `compact_memory` remain non-goals.

## 7. Live Drain Verification

Run: 2026-11-02T17:36:32+03:00, kernel `6.12.113+deb13-amd64`, binary
`swapaudit v0.1.0-73-g31f9c52`. Root was obtained with a user-approved
`pkexec` prompt on the running KDE polkit agent; the agent cannot run `sudo`
because the global OpenCode config denies `sudo *`.

Target: disposable 1 GiB disk-backed file `/var/tmp/swapaudit-live.swap`
at priority 100. `/dev/nvme0n1p3`, the real 17 GiB swap partition, was never
modified.

### Before

```text
Filename				Type		Size		Used		Priority
/dev/nvme0n1p3                          partition	17892336	0		-2

               total        used        free      shared  buff/cache   available
Mem:           31994       13675       10185        2535       11133       18319
Swap:          17472           0       17472
```

### Test file enabled and pages forced into it

`memhold` (512 MiB anonymous buffer, 4 KiB pages, `MADV_PAGEOUT`) reported
`MADV_PAGEOUT ok on 128/128 chunks`. The test file took 522240 KiB.

```text
Filename				Type		Size		Used		Priority
/dev/nvme0n1p3                          partition	17892336	0		-2
/var/tmp/swapaudit-live.swap            file		1048572		522240		100

               total        used        free      shared  buff/cache   available
Mem:           31994       14147        8675        2540       12190       17847
Swap:          18496         510       17986
```

### Dry run

```text
SYSTEM RECLAIM
------------------------------------------------------------------
  Swap         510.0 MB used / 18.1 GB total across 2 device(s)
  Device       /dev/nvme0n1p3         partition 0 B used / 17.1 GB  already empty
  Device       /var/tmp/swapaudit-... file      510.0 MB used / 1024.0 MB
  RAM          17.4 GB available / 31.2 GB total
  Reserve      3.1 GB (10% of RAM, floor 512.0 MB)
  Headroom     13.8 GB after drain
  Pressure     some 0.00 / full 0.00 (avg10)
  Page cache   9.4 GB reclaimable with --drop-caches (root)
  Plan         drain 1 device(s), 510.0 MB -> RAM, feasible
  Command      swapoff /var/tmp/swapaudit-live.swap && swapon -p 100 /var/tmp/swapaudit-live.swap

  Dry run     would drain 1 device(s); no commands executed
```

### Drain receipt

`swapaudit reclaim --drain-swap --yes --json` exited `0` with empty stderr.

```json
{
  "schema_version": 1,
  "action": "drain-swap",
  "dry_run": false,
  "root": true,
  "feasible": true,
  "partial": false,
  "reserve_bytes": 3354918502,
  "headroom_bytes": 14812201370,
  "all_headroom_bytes": 14812201370,
  "skipped_for_headroom_bytes": 0,
  "before": {
    "swap_used_bytes": 534773760,
    "swap_total_bytes": 19395489792,
    "mem_available_bytes": 18701893632,
    "mem_free_bytes": 9084682240,
    "page_cache_bytes": 10118189056,
    "pressure_some_avg10": 0,
    "pressure_full_avg10": 0
  },
  "after": {
    "swap_used_bytes": 0,
    "swap_total_bytes": 19395489792,
    "mem_available_bytes": 18172604416,
    "mem_free_bytes": 8555393024,
    "page_cache_bytes": 10118189056,
    "pressure_some_avg10": 0,
    "pressure_full_avg10": 0
  },
  "devices": [
    {
      "filename": "/var/tmp/swapaudit-live.swap",
      "type": "file",
      "priority": 100,
      "used_before_bytes": 534773760,
      "drained": true,
      "restored": true
    }
  ],
  "result": "clean",
  "errors": []
}
```

### After the drain

```text
Filename				Type		Size		Used		Priority
/dev/nvme0n1p3                          partition	17892336	0		-2
/var/tmp/swapaudit-live.swap            file		1048572		0		100

               total        used        free      shared  buff/cache   available
Mem:           31994       14658        8164        2540       12190       17336
Swap:          18496           0       18496
```

### Checks

| Check | Expected | Observed | Result |
| --- | --- | --- | --- |
| exit code | `0` | `0` | PASS |
| `result` | `clean` | `clean` | PASS |
| `partial` | `false` | `false` | PASS |
| device `drained` | `true` | `true` | PASS |
| device `restored` | `true` | `true` | PASS |
| `used_before_bytes` | `> 0` | `534773760` | PASS |
| test file `Used` after | `0` | `0` | PASS |
| `/dev/nvme0n1p3` `Used` | `0` | `0` | PASS |
| cleanup | file removed, only real swap left | removed | PASS |

After cleanup `/proc/swaps` listed only `/dev/nvme0n1p3` with `Used 0`.

Raw artifacts: `live-run.txt` (full console transcript) and
`live-receipt.json` (the JSON receipt above).

## Memory Writes

<!-- sila/memory-writes/v1 -->
```json
{
  "schema": "sila/memory-writes/v1",
  "goal_slug": "no-reboot-memory-reclaim",
  "prime": {
    "ran": true
  },
  "recall": [
    {
      "query": "swapoff swapon drain swap without reboot",
      "matches": 52
    },
    {
      "query": "ollama serve orphan false positive",
      "matches": 63
    }
  ],
  "writes": [
    {
      "verb": "memo",
      "target": "internal/reclaim/plan.go",
      "fact_id": "fact_288f271e",
      "note": "Swap drain feasibility = MemAvailable - selected used swap - reserve >= 0; per-device swapoff/swapon with restore; partial subset when the full footprint does not fit"
    },
    {
      "verb": "lesson",
      "target": "internal/triage/triage.go",
      "fact_id": "fact_d14425f5",
      "note": "Ollama serve was classified as an abandoned agent because bare 'serve' marked it as an agent and PID 1 was treated as orphaned; fixed with narrower agent detection and fail-closed ClassifyOrphan"
    },
    {
      "verb": "case",
      "target": "internal/reclaim/run.go",
      "fact_id": "fact_e95777f5",
      "note": "Guarded no-reboot swap/RAM reclaim pattern: budgeted device subset, verify-before-and-after swapoff/swapon, restore on failure, drop_caches as an explicit separate action"
    },
    {
      "verb": "memo",
      "target": "goals/no-reboot-memory-reclaim/evidence.md",
      "fact_id": "fact_74f297f6",
      "note": "Live drain verified with a disposable 1 GiB swap file: 510 MiB used drained to 0 with drained=true and restored=true; root via user-approved pkexec because the harness denies sudo; the real partition stayed at 0"
    }
  ],
  "facts_verify": {
    "exit_code": 0,
    "grounding": "15/16 file-referenced facts grounded; 2 pre-existing missing files: goals/sila-cc-program/goal.md, goals/sila-cc-preflight/goal.md"
  },
  "handoff": {
    "id": "hoff_f541a8aff845"
  }
}
```
<!-- /sila/memory-writes/v1 -->

Verdict: PASS
