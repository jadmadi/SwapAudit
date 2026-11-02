# Handoff: no-reboot-memory-reclaim

Sila handoff: `hoff_0f1c856d08bf`
Branch: `main`
Created: 2026-11-02

## Status

Implementation is complete and Sila-certified. The core live claim is
**UNVERIFIED**. Do not mark the goal complete until a real `swapoff`/`swapon`
drain has been captured with used swap.

- Feature commit: `64c451f`
- Certification commit: `da51516`
- Repo hygiene commit: `320249f`
- Worktree: clean
- Tests: `go test -count=1 -race ./...` pass
- Gate: `./scripts/check.sh --full` pass
- Sila: implemented / verified, `unverified_count: 0`
- Independent completion check: **REJECTED** because the only live run
  returned `already-clean` on a host with `0 B` swap.

## What is already proven

- Pure planner, partial subset selection, and zram handling.
- Executor contract with injected fakes: dry-run purity, headroom abort,
  restore-before-next, `swapon` failure -> exit 3, recovery command.
- Real-UID kill guard and fail-closed `ClassifyOrphan`.
- Live audit on this host: `ollama serve` is no longer an actionable orphan.
- Live reclaim dry-run reads the real `/proc/swaps` and `/proc/meminfo`.

## The blocker

This host has `0 B` used swap and no unprivileged path to `CAP_SYS_ADMIN`:

```text
$ sudo -n true
sudo: a password is required

$ grep CapEff /proc/self/status
CapEff: 0000000000000000

$ docker version
failed to connect to the docker API ... no such file or directory

$ podman version
Client: Podman Engine 5.4.2   # rootless; swap is not namespaced

$ command -v qemu-system-x86_64
# not installed

$ timeout 5 pkexec --disable-internal-agent /bin/true
exit 124   # waited for auth; KDE polkit agent PID 25746 is running
```

QEMU is the cleanest rootless path, but it is not installed. Installing it
requires the same root access that is missing. Do not try to exploit setuid
binaries or namespaces: `swapoff` requires `CAP_SYS_ADMIN` in the initial
user namespace.

## Root options

1. The user runs the protocol below in their own terminal with `sudo`.
2. The user approves a `pkexec` prompt on the KDE desktop and the agent
   runs a root script. `pkexec` must have a visible agent and a longer
   timeout than 5 seconds.
3. The user grants a temporary, narrow sudoers rule for the two swap tools
   and `swapaudit reclaim` (not recommended for permanent use).

Do not ask for or store the user's password.

## Live verification protocol

Use a disk-backed path. `/tmp` is tmpfs and cannot back a swap file. Never
touch `/dev/nvme0n1p3`; it is the real 17 GiB swap partition.

```bash
# 0. Build the current tool and the page-out helper.
cd /mnt/Jad/github/projects/jadmadi/swapaudit
./build.sh --no-install
cat >/tmp/memhold.go <<'EOF'
package main

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
)

func main() {
	const size = 256 << 20
	buf := make([]byte, size)
	for i := 0; i < len(buf); i += 4096 {
		buf[i] = 1
	}
	if err := syscall.Madvise(buf, 21); err != nil { // MADV_PAGEOUT
		panic(err)
	}
	fmt.Println("MADV_PAGEOUT done; holding pages")
	time.Sleep(10 * time.Minute)
	runtime.KeepAlive(buf)
}
EOF
CGO_ENABLED=0 go build -o /tmp/memhold /tmp/memhold.go

# 1. Create the disposable swap file (1 GiB) and enable it.
sudo fallocate -l 1G /var/tmp/swapaudit-live.swap
sudo chmod 600 /var/tmp/swapaudit-live.swap
sudo mkswap /var/tmp/swapaudit-live.swap
sudo swapon -p 100 /var/tmp/swapaudit-live.swap
grep swapaudit /proc/swaps

# 2. Force 256 MiB of anonymous pages into the new swap area.
/tmp/memhold &
sleep 2
awk '$1=="/var/tmp/swapaudit-live.swap" {print "used_kib="$4}' /proc/swaps
free -m

# 3. Plan and execute the drain through the tool.
./swapaudit reclaim --dry-run --no-color
sudo ./swapaudit reclaim --drain-swap --yes --json \
  | tee /tmp/reclaim-live-receipt.json | jq .

# 4. Verify the test device is still active with zero used bytes.
awk '$1=="/var/tmp/swapaudit-live.swap" {print "used_kib="$4}' /proc/swaps
free -m

# 5. Clean up.
kill %1
sudo swapoff /var/tmp/swapaudit-live.swap
sudo rm -f /var/tmp/swapaudit-live.swap

# 6. Confirm the real partition was untouched.
cat /proc/swaps
```

Expected tool output:

- The dry-run lists `/var/tmp/swapaudit-live.swap` with `used > 0` and a
  feasible plan.
- The JSON receipt has `partial: false`, one device with
  `drained: true` and `restored: true`, and `result: "clean"`.
- `/proc/swaps` still lists `/var/tmp/swapaudit-live.swap` with `Used 0`.
- `/dev/nvme0n1p3` remains listed with `Used 0` throughout.

If `swapon` fails, the tool exits `3` and prints:

```text
recover with: sudo swapon -p 100 /var/tmp/swapaudit-live.swap
```

Run that recovery command before cleanup.

## Evidence to append

Add a `## 7. Live Drain Verification` section to
`goals/no-reboot-memory-reclaim/evidence.md` containing:

- `cat /proc/swaps` before, after forcing pages, and after the drain.
- `free -m` before and after.
- The full JSON receipt.
- The root method used (`sudo`, `pkexec`, or user-run script).
- A standalone `Verdict: PASS` only after the drain succeeds.

Then:

```bash
sila goals audit no-reboot-memory-reclaim \
  --evidence=goals/no-reboot-memory-reclaim/evidence.md \
  --agent=mystery-auditor
sila goals
```

Commit the new audit receipt and evidence with:

```bash
git commit -m "chore(goals): record live swap drain verification [goal: no-reboot-memory-reclaim]"
```

Save a final Sila handoff and update the `## Memory Writes` block with the
new handoff ID and the live-verification memo ID.

## Safety rules

- Never run `swapoff -a` on this host.
- The disposable file is the only target. Priority `100` makes it the
  preferred area for the forced pages.
- The real partition has `0 B` used, so the planner marks it
  `already-empty` and skips it.
- The executor re-enables each device before touching the next one.
- A restore failure exits `3`; run the printed `swapon` command immediately.
- Kill `/tmp/memhold` after cleanup so its pages return to the free pool.

## Known follow-ups

- Cache open-handle, cwd, and mapped-file liveness is still a design
  follow-up. `docs/cache-liveness.md` has an implementation-status banner.
- Explicit `--device` selection is not implemented; partial selection is
  automatic.
- `data/x.db*` in the repo root is an unrelated x.com SQLite artifact.
  It is ignored by `.gitignore` and must be left on disk.
