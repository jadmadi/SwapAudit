# Plan: Expand Updater Edge-Case and Download Boundary Tests

## Execution Steps
- [x] **Phase 1**: Add TestDownloadFileOversizedLimit to verify stream capping
- [x] **Phase 2**: Add TestFetchChecksumMalformedLines covering truncated and invalid lines
- [x] **Phase 3**: Add TestDownloadFileHTTPStatusErrors for non-200 responses
- [x] **Phase 4**: Verify test suite with race detector: go test -race ./internal/updater
