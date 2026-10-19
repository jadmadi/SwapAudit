# Contributing to swapaudit

Thank you for contributing to swapaudit.

swapaudit is an open-source tool dedicated as an Islamic digital endowment (Waqf) under the Waqf Digital Public License (Level 2: WaqfDPL-Mulzim 1.0) and dual-licensed under GNU AGPLv3. Governance guidelines are codified in [GOVERNANCE.md](GOVERNANCE.md) and maintainer information in [MAINTAINERS.md](MAINTAINERS.md).

## Developer Certificate of Origin (DCO)

All contributors must certify that they wrote or have the right to submit the code they are contributing. To certify, include a `Signed-off-by` trailer in every git commit:

```bash
git commit -s -m "feat(proc): parse custom kernel sched metrics"
```

## Development Workflow

1. Fork or branch from `main`.
2. Ensure you have Go 1.23 or newer installed on a Linux environment.
3. Make atomic, focused changes.
4. Run the fast-path pre-commit verification (<3s):
   ```bash
   ./scripts/check.sh --quick
   ```
5. Before opening a PR or pushing, run the comprehensive verification suite:
   ```bash
   ./scripts/check.sh --full
   ```
6. Build and verify the binary locally:
   ```bash
   ./build.sh --no-install
   ./swapaudit --help
   ```

## Commit Message Guidelines

We use conventional semantic commit messages:

- `feat:` new capabilities, metrics, or CLI flags
- `fix:` bug fixes or incorrect calculation logic
- `docs:` documentation, architecture notes, or README updates
- `refactor:` code restructuring without external behavioral changes
- `test:` new unit or integration test cases
- `chore:` maintenance, build pipeline, or dependency updates

## Licensing of Contributions

By submitting code, you agree that your contributions will be licensed under both the Waqf Digital Public License (WaqfDPL-Mulzim 1.0) and GNU AGPLv3.
