# Plan: Harden Self-Updater Stream Limits and Temporary File Confinement

## Execution Steps
- [ ] **Phase 1**: Wrap release binary downloads with 50MB LimitReader and checksum downloads with 1MB LimitReader in updater.go
- [ ] **Phase 2**: Scope temporary state cache fallback filename to current UID in CacheFilePath
- [ ] **Phase 3**: Enforce fail-closed temporary file creation in scripts/build installer
- [ ] **Phase 4**: Add unit tests for LimitReader and UID-scoped cache path in updater_test.go
