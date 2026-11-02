# Roadmap

swapaudit is a Linux tool for finding and safely reclaiming swap, RAM, and
`/tmp` pressure. This roadmap is intentionally short. Feature work is frozen
until demand validation produces a written decision.

## Shipped

- v0.1.0: per-process swap and `/tmp` cache audit, guarded cleanup, four
  Linux architectures, signed release assets.
- v0.2.0: guarded no-reboot swap drain, unified RAM and swap audit, versioned
  JSON snapshot, headless sampler with rates and ETA, foreground watch mode,
  interactive action picker, and native coverage for OpenCode, Muse, and
  Antigravity-style agents.

## In progress

- Demand validation: five target-user interviews, a public install test, and a
  written Proceed, Narrow, Stop, or Inconclusive decision.
  See [docs/research/README.md](docs/research/README.md).

## Next

The validation decision chooses the next work. No feature is committed before
that decision.

Candidates, in order of evidence value:

1. Distribution friction fixes: packaging, install channels, and trust signals,
   if the install test shows friction.
2. Deeper agent-aware cleanup, if interviews show that wedge matters.
3. Cache liveness beyond mtime, including open file descriptors and mappings.

## Not planned

- Telemetry, accounts, or a hosted service.
- A daemon or background poller.
- A GUI or web interface.
- Pricing, paywalls, or commercial plans.
- New scanners or cleanup categories without demand evidence.

## How decisions are made

- Governance and the feature freeze: [GOVERNANCE.md](GOVERNANCE.md).
- Validation method and decision rules: [docs/research/README.md](docs/research/README.md).
- Release process: [docs/release.md](docs/release.md).
