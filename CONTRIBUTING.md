# Contributing to swapaudit

Thank you for contributing to swapaudit.

swapaudit is an open-source tool dedicated as an Islamic digital endowment (Waqf) under the Waqf Digital Public License (Level 2: WaqfDPL-Mulzim 1.0) and dual-licensed under GNU AGPLv3.

## Developer Certificate of Origin (DCO)

All contributors must certify that they wrote or have the right to submit the code they are contributing. To certify, include a `Signed-off-by` trailer in every git commit:

```bash
git commit -s -m "feat(proc): parse custom kernel sched metrics"
```

## Development Workflow

1. Fork or branch from `main`.
2. Ensure you have Go 1.23 or newer installed.
3. Make atomic, focused changes.
4. Run the test suite:
   ```bash
   make test
   ```
5. Check formatting and linting:
   ```bash
   make lint
   ```
6. Build and verify the binary:
   ```bash
   make build
   ./swapaudit --help
   ```

## Commit Message Guidelines

We use conventional semantic commits:

- `feat:` new capabilities or flags
- `fix:` bug fixes or incorrect calculations
- `docs:` documentation or README improvements
- `refactor:` architectural changes with no user-facing difference
- `test:` new unit or integration test cases
- `chore:` maintenance, build scripts, or dependencies

## Licensing of Contributions

By submitting code, you agree that your contributions will be licensed under both the Waqf Digital Public License (WaqfDPL-Mulzim 1.0) and GNU AGPLv3.
