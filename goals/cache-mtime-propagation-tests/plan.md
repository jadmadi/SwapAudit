# Plan: Expand Cache Nested Directory Mtime Propagation Tests

## Execution Steps
- [x] **Phase 1**: Add TestScanTmpDirectoriesNestedMtime for deep file timestamp derivation
- [x] **Phase 2**: Add TestScanTmpDirectoriesSubdirSymlinks for symlink safety in subdirectories
- [x] **Phase 3**: Verify test suite with race detector: go test -race ./internal/cache
