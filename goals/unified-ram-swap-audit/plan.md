# Plan: Unified RAM & Swap Audit Implementation

- [x] Task 1: Model & Proc Memory Extraction
  - Add `RSSBytes int64` to `model.SwapItem`.
  - Add `ProcMemory` struct in `internal/proc` with `VmSwap int64`, `VmRSS int64`, and `PPid int`.
  - Implement `ParseProcMemory(r io.Reader) ProcMemory` and `ReadProcMemory(pid int) ProcMemory`.
  - Keep `ParseProcStatus` as a wrapper calling `ParseProcMemory` for backwards compatibility.
  - Add unit tests in `internal/proc/proc_test.go` covering `VmRSS` and `VmSwap` parsing.

- [x] Task 2: App Collection & Ranking Flags
  - Update `CollectSwapItems` in `internal/app/app.go` to capture `proc.ReadProcMemory(pid)`.
  - Update the process inclusion filter: capture processes with `swapBytes > 0 || rssBytes >= 10*1024*1024` (or cleanable).
  - Update `--sort` flag validation to support `size`, `swap`, `ram`, and `age`.
  - Update `internal/model/model.go` `ItemSize` to reflect combined memory footprint while maintaining clean separation.

- [x] Task 3: Display & Snapshot Reporting
  - Update `internal/app/display.go` to format memory columns cleanly (showing RAM + Swap in wide tables, or dedicated sorting).
  - Update `internal/snapshot` to include `rss_bytes` on items.
  - Update snapshot tests and assertions.

- [x] Task 4: End-to-End Verification & Evidence
  - Run full test suite with race detector: `go test -v -race ./...`.
  - Verify live run of `swapaudit --sort ram` and `swapaudit --sort swap`.
  - Produce `evidence.md` with standalone `Verdict: PASS`.
