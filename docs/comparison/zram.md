# swapaudit vs. zram

zram and swapaudit are not alternatives. **zram** is a kernel block device that
compresses pages in RAM, commonly used as a fast swap device or a compressed
`/tmp`. **swapaudit** is a userspace auditor that reports which processes and
directories are consuming swap and tmpfs, and can reclaim selected ones.

zram changes what swap costs. swapaudit tells you who is in it.

## At a glance

| Dimension | swapaudit | zram |
| :-------- | :-------- | :--- |
| Role | Userspace auditor and surgical cleanup tool | Kernel compressed-RAM block device |
| Layer | `/proc` and `/tmp` analysis | Block device and swap backend |
| Model | One-shot CLI | Configured at boot or with `zramctl`; kernel handles I/O |
| What it stores | Nothing | Compressed pages for swap, `/tmp`, caches |
| Trigger | You invoke it | Kernel page-out under memory pressure |
| Unit of action | Individual PID or directory | Block device (`/dev/zramN`) |
| Per-process visibility | Yes: `VmSwap`, idle time, parent PID | No; zram sees block I/O, not processes |
| Accounting | Logical swap bytes per PID | Logical `disksize`; actual RAM in `mem_used_total` |
| Action | SIGTERM agents, remove stale directories | Compress and store pages; optional writeback and recompression |
| Default policy | 12h stale `/tmp`, 2h idle agents | Distro-dependent; Fedora: `min(50% RAM, 4 GiB)` |
| Privileges | Normal user | Root to configure; kernel does the work |
| Requirements | Linux with `/proc` | `CONFIG_ZRAM`; optionally `zram-generator` |
| Footprint | One-shot, sub-second, no idle cost | ~0.1% of `disksize` when idle, plus compressed data |
| Observability | Terminal report with categories and reclaim totals | `zramctl`, `/sys/block/zram*/mm_stat` |
| License | WaqfDPL-1.0 or AGPL-3.0 | GPL-2.0 (kernel); `zram-generator` is MIT |
| Best fit | Finding and reclaiming swap and tmpfs occupants | Making swap fast and reducing disk I/O |

## The core difference: storage vs. attribution

zram creates RAM-backed block devices named `/dev/zram<id>`. Pages written to
one of these devices are compressed and stored in memory. The device then behaves
like any other block device: it can be formatted as a filesystem, mounted as
`/tmp`, or initialized as swap with `mkswap` and `swapon`.

The important number is that `disksize` is **logical** capacity, not RAM usage.
If zram achieves 2:1 compression, 4 GiB of swapped pages occupy roughly 2 GiB of
RAM. The kernel exposes the real cost through `/sys/block/zram<id>/mm_stat`:

- `orig_data_size` — uncompressed bytes stored.
- `compr_data_size` — compressed bytes stored.
- `mem_used_total` — memory allocated for the device, including allocator and
  metadata overhead.
- `same_pages` — identical pages that share storage.
- `huge_pages` — incompressible pages.

`zramctl` presents the same picture as a table: its default columns include the
logical size, uncompressed data, compressed data, and memory used, and
`-o+COMP-RATIO` adds the compression ratio.

```
   anonymous pages
          |
          v
   kernel swap layer  ---->  /dev/zram0  ---->  compressed pages in RAM
          |                        |
          |                        +-- zramctl / mm_stat: real RAM cost
          |
   /proc/<pid>/status: VmSwap (logical, uncompressed)
          |
          +-- swapaudit reads this per PID
```

swapaudit reads `VmSwap` from `/proc/<pid>/status`, idle time from
`/proc/<pid>/sched`, the parent PID from `/proc/<pid>/stat`, and file sizes and
mtimes under `/tmp`. It can name the process or directory responsible. zram
cannot: it only sees block requests, with no notion of which process owns a
compressed page.

**Accounting caveat.** On a zram-backed swap device, swapaudit's swap totals are
logical swapped memory, not physical RAM cost. A 1 GiB `VmSwap` total may cost
roughly 0.5 GiB of RAM at 2:1 compression. swapaudit does not read
`/sys/block/zram*/`, so use `zramctl` or `mm_stat` when you need the actual
memory consumed by zram. The same applies to `/tmp` on zram: swapaudit reports
file sizes, and deleting files frees logical blocks; the compressed RAM released
depends on the compression ratio and on discard being enabled.

## Where zram wins

- **It makes swap fast.** Page-outs go to RAM instead of a disk, avoiding seek
  and write latency. The difference is order-of-magnitude, not incremental.
- **It reduces SSD wear.** Swap churn stays in volatile memory instead of
  generating write amplification on flash.
- **It increases effective capacity.** Typical compression is around 2:1. zstd
  can achieve better ratios on compressible data, while lz4 favors speed. A 4 GiB
  logical zram device can hold 4 GiB of swapped pages in about 2 GiB of RAM.
- **It is flexible.** zram can back swap, `/tmp`, or other caches. Optional
  writeback moves idle or incompressible pages to a real device, and
  recompression can shrink cold pages with a stronger algorithm.
- **It is transparent and tiny.** No userspace daemon is required; the kernel
  handles everything. An idle zram device costs roughly 0.1% of its configured
  size, or about 1 MiB per 1 GiB.
- **It is a sensible default on low-RAM systems.** Fedora 33+ creates a
  zram swap device (`min(50% RAM, 4 GiB)`) instead of a swap partition and gives
  it a higher priority than disk swap.
- **It reduces I/O thrash.** On a machine with slow or no swap, zram can keep the
  system responsive without turning every page fault into a disk read.

## Where swapaudit wins

- **Per-process attribution.** It names the PID holding the swap, how long it has
  been idle, and whether its parent shell died. zram has no process model.
- **Idle and orphan awareness.** zram has optional page-idle tracking and a
  debug `block_state` interface, but no per-process ownership report. swapaudit
  classifies dormant agents and orphaned sessions directly.
- **tmpfs and file cleanup.** It can remove stale build caches and abandoned test
  runs. zram cannot delete files.
- **Actionable safety.** Path confinement to `/tmp` and `/var/tmp`, ownership
  checks on files, symlink rejection, and PID start-ticks verification before
  SIGTERM. zram has no concept of a safe cleanup target.
- **No root or configuration.** A normal user can audit and clean their own
  footprint. Configuring zram usually requires root.
- **It works without zram.** On systems with disk swap or tmpfs `/tmp`, swapaudit
  is still useful. zram is optional infrastructure.
- **It explains the occupants.** zram tells you how much is compressed; swapaudit
  tells you whether the occupants are dormant agents, active apps, or abandoned
  build output.

## Where each falls short

### zram

- It is not an auditor. It cannot tell you which process or directory is
  responsible for swap usage, or what is safe to remove.
- Compression costs CPU. Incompressible pages (`huge_pages`) consume capacity
  with little or no savings.
- `disksize` is a hard logical cap. If zram fills, swap pressure returns. An
  oversized zram can trade I/O thrash for CPU and memory thrash.
- It is volatile. zram-only swap cannot hibernate, as the Fedora change notes.
- It requires kernel support and root to configure. Distribution defaults vary,
  and a manually created device is easy to mis-size.
- `swpd` and `VmSwap` overstate the RAM cost. Only `zramctl` and `mm_stat` show
  the compressed reality.
- Writeback and recompression add tuning complexity, and writeback depends on a
  backing device and kernel configuration.

### swapaudit

- It does not read zram sysfs. It cannot show the compression ratio or the actual
  RAM consumed by zram, and its swap totals are logical rather than physical.
- It is not a swap backend. It cannot compress pages or improve swap latency.
- It is manual: no continuous monitoring, no pressure trigger, and no automatic
  action unless you schedule it.
- It only acts on its known categories (agents and stale `/tmp` artifacts). It
  will not kill an arbitrary memory hog.
- It sends SIGTERM but has no SIGKILL escalation.
- It is Linux-only and `/proc`-dependent.

## Using them together

1. **Inspect the compressed layer first.**

   ```bash
   zramctl
   cat /sys/block/zram0/mm_stat
   ```

   This tells you the logical size, compression ratio, and actual memory used.
   If the ratio is high, swapaudit's swap numbers will overstate RAM pressure.

2. **Attribute the occupants.**

   ```bash
   swapaudit
   swapaudit --sort age
   ```

   Look for dormant agents and orphaned sessions. On a zram system, those
   processes may be cheap in RAM but still hold swap capacity that prevents new
   pages from being compressed.

3. **Reclaim the safe targets.**

   ```bash
   swapaudit --clean-all --dry-run
   swapaudit --clean-untouched --clean-abandoned -y
   ```

   If `/tmp` is mounted from zram, pruning stale directories frees compressed
   blocks as well as logical space. If `/tmp` is tmpfs, it frees tmpfs pages.

4. **Do not expect zram to prevent OOM.** zram is a storage optimization, not an
   OOM killer. Pair it with earlyoom or systemd-oomd if you need an automatic
   guardrail.

A minimal manual zram swap setup looks like this:

```bash
modprobe zram
zramctl --find --size 4G --algorithm zstd
mkswap /dev/zram0
swapon --priority 100 /dev/zram0
```

On Fedora and other systems using `zram-generator`, the equivalent configuration
lives in `/etc/systemd/zram-generator.conf`:

```ini
[zram0]
zram-size = min(ram / 2, 4096)
compression-algorithm = zstd
```

## Which one should you use?

| Situation | Recommendation |
| :-------- | :------------- |
| Low-RAM laptop or desktop | zram for swap speed, swapaudit for periodic cleanup |
| Fedora 33+ default install | zram is already there; use swapaudit to find swap occupants |
| Server with fast SSD and ample swap | zram optional; swapaudit for attribution and tmpfs cleanup |
| You need to know which process is in swap | swapaudit |
| You need faster swap or less disk I/O | zram |
| You want to remove stale `/tmp` build output | swapaudit |
| You need automatic OOM prevention | earlyoom or systemd-oomd, not zram |
| You want a single tool and no kernel configuration | swapaudit |

## References

- zram kernel documentation: <https://docs.kernel.org/admin-guide/blockdev/zram.html>
- `zramctl(8)`: <https://man7.org/linux/man-pages/man8/zramctl.8.html>
- `zram-generator`: <https://github.com/systemd/zram-generator>
- Fedora swap-on-zram change: <https://fedoraproject.org/wiki/Changes/SwapOnZRAM>
- swapaudit: [README](../../README.md), [`internal/proc`](../../internal/proc/proc.go),
  [`internal/triage`](../../internal/triage/triage.go),
  [`internal/cleaner`](../../internal/cleaner/cleaner.go),
  [`internal/cache`](../../internal/cache/cache.go)

zram behavior verified against the kernel documentation and the util-linux
`zramctl` documentation; Fedora defaults verified against the Fedora 33 change
proposal.

## Related comparisons

- [All comparisons](README.md)
