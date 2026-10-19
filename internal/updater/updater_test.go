//go:build linux

package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"0.1.0", "0.1.0", 0},
		{"v0.1.0", "0.1.0", 0},
		{"v0.1.0", "v0.1.1", -1},
		{"v0.2.0", "v0.1.9", 1},
		{"v1.0.0", "v2.0.0", -1},
		{"v1.0.0-rc.1", "v1.0.0", -1},
		{"v1.0.0", "v1.0.0-rc.1", 1},
		{"dev", "v0.1.0", -1},
		{"v0.1.0", "dev", 1},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.v1, tt.v2), func(t *testing.T) {
			res := CompareVersions(tt.v1, tt.v2)
			if res != tt.expected {
				t.Errorf("CompareVersions(%q, %q) = %d, expected %d", tt.v1, tt.v2, res, tt.expected)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current  string
		target   string
		expected bool
	}{
		{"0.1.0", "0.1.0", false},
		{"0.1.0", "0.2.0", true},
		{"0.2.0", "0.1.0", false},
		{"dev", "v0.1.0", true},
		{"none", "v0.1.0", true},
		{"v0.1.0", "", false},
		{"v0.1.0", "invalid", false},
		{"v0.1.0-dirty", "v0.1.1", true},
		{"v0.1.0", "v0.1.0-rc.1", false},
		{"v0.1.0-rc.1", "v0.1.0", true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.current, tt.target), func(t *testing.T) {
			res := IsNewer(tt.current, tt.target)
			if res != tt.expected {
				t.Errorf("IsNewer(%q, %q) = %v, expected %v", tt.current, tt.target, res, tt.expected)
			}
		})
	}
}

func TestStateSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "state.json")

	now := time.Now().Truncate(time.Second)
	s := &State{
		LastChecked:   now,
		LatestVersion: "v1.2.3",
		ReleaseURL:    "https://github.com/jadmadi/swapaudit/releases/tag/v1.2.3",
	}

	if err := SaveState(path, s); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if loaded.LatestVersion != s.LatestVersion {
		t.Errorf("LatestVersion = %q, expected %q", loaded.LatestVersion, s.LatestVersion)
	}
	if loaded.ReleaseURL != s.ReleaseURL {
		t.Errorf("ReleaseURL = %q, expected %q", loaded.ReleaseURL, s.ReleaseURL)
	}
	if !loaded.LastChecked.Equal(now) {
		t.Errorf("LastChecked = %v, expected %v", loaded.LastChecked, now)
	}
}

func setGitHubBases(t *testing.T, api, web string) {
	t.Helper()
	oldAPI, oldWeb := githubAPIBase, githubWebBase
	githubAPIBase, githubWebBase = api, web
	t.Cleanup(func() {
		githubAPIBase, githubWebBase = oldAPI, oldWeb
	})
}

func TestFetchLatestReleaseAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/test/repo/releases/latest" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name": "v0.2.0", "html_url": "https://github.com/test/repo/releases/tag/v0.2.0"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	setGitHubBases(t, ts.URL, ts.URL)
	latest, releaseURL, err := FetchLatestReleaseContext(context.Background(), "test/repo", ts.Client())
	if err != nil {
		t.Fatalf("FetchLatestReleaseContext() error = %v", err)
	}
	if latest != "v0.2.0" {
		t.Errorf("latest = %q, want v0.2.0", latest)
	}
	if !strings.Contains(releaseURL, "/tag/v0.2.0") {
		t.Errorf("releaseURL = %q, want tag URL", releaseURL)
	}
}

func TestFetchChecksumAndComputeSHA256(t *testing.T) {
	content := []byte("binary data to hash")
	h := sha256.Sum256(content)
	expectedHex := hex.EncodeToString(h[:])

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "swapaudit-linux-amd64")
	if err := os.WriteFile(binPath, content, 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	actualHex, err := computeSHA256(binPath)
	if err != nil {
		t.Fatalf("computeSHA256 failed: %v", err)
	}
	if actualHex != expectedHex {
		t.Errorf("computeSHA256 = %q, expected %q", actualHex, expectedHex)
	}

	// Test checksum file parsing
	checksumsContent := fmt.Sprintf("%s  dist/swapaudit-linux-amd64\nabc123def  dist/swapaudit-linux-arm64\n", expectedHex)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksumsContent))
	}))
	defer ts.Close()

	found := fetchChecksum(ts.Client(), ts.URL, "swapaudit-linux-amd64")
	if found != expectedHex {
		t.Errorf("fetchChecksum = %q, expected %q", found, expectedHex)
	}

	notFound := fetchChecksum(ts.Client(), ts.URL, "swapaudit-linux-riscv64")
	if notFound != "" {
		t.Errorf("fetchChecksum for missing asset = %q, expected empty", notFound)
	}
}

func TestCopyFileAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "source")
	dest := filepath.Join(tmpDir, "destination")

	data := []byte("sample binary content")
	if err := os.WriteFile(src, data, 0o755); err != nil {
		t.Fatalf("WriteFile src failed: %v", err)
	}

	if err := copyFileAtomic(src, dest); err != nil {
		t.Fatalf("copyFileAtomic failed: %v", err)
	}

	destData, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile dest failed: %v", err)
	}
	if string(destData) != string(data) {
		t.Errorf("dest content = %q, expected %q", string(destData), string(data))
	}
}

func TestCheckForUpdateSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/test/repo/releases/latest" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name": "v0.2.0", "html_url": "https://github.com/test/repo/releases/tag/v0.2.0"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	setGitHubBases(t, ts.URL, ts.URL)
	t.Setenv("SWAPAUDIT_REPO", "test/repo")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	info, err := CheckForUpdate(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("CheckForUpdate() error = %v", err)
	}
	if info == nil || info.Latest != "v0.2.0" {
		t.Fatalf("CheckForUpdate() = %+v, want latest v0.2.0", info)
	}
	state, err := LoadState(CacheFilePath())
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if state.LatestVersion != "v0.2.0" {
		t.Errorf("state.LatestVersion = %q, want v0.2.0", state.LatestVersion)
	}
}

func TestCheckForUpdateNoUpdate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name": "v0.1.0", "html_url": "https://github.com/test/repo/releases/tag/v0.1.0"}`))
	}))
	defer ts.Close()

	setGitHubBases(t, ts.URL, ts.URL)
	t.Setenv("SWAPAUDIT_REPO", "test/repo")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	info, err := CheckForUpdate(context.Background(), "v0.1.0")
	if err != nil {
		t.Fatalf("CheckForUpdate() error = %v", err)
	}
	if info != nil {
		t.Errorf("CheckForUpdate() = %+v, want nil for current version", info)
	}
}

func TestCheckForUpdateFailurePreservesState(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)

	statePath := filepath.Join(tmpDir, "swapaudit", "update_check.json")
	if err := SaveState(statePath, &State{
		LastChecked:   time.Now().Add(-time.Hour),
		LatestVersion: "v0.3.0",
		ReleaseURL:    "https://github.com/test/repo/releases/tag/v0.3.0",
	}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusInternalServerError)
	}))
	defer ts.Close()

	setGitHubBases(t, ts.URL, ts.URL)
	t.Setenv("SWAPAUDIT_REPO", "test/repo")

	if _, err := CheckForUpdate(context.Background(), "v0.1.0"); err == nil {
		t.Fatal("CheckForUpdate() error = nil, want failure")
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if state.LatestVersion != "v0.3.0" {
		t.Errorf("state.LatestVersion = %q, want preserved v0.3.0", state.LatestVersion)
	}
}

func TestCheckForUpdateTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(`{"tag_name": "v0.2.0"}`))
	}))
	defer ts.Close()

	setGitHubBases(t, ts.URL, ts.URL)
	t.Setenv("SWAPAUDIT_REPO", "test/repo")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := CheckForUpdate(ctx, "v0.1.0")
	if err == nil {
		t.Fatal("CheckForUpdate() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("CheckForUpdate() error = %v, want deadline exceeded", err)
	}
}

func TestAssetNameFor(t *testing.T) {
	tests := map[string]string{
		"amd64":   "swapaudit-linux-amd64",
		"arm64":   "swapaudit-linux-arm64",
		"arm":     "swapaudit-linux-armv7",
		"riscv64": "swapaudit-linux-riscv64",
	}
	for arch, want := range tests {
		if got := assetNameFor(arch); got != want {
			t.Errorf("assetNameFor(%q) = %q, want %q", arch, got, want)
		}
	}
}

func TestCheckForUpdateCommitHashVersion(t *testing.T) {
	if !IsNewer("1a8479e", "v0.1.0") {
		t.Error("IsNewer(commit hash, release) = false, want true")
	}
}

func TestFetchLatestReleaseWrapper(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name": "v0.2.0", "html_url": "https://github.com/test/repo/releases/tag/v0.2.0"}`))
	}))
	defer ts.Close()

	setGitHubBases(t, ts.URL, ts.URL)
	latest, _, err := FetchLatestRelease("test/repo", ts.Client())
	if err != nil {
		t.Fatalf("FetchLatestRelease() error = %v", err)
	}
	if latest != "v0.2.0" {
		t.Errorf("latest = %q, want v0.2.0", latest)
	}
}

func TestCheckWritable(t *testing.T) {
	if err := checkWritable(t.TempDir()); err != nil {
		t.Errorf("checkWritable(writable dir) error = %v", err)
	}
	if err := checkWritable(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("checkWritable(missing dir) error = nil, want error")
	}
}

func TestUpgradeEndToEnd(t *testing.T) {
	arch := runtime.GOARCH
	if arch == "arm" {
		arch = "armv7"
	}
	assetName := "swapaudit-linux-" + arch
	binaryContent := []byte("new release binary")
	sum := sha256.Sum256(binaryContent)
	checksum := hex.EncodeToString(sum[:])

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/test/repo/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tag_name": "v0.2.0", "html_url": "https://github.com/test/repo/releases/tag/v0.2.0"}`))
		case "/test/repo/releases/download/v0.2.0/" + assetName:
			_, _ = w.Write(binaryContent)
		case "/test/repo/releases/download/v0.2.0/checksums.txt":
			_, _ = w.Write([]byte(checksum + "  " + assetName + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	setGitHubBases(t, ts.URL, ts.URL)
	t.Setenv("SWAPAUDIT_REPO", "test/repo")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("SWAPAUDIT_DEV_SYNC_PATH", "")

	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "swapaudit")
	if err := os.WriteFile(target, []byte("old binary"), 0o755); err != nil {
		t.Fatalf("WriteFile target failed: %v", err)
	}
	oldExecutable := executablePath
	executablePath = func() (string, error) { return target, nil }
	defer func() { executablePath = oldExecutable }()

	var out, errOut bytes.Buffer
	code := Upgrade("v0.1.0", &out, &errOut)
	if code != 0 {
		t.Fatalf("Upgrade() = %d, want 0; stderr=%s", code, errOut.String())
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile target failed: %v", err)
	}
	if string(got) != string(binaryContent) {
		t.Errorf("target content = %q, want %q", got, binaryContent)
	}
	if !strings.Contains(out.String(), "Checksum verified successfully") {
		t.Errorf("upgrade output missing checksum verification: %s", out.String())
	}
}

func TestUpgradeAlreadyCurrent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name": "v0.1.0", "html_url": "https://github.com/test/repo/releases/tag/v0.1.0"}`))
	}))
	defer ts.Close()

	setGitHubBases(t, ts.URL, ts.URL)
	t.Setenv("SWAPAUDIT_REPO", "test/repo")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	var out, errOut bytes.Buffer
	if code := Upgrade("v0.1.0", &out, &errOut); code != 0 {
		t.Fatalf("Upgrade() = %d, want 0; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "already up to date") {
		t.Errorf("upgrade output missing up-to-date message: %s", out.String())
	}
}

func TestUpgradeFetchError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusInternalServerError)
	}))
	defer ts.Close()

	setGitHubBases(t, ts.URL, ts.URL)
	t.Setenv("SWAPAUDIT_REPO", "test/repo")

	var out, errOut bytes.Buffer
	if code := Upgrade("v0.1.0", &out, &errOut); code != 1 {
		t.Fatalf("Upgrade() = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error checking for updates") {
		t.Errorf("upgrade stderr missing fetch error: %s", errOut.String())
	}
}

func TestCacheFilePathUIDFallback(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")

	oldUserHomeDir := userHomeDir
	defer func() { userHomeDir = oldUserHomeDir }()
	userHomeDir = func() (string, error) {
		return "", os.ErrNotExist
	}

	expected := filepath.Join(os.TempDir(), fmt.Sprintf("swapaudit-%d-update-check.json", os.Getuid()))
	actual := CacheFilePath()
	if actual != expected {
		t.Errorf("CacheFilePath() fallback = %q, expected %q", actual, expected)
	}
}

func TestDownloadFileOversizedLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		chunk := make([]byte, 1024*1024) // 1MB chunk
		for i := 0; i < 52; i++ {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "oversized_bin")
	err := downloadFile(ts.Client(), ts.URL, dest)
	if err == nil {
		t.Fatal("expected error downloading stream exceeding 50MB limit, got nil")
	}
	if !strings.Contains(err.Error(), "download size exceeded limit") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFetchChecksumMalformedLines(t *testing.T) {
	malformedContent := strings.Join([]string{
		"",
		"   ",
		"single_field_without_filename",
		"validhash  other-file",
		"onlyoneword",
		"targethash  swapaudit-linux-amd64",
		"trailingline",
	}, "\n")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(malformedContent))
	}))
	defer ts.Close()

	found := fetchChecksum(ts.Client(), ts.URL, "swapaudit-linux-amd64")
	if found != "targethash" {
		t.Errorf("fetchChecksum with malformed content = %q, want %q", found, "targethash")
	}

	notFound := fetchChecksum(ts.Client(), ts.URL, "nonexistent-asset")
	if notFound != "" {
		t.Errorf("fetchChecksum for non-existent asset = %q, want empty", notFound)
	}

	// Test non-200 response handling
	tsErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer tsErr.Close()

	errFound := fetchChecksum(tsErr.Client(), tsErr.URL, "swapaudit-linux-amd64")
	if errFound != "" {
		t.Errorf("fetchChecksum on 404 response = %q, want empty", errFound)
	}
}

func TestDownloadFileHTTPStatusErrors(t *testing.T) {
	tests := []struct {
		status int
	}{
		{http.StatusNotFound},
		{http.StatusInternalServerError},
		{http.StatusForbidden},
		{http.StatusBadGateway},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("HTTP_%d", tt.status), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "error", tt.status)
			}))
			defer ts.Close()

			dest := filepath.Join(t.TempDir(), "error_bin")
			err := downloadFile(ts.Client(), ts.URL, dest)
			if err == nil {
				t.Fatalf("expected error for HTTP status %d, got nil", tt.status)
			}
			expectedCode := fmt.Sprintf("HTTP %d", tt.status)
			if !strings.Contains(err.Error(), expectedCode) {
				t.Errorf("expected error containing %q, got %q", expectedCode, err.Error())
			}
		})
	}
}
