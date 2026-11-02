# Evidence: Unified RAM & Swap Audit Implementation

## Goal Attribution
- Goal: `unified-ram-swap-audit`
- Repository: `/mnt/Jad/github/projects/jadmadi/swapaudit`
- Status: Implemented & Verified

## Verified Capabilities
1. `internal/model`:
   - Added `RSSBytes int64` to `SwapItem`.
   - Updated `ItemSize` to account for combined physical RAM and swap footprints.
2. `internal/proc`:
   - Implemented `ProcMemory` struct, `ParseProcMemory(io.Reader) ProcMemory`, and `ReadProcMemory(pid int) ProcMemory`.
   - Preserved exact signature and behavior for `ParseProcStatus(io.Reader) (int64, int)` and `ReadProcStatus(pid int) (int64, int)`.
   - Verified with unit tests in `internal/proc/proc_test.go`.
3. `internal/app`:
   - Updated `CollectSwapItems` to scan both physical RAM (`VmRSS`) and Swap (`VmSwap`).
   - Added `--sort <size|swap|ram|age>` flag validation and sorting algorithms.
   - Updated table rendering in `internal/app/display.go` to display dedicated RAM and Swap columns on wide terminals and dynamically tailored size headers.
   - Updated `internal/snapshot` to record `rss_bytes` on serialized items and updated golden fixture tests.
4. Active Go Binary:
   - Compiled and placed new binary at `/home/jad/scripts/active/go/swapaudit`.

## Verification Commands & Output
```bash
$ go test -count=1 -v -race ./...
ok  	github.com/jadmadi/swapaudit/cmd/swapaudit	0.495s
ok  	github.com/jadmadi/swapaudit/internal/app	1.592s
ok  	github.com/jadmadi/swapaudit/internal/cache	0.005s
ok  	github.com/jadmadi/swapaudit/internal/cleaner	0.006s
ok  	github.com/jadmadi/swapaudit/internal/live	0.257s
ok  	github.com/jadmadi/swapaudit/internal/model	0.002s
ok  	github.com/jadmadi/swapaudit/internal/notify	0.004s
ok  	github.com/jadmadi/swapaudit/internal/picker	0.003s
ok  	github.com/jadmadi/swapaudit/internal/proc	0.003s
ok  	github.com/jadmadi/swapaudit/internal/snapshot	0.003s
ok  	github.com/jadmadi/swapaudit/internal/term	0.003s
ok  	github.com/jadmadi/swapaudit/internal/triage	0.002s
ok  	github.com/jadmadi/swapaudit/internal/updater	0.030s
ok  	github.com/jadmadi/swapaudit/internal/version	0.006s
ok  	github.com/jadmadi/swapaudit/internal/watch	0.007s
```

Verdict: PASS
