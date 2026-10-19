# Plan: Expand Updater Edge-Case and Download Boundary Tests

## Execution Steps
- [ ] **Phase 1**: Add TestDownloadFileOversizedLimit to verify stream capping
- [ ] **Phase 2**: Add TestFetchChecksumMalformedLines covering truncated and invalid lines
- [ ] **Phase 3**: Add TestDownloadFileHTTPStatusErrors for non-200 responses
- [ ] **Phase 4**: Verify test suite with race detector: go test -race ./internal/updater
