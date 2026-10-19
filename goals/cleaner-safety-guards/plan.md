# Plan: Harden Process and Filesystem Cleanup Safety Guards

## Execution Steps
- [x] **Phase 1**: Add PID <= 1 and PID == os.Getpid() defensive skip guard in cleaner.KillProcessTargets
- [x] **Phase 2**: Enforce strict /tmp/ and /var/tmp/ path prefix check in cleaner.PruneTmpTargets
- [x] **Phase 3**: Add pre-removal os.Lstat check in cleaner.PruneTmpTargets to guard against concurrent symlink replacement
- [x] **Phase 4**: Add unit tests in cleaner_test.go covering PID 0, PID 1, PID == self, paths outside /tmp, and symlink targets
- [x] **Phase 5**: Verify test suite with race detector: go test -race ./internal/cleaner
