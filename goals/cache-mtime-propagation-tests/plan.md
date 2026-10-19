# Plan: Expand Cache Nested Directory Mtime Propagation Tests

## Execution Steps
- [ ] **Phase 1**: Add TestScanTmpDirectoriesNestedMtime for deep file timestamp derivation
- [ ] **Phase 2**: Add TestScanTmpDirectoriesSubdirSymlinks for symlink safety in subdirectories
- [ ] **Phase 3**: Verify test suite with race detector: go test -race ./internal/cache
