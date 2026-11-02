# Plan: Compare git-describe dev builds as newer than their base tag

## Execution Steps
- [x] **Phase 1 — Contracts**: Add `gitDescribeSuffix` and a `commitsAhead` field to `parseSemVer`.
- [x] **Phase 2 — Core Implementation**: Order equal triples by prerelease, then commit distance.
- [x] **Phase 3 — Verification**: Add regression cases, run `go test -race ./...` and `go vet ./...`, capture `evidence.md`.
- [x] **Phase 4 — Attribution**: Commit with `(goals/updater-dev-version-compare/goal.md)`, certify, reconcile `goals/README.md`.
