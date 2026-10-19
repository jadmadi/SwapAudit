# Interview script

## Consent script

> I’m researching how Linux developers find and reclaim memory and `/tmp` pressure on their workstations. This is not a sales call. The interview takes about 30–45 minutes. I’ll take anonymized notes; I will not record your name, employer, email, or identifying details. You can skip any question and stop at any time. May I take notes and use anonymized quotes in an internal synthesis?

Record: consent yes/no, date, coarse role/distro. Do not record identifying details.

## Screener

1. Do you use Linux as a primary development workstation?
2. Which AI coding agents, compilers, containers, or browsers do you run?
3. Have you hit swap or `/tmp` pressure in the last 90 days?
4. What do you currently use to investigate or clean it?

Qualify if at least two target-profile criteria are met.

## Questions

Ask questions 1–6 before describing SwapAudit.

1. Tell me about the last time your workstation ran low on memory or swap. What happened?
2. What did you do? Which commands or tools did you use?
3. How often does this happen, and what triggers it?
4. Which AI agents, language servers, containers, compilers, or browsers do you run at the same time?
5. What did the incident cost you — time, lost work, a reboot, a meeting?
6. What do you use today to find what is consuming swap or `/tmp`? What is missing from it?
7. If a tool showed per-process swap, `/tmp` cache age, and a guarded cleanup in one pass, what would make you try it?
8. What would stop you from installing it — a raw binary, root prompts, killing processes, privacy, trust, or something else?
9. Which install channel do you prefer: raw binary, `go install`, AUR, Homebrew, Nix, deb/rpm, or a distro repo?
10. Would you pay for support or a team feature, or is this strictly a free/waqf utility?
11. What should the tool explicitly **not** do?
12. Who else should I talk to?

## Follow-up prompts

- "Tell me about the last time, not a hypothetical."
- "What did you do instead?"
- "What would have made that safe enough to run?"
- "Did you install anything after that? What happened?"
- "What would make you recommend it to a teammate?"

## Capture template

```text
Label: P1
Consent: yes/no
Role/distro (coarse):
Agent runtimes:
Pain frequency:
Last incident and cost:
Current tools/workarounds:
Reaction to the concept:
Trust/install concerns:
Preferred channel:
Willingness to install / recommend / pay:
Strongest counter-evidence:
Quotes (only with permission):
```

Do not fabricate a participant, a quote, or a pain report. If the sample is incomplete, say so.
