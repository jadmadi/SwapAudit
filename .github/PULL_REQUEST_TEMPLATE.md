## Summary

<!-- What changed and why? Link the goal or issue. -->

## Verification

- [ ] `gofmt -l .` is empty.
- [ ] `go vet ./...` passes.
- [ ] `go test -race ./...` passes.
- [ ] `./scripts/check.sh --full` passes.
- [ ] Docs are updated.
- [ ] No secrets, tokens, or local machine paths are included.

## Scope

- [ ] This is a bug fix, safety fix, documentation change, or distribution fix.
- [ ] This does not add a feature during the demand-validation freeze.
- [ ] If it changes cleanup behavior, it includes a dry-run/test that cannot signal a real user process.

## Goal reference

<!-- goals/<slug>/goal.md, or "not a goal". -->
