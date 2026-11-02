# VPS Deployment Guide: Keeping AI CLI Tools Clean

When running continuous AI coding agents (`opencode`, `muse`, `agy`) on a Linux VPS, memory leaks occur via two primary mechanisms:
1. **RAM-backed `/tmp` exhaustion**: Stale build caches, test workspaces, and tool outputs allocated in `tmpfs`.
2. **Orphaned workers**: Subprocesses and background runners reparented to `PID 1` after an agent CLI or SSH session terminates.

SwapAudit runs with zero runtime dependencies and sub-second execution overhead, making it ideal for automated VPS housekeeping.

---

## 1. Automated Housekeeping via Cron (Recommended)

Run the housekeeping job as the user who owns the agent processes. A root cron
job cannot signal UID 1000 processes: the ownership guard compares against the
real UID, and root cron has real UID 0.

Create a user crontab entry:

```bash
# Every hour: prune stale /tmp caches and end dormant agent sessions.
# Orphaned kills stay explicit; do not automate them by default.
0 * * * * /usr/local/bin/swapaudit --clean-all -y --no-color \
  >> "$HOME/.cache/swapaudit-cron.log" 2>&1
```

`--clean-all` covers untouched caches, abandoned runs, and dormant agents. It
never includes `--clean-orphaned`. If you have verified the orphan heuristic on
your host, add `--clean-orphaned` explicitly and accept that it is a heuristic.

To empty swap without rebooting, a root job can run the guarded drain. It is
refused when used swap does not fit in available RAM plus the reserve, so a
failed attempt is safe:

```bash
# Root crontab, hourly at minute 17. Check with --dry-run first.
17 * * * * /usr/local/bin/swapaudit reclaim --drain-swap --yes \
  >> /var/log/swapaudit-reclaim.log 2>&1
```

---

## 2. Aggressive VPS Mode (Tight Memory Limits)

For small VPS instances (e.g. 2 GB - 4 GB RAM) running heavy agent workloads, adjust the age and idle thresholds:

```bash
# Prune test caches older than 2 hours and end agent sessions idle for over 45 minutes
swapaudit --clean-all --max-age 2h --idle-threshold 45m -y
```

---

## 3. Early Warning Notifications via `swapaudit watch`

To monitor rate spikes and receive alerts before the Linux OOM-killer activates:

```bash
# Run in the background as a user service:
systemd-run --unit=swapaudit-watch --user swapaudit watch --sustain=30s
```

When swap or tmpfs starts filling at a dangerous rate, `swapaudit watch` detects the escalation and dispatches an incident warning.
