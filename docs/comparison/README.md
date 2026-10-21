# Comparisons

swapaudit compared with other Linux memory-management tools:

| Tool | Type | What it answers | Can it act? |
| :--- | :--- | :-------------- | :---------- |
| [earlyoom](earlyoom.md) | Userspace daemon | Is memory or swap low enough to kill? | SIGTERM/SIGKILL the process with the highest `oom_score` |
| [systemd-oomd](systemd-oomd.md) | systemd service | Which cgroup is stalling under PSI? | SIGKILL every process in the selected cgroup |
| [zram](zram.md) | Kernel block device | How can swap or `/tmp` be compressed in RAM? | No; storage only |
| [smem](smem.md) | Userspace reporter | How much physical memory (USS/PSS) does each process cost? | No; read-only |
| [vmstat](vmstat.md) | Userspace sampler | Is the system swapping or paging right now? | No; read-only |
| swapaudit | Userspace auditor | Which PIDs and `/tmp` directories hold swap and tmpfs, and what is safe to reclaim? | SIGTERM selected agents and remove selected directories |

earlyoom and systemd-oomd are automatic OOM guardrails. zram is a storage layer.
smem and vmstat are read-only observers. swapaudit is the one that attributes
swap to specific processes and directories and can reclaim them surgically.
