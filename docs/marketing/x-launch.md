# X / Twitter launch drafts — SwapAudit v0.1.0

> **Drafts only. Nothing here has been posted.** Review before publishing. Do not add metrics that were not measured.

Links:

- Repository: https://github.com/jadmadi/SwapAudit
- Release: https://github.com/jadmadi/SwapAudit/releases/tag/v0.1.0

## 1. Launch — project voice

```text
SwapAudit v0.1.0 is out.

`free` shows swap is full. SwapAudit shows which processes and /tmp caches are responsible — then gives you a guarded cleanup path.

Zero deps. Four architectures. Signed release.

https://github.com/jadmadi/SwapAudit
```

## 2. Launch — author voice

```text
I built SwapAudit: a Linux CLI for the question `free` can't answer — which processes are in swap, which /tmp caches are stale, and what is safe to reclaim.

v0.1.0 is public, signed, and available for amd64/arm64/armv7/riscv64.

https://github.com/jadmadi/SwapAudit
```

## 3. Technical single

```text
`free` shows aggregate swap. `top` shows RSS.

SwapAudit reads /proc and ranks per-process VmSwap, /tmp cache age, and dormant agents — then offers a dry-run and guarded cleanup.

v0.1.0: Linux amd64/arm64/armv7/riscv64
https://github.com/jadmadi/SwapAudit
```

## 4. Privacy single

```text
SwapAudit is network-silent by default. Update checks are opt-in (`--check-updates`). No telemetry, no account, no daemon.

v0.1.0 is public, checksummed, and cosign-signed:
https://github.com/jadmadi/SwapAudit/releases/tag/v0.1.0
```

## 5. Safety single

```text
Killing the wrong process is worse than a full swap.

SwapAudit checks UID ownership, PID start ticks, path confinement, symlinks, and cgroup/tty orphan evidence. Orphan cleanup is opt-in; --dry-run first.

https://github.com/jadmadi/SwapAudit
```

## 6. Quote-tweet option

```text
The best safety feature is what SwapAudit refuses to do: no ambiguous orphan kill, no implicit --clean-all, no default network call.

v0.1.0 is public:
https://github.com/jadmadi/SwapAudit/releases/tag/v0.1.0
```

## 7. Seven-post thread

### 1/7

```text
SwapAudit v0.1.0 is out.

It answers the question `free` can't: what is actually in swap and /tmp, and what can I safely reclaim right now?

A thread on the design and the limits. 🧵
```

### 2/7

```text
The pressure is real: AI agent workers, language servers, containers, browsers, and test caches.

Inactive processes get paged out; /tmp is often tmpfs. `free`, `top`, and `vmstat` only show aggregates.
```

### 3/7

```text
SwapAudit reads `/proc/[pid]/status` for VmSwap, `/proc/[pid]/sched` for idle time, and walks `/tmp` for size + newest mtime.

It ranks occupants and classifies cleanable vs protected.
```

### 4/7

```text
Cleanup is the risky part.

SwapAudit refuses ambiguous orphan evidence, checks the target UID, re-reads start ticks before SIGTERM, confines paths, and rejects symlinks.

`--clean-all` excludes orphan cleanup; `--include-orphaned` is explicit.
```

### 5/7

```text
Privacy: the default audit makes no network requests.

`--check-updates` is an explicit opt-in. No telemetry, no account, no daemon.
```

### 6/7

```text
v0.1.0 ships Linux amd64/arm64/armv7/riscv64, SHA256 checksums, and cosign keyless signatures.

Install:
`go install github.com/jadmadi/swapaudit/cmd/swapaudit@v0.1.0`

Or grab a release binary:
https://github.com/jadmadi/SwapAudit/releases/tag/v0.1.0
```

### 7/7

```text
Limits: Linux-only, /tmp only, four agent runtimes, no JSON or package-manager package yet.

It's free software (WaqfDPL/AGPL). Feedback and bug reports:
https://github.com/jadmadi/SwapAudit
```

## 8. Short variant

```text
SwapAudit v0.1.0: per-process swap + /tmp cache audit + guarded cleanup for Linux.

Public, signed, four architectures.
https://github.com/jadmadi/SwapAudit
```

## Media suggestions

- Attach a sanitized screenshot of `swapaudit --no-color --ascii --limit 5`.
- Or record a 10–15 second asciinema/GIF of `swapaudit --dry-run --clean-all` and stop before any destructive action.
- Suggested alt text: "SwapAudit terminal output showing swap, RAM, /tmp usage, ranked reclaim candidates, and a cleanup summary."
- Do not show real usernames, private paths, or live process command lines in the screenshot.

## Review notes

- The launch tweet and thread make no performance claim. The README's 81 ms figure is a single reference-host observation, not a universal benchmark.
- The comparison with `ps_mem`, `smem`, and `systemd-tmpfiles` is in the README; do not claim SwapAudit replaces them.
- Avoid absolute safety claims or "zero risk." The correct description is "guarded cleanup with a dry-run."
- X counts every URL as 23 characters; the raw text above is kept short so it fits under 280 either way.
- Post one launch tweet, then the thread as a reply. Do not post the same text to multiple communities on the same day.
