# Cleaning swap and RAM without rebooting

Linux does not need a reboot to empty swap. The supported mechanism is a
swap-drain cycle:

1. Confirm that every used swap page fits in available RAM plus a reserve.
2. `swapoff <device>` so the kernel faults every swapped page back into RAM.
3. `swapon [-p priority] <device>` so the device is active again, now empty.
4. Repeat per device, then verify every device is active.

`swapaudit reclaim` performs this cycle with a feasibility check, a reserve,
per-device restore, and a measured receipt.

Each device is briefly disabled while it is re-enabled. The machine stays up,
but this is not zero downtime.

## Quick start

Read-only plan. No root and no mutation:

```bash
swapaudit reclaim --dry-run
```

Typical plan on a host with 4.0 GB in swap and 20.0 GB available:

```text
SYSTEM RECLAIM
------------------------------------------------------------------
  Swap         4.0 GB used / 17.1 GB total across 1 device(s)
  Device       /dev/nvme0n1p3         partition 4.0 GB used / 17.1 GB
  RAM          20.0 GB available / 31.2 GB total
  Reserve      3.1 GB (10% of RAM, floor 512.0 MB)
  Headroom     12.9 GB after drain
  Pressure     some 0.00 / full 0.00 (avg10)
  Page cache   1.7 GB reclaimable with --drop-caches (root)
  Plan         drain 1 device(s), 4.0 GB -> RAM, feasible
  Command      swapoff /dev/nvme0n1p3 && swapon /dev/nvme0n1p3
```

Execute with confirmation:

```bash
sudo swapaudit reclaim --drain-swap
```

Execute non-interactively:

```bash
sudo swapaudit reclaim --drain-swap --yes
```

Machine-readable receipt:

```bash
sudo swapaudit reclaim --dry-run --json | jq .
sudo swapaudit reclaim --drain-swap --yes --json > receipt.json
```

## What the receipt means

`swap 4.0 GB -> 0 B` means the swapped pages moved back into RAM. It does not
mean 4.0 GB of RAM became free. The pages are resident again, so `RAM avail`
normally drops by roughly the drained swap. That is the intended trade: the
machine has enough RAM, so swap no longer holds stale pages.

Page-cache reclaim is different. `--drop-caches` calls `sync()` and then writes
`3` to `/proc/sys/vm/drop_caches`. It releases clean page cache, dentries, and
inodes. It increases `MemFree`, but `MemAvailable` often changes little because
the kernel already counted that cache as available. It never empties swap.

## Feasibility math

Let:

- `U` = sum of `Used` bytes over every swap device that will be drained
- `A` = `MemAvailable`
- `R` = reserve

Draining is allowed only when:

```text
A - U - R >= 0
```

The default reserve is `max(512 MiB, MemTotal / 10)`. Override it explicitly:

```bash
sudo swapaudit reclaim --dry-run --reserve 4GiB
sudo swapaudit reclaim --drain-swap --reserve 4GiB --yes
```

The executor re-reads `MemAvailable` before every device. If headroom vanished
since the plan, it aborts before `swapoff` and restores anything already
drained. If `swapoff` succeeds but `swapon` fails, the executor retries with a
background context, prints the exact recovery command, and exits `3`.

### Partial drains

If all devices do not fit, the planner selects a subset whose used bytes fit
the budget. It compares an ascending and a descending greedy pass and keeps the
larger total, so it can choose either one large device or several small ones.
The plan prints `partial` and the bytes left in swap:

```text
Plan  drain 1 of 2 device(s), 6.0 GB -> RAM, partial
      5.0 GB remains in swap (insufficient headroom)
```

The unselected devices stay active and untouched. The receipt sets
`partial: true` and reports `skipped_for_headroom_bytes`. `--reserve` controls
the budget; `--device` selection is not implemented yet.

### Receipt fields

| Field | Meaning |
| --- | --- |
| `headroom_bytes` | RAM remaining after the selected drain |
| `all_headroom_bytes` | RAM remaining if every device were drained (negative when partial) |
| `skipped_for_headroom_bytes` | Used swap left on unselected devices |
| `partial` | True when at least one device was left in swap |
| `devices[]` | Per-device `drained`, `restored`, and error text |

## zram

zram is compressed swap stored in RAM. Draining it decompresses pages into RAM
and usually increases RAM pressure, so zram devices are skipped by default:

```text
Plan  zram swap only; draining increases RAM use
Next  add --include-zram only if you accept that trade-off
```

Use `--include-zram` only when you understand that trade-off.

## Safety invariants

- `--dry-run` never calls `swapoff`, `swapon`, `sync`, or `drop_caches`.
- Mutation requires effective UID 0. The plan prints the exact `sudo` command.
- Mutation without `--yes` requires a terminal. A pipe or CI job must pass
  `--yes`; otherwise the command exits `2` before any command.
- Every device is re-enabled before the next device is disabled.
- A device that remains disabled produces exit `3` and a `sudo swapon` recovery
  line on stderr.
- `--drain-swap` and `--drop-caches` are explicit. No kernel tunable
  (`swappiness`, `vfs_cache_pressure`) is written.
- No daemon, pidfile, lock file, or systemd unit is created.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success, dry-run, cancelled, already clean, or no swap configured |
| `2` | Not root, missing `--yes` on a non-TTY, invalid flags, or infeasible |
| `3` | A reclaim command failed or a device was left disabled |
| `1` | `/proc` or `/sys` could not be read |

## Verification protocol

Use a disposable swap file on a host where you can run `sudo`:

```bash
sudo fallocate -l 1G /swapfile-swapaudit-test
sudo chmod 600 /swapfile-swapaudit-test
sudo mkswap /swapfile-swapaudit-test
sudo swapon /swapfile-swapaudit-test

# Force a few pages into swap, for example by running a memory-heavy build
# or a disposable test process under memory pressure.
swapaudit reclaim --dry-run
sudo swapaudit reclaim --drain-swap --yes

# Clean up the fixture
sudo swapoff /swapfile-swapaudit-test
sudo rm /swapfile-swapaudit-test
```

Verify that `/proc/swaps` still lists every original device after the run and
that each device's `Used` column is `0`.

## Limitations

- A swap area re-enabled with `swapon <device>` uses kernel defaults for I/O
  options. Options from `/etc/fstab` are not replayed. The device and priority
  are preserved.
- The default per-command timeout is 5 minutes. A large swap area on a slow
  disk may need `--timeout 20m`; a timeout leaves the device active or triggers
  the restore path.
- The drain cannot protect against another process consuming RAM between the
  headroom re-check and the kernel's page faulting. Keep the reserve meaningful.
- On a host with multiple swap devices, peak RAM is the sum of all drained
  pages, because faulted pages stay resident while later devices are drained.
- Rootless containers and some sandboxes deny `swapoff`, `swapon`, and
  `drop_caches` even with UID 0; the command reports the kernel error.
- Shmem/tmpfs pages are excluded from the page-cache estimate because
  `drop_caches` does not free them.
