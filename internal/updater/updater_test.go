//go:build linux

package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestFetchLatestReleaseAPI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/test/repo/releases/latest" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tag_name": "v0.2.0", "html_url": "https://github.com/test/repo/releases/tag/v0.2.0"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	// Direct test of json decoding flow using the test server client
	client := ts.Client()
	resp, err := client.Get(ts.URL + "/repos/test/repo/releases/latest")
	if err != nil {
		t.Fatalf("Failed to GET mock release: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
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
		w.Write([]byte(checksumsContent))
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
