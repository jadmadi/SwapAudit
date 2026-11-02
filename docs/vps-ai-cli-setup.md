# VPS Deployment Guide: Keeping AI CLI Tools Clean

When running continuous AI coding agents (`opencode`, `commandcode` / `cmd`, `muse`, `agy`) on a Linux VPS, memory leaks occur via two primary mechanisms:
1. **RAM-backed `/tmp` exhaustion**: Stale build caches, test workspaces, and tool outputs allocated in `tmpfs`.
2. **Orphaned workers**: Subprocesses and background runners reparented to `PID 1` after an agent CLI or SSH session terminates.

SwapAudit runs with zero runtime dependencies and sub-second execution overhead, making it ideal for automated VPS housekeeping.

---

## 1. Automated Housekeeping via Cron (Recommended)

To keep your VPS clean automatically without rebooting, set up a simple hourly cron job:

Create `/etc/cron.hourly/swapaudit`:
```bash
#!/bin/sh
# Prune stale /tmp caches older than 12h and terminate orphaned agent workers
/usr/local/bin/swapaudit --clean-all -y --no-color > /var/log/swapaudit.log 2>&1
```

Make it executable:
```bash
chmod +x /etc/cron.hourly/swapaudit
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
