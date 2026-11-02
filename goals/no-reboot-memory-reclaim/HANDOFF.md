# Handoff: no-reboot-memory-reclaim

Sila handoff: `hoff_2f8319e467d1`
Branch: `main`
Created: 2026-11-02, updated 2026-11-02 after the live drain

## Status

Complete and verified. The live `swapoff`/`swapon` drain ran on this host and
passed every check.

- Feature commit: `a4c685c`
- First certification: `2f566ed`
- Live evidence commit: `3491dfa`
- Tests: `go test -count=1 -race ./...` pass
- Gate: `sila gate` pass
- Sila: implemented / verified, `unverified_count: 0`

## Live drain proof

Run: 2026-11-02T17:36:32+03:00, kernel `6.12.113+deb13-amd64`, binary
`v0.1.0-73-g31f9c52`.

- Disposable 1 GiB disk-backed file `/var/tmp/swapaudit-live.swap`, priority
  100. `/dev/nvme0n1p3` was never touched.
- `memhold` forced 512 MiB of anonymous pages in with `MADV_PAGEOUT`
  (`128/128` chunks, 522240 KiB used).
- `swapaudit reclaim --drain-swap --yes --json` exited `0` with
  `result: clean`, `partial: false`, one device with `drained: true` and
  `restored: true`, and `swap_used_bytes` `534773760 -> 0`.
- `/proc/swaps` showed the test file at `Used 0` after the drain; cleanup
  removed it and left only the real partition at `Used 0`.

Full transcript: `live-run.txt`. Receipt: `live-receipt.json`. Prose record:
`evidence.md` section 7. Decision record: memo `fact_74f297f6`.

## What is proven

- Pure planner, partial subset selection, and zram handling.
- Executor contract with injected fakes: dry-run purity, headroom abort,
  restore-before-next, `swapon` failure exits `3`, recovery command.
- Real-UID kill guard and fail-closed `ClassifyOrphan`.
- Live audit: `ollama serve` is not an actionable orphan.
- Live drain of real used swap, with the device restored and zeroed.

## How the root blocker was resolved

The agent harness denies `sudo *`, and the host has no passwordless root.
The user approved one `pkexec` prompt on the KDE polkit agent, which ran
`/tmp/opencode/swapaudit-verify/verify.sh` as root. The script is a session
artifact and is not committed; `docs/reclaim.md#verification-protocol` has the
portable procedure.

## Re-running the live test

Use a disposable disk-backed file, never the real partition:

```bash
sudo fallocate -l 1G /var/tmp/swapaudit-live.swap
sudo chmod 600 /var/tmp/swapaudit-live.swap
sudo mkswap /var/tmp/swapaudit-live.swap
sudo swapon -p 100 /var/tmp/swapaudit-live.swap
# force pages in, then:
sudo ./swapaudit reclaim --drain-swap --yes --json
sudo swapoff /var/tmp/swapaudit-live.swap
sudo rm -f /var/tmp/swapaudit-live.swap
```

Never run `swapoff -a` on this host. Never target `/dev/nvme0n1p3`.

## Known follow-ups

- Cache open-handle, cwd, and mapped-file liveness is still a design follow-up.
  `docs/cache-liveness.md` has an implementation-status banner.
- Explicit `--device` selection is not implemented; partial selection is
  automatic.
- The disposable-drain verifier lives only in the session temp directory. It
  could become `scripts/live-drain-verify.sh` if the check needs to repeat.
- `data/x.db*` in the repo root is an unrelated x.com SQLite artifact. It is
  ignored by `.gitignore` and must be left on disk.

## Next goal

`public-install-demand-validation` is the only ready goal (16/61 tasks). See
`goals/public-install-demand-validation/goal.md`.
