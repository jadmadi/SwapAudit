//go:build linux

package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

func TestMatchTmpPattern(t *testing.T) {
	patterns := []string{
		"cs1*", "sila*", "manara*", "test-bwrap*", "tsx-*", "custom-run",
	}

	tests := []struct {
		name     string
		expected bool
	}{
		{"sila-cli-test-profile", true},
		{"manara-backend", true},
		{"test-bwrap-instance", true},
		{"tsx-1234", true},
		{"custom-run-567", true},
		{"unrelated-file", false},
		{"chrome-cache", false},
	}

	for _, tt := range tests {
		matched := false
		for _, pat := range patterns {
			if MatchTmpPattern(pat, tt.name) {
				matched = true
				break
			}
		}
		if matched != tt.expected {
			t.Errorf("MatchTmpPattern against %q = %v, want %v", tt.name, matched, tt.expected)
		}
	}
}

func TestScanTmpDirectoriesIn(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now()

	// 1. Build cache directory (old -> untouched)
	buildDir := filepath.Join(tmpDir, "go-build123456")
	if err := os.Mkdir(buildDir, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	oldTime := now.Add(-24 * time.Hour)
	if err := os.Chtimes(buildDir, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	// 2. Test run directory (old -> abandoned)
	testDir := filepath.Join(tmpDir, "sila-test-run")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	if err := os.Chtimes(testDir, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	// 3. Active cache directory (recent -> active, cleanable=false)
	activeDir := filepath.Join(tmpDir, "active-cache")
	if err := os.Mkdir(activeDir, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// 4. Excluded daemon dir
	daemonDir := filepath.Join(tmpDir, "tmux-1000")
	if err := os.Mkdir(daemonDir, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	items := ScanTmpDirectoriesIn(tmpDir, now, 12*time.Hour, []string{"sila-*"})

	if len(items) != 3 {
		t.Fatalf("ScanTmpDirectoriesIn found %d items, want 3", len(items))
	}

	foundBuild := false
	foundTest := false
	foundActive := false

	for _, it := range items {
		if it.Name == "go-build123456" {
			foundBuild = true
			if it.Category != model.CatUntouchedCache || !it.Cleanable {
				t.Errorf("go-build123456 category = %v, cleanable = %v, want untouched-caches, true", it.Category, it.Cleanable)
			}
		}
		if it.Name == "sila-test-run" {
			foundTest = true
			if it.Category != model.CatAbandonedRun || !it.Cleanable {
				t.Errorf("sila-test-run category = %v, cleanable = %v, want abandoned-runs, true", it.Category, it.Cleanable)
			}
		}
		if it.Name == "active-cache" {
			foundActive = true
			if it.Category != model.CatActiveCache || it.Cleanable {
				t.Errorf("active-cache category = %v, cleanable = %v, want active-caches, false", it.Category, it.Cleanable)
			}
		}
	}

	if !foundBuild || !foundTest || !foundActive {
		t.Errorf("missing expected items in scan results: build=%v, test=%v, active=%v", foundBuild, foundTest, foundActive)
	}
}

func TestScanTmpDirectoriesNestedMtime(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now()

	parentDir := filepath.Join(tmpDir, "test-build-cache")
	if err := os.Mkdir(parentDir, 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	subDir := filepath.Join(parentDir, "nested-sub")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("Mkdir subDir failed: %v", err)
	}

	recentFile := filepath.Join(subDir, "artifact.dat")
	if err := os.WriteFile(recentFile, []byte("recent content"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	recentTime := now.Add(-10 * time.Minute)
	if err := os.Chtimes(recentFile, recentTime, recentTime); err != nil {
		t.Fatalf("Chtimes recentFile failed: %v", err)
	}

	oldTime := now.Add(-24 * time.Hour)
	if err := os.Chtimes(subDir, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes subDir failed: %v", err)
	}
	if err := os.Chtimes(parentDir, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes parent failed: %v", err)
	}

	items := ScanTmpDirectoriesIn(tmpDir, now, 12*time.Hour, []string{"test-*"})

	if len(items) != 1 {
		t.Fatalf("ScanTmpDirectoriesIn found %d items, want 1", len(items))
	}

	item := items[0]
	if item.Name != "test-build-cache" {
		t.Fatalf("item name = %q, want test-build-cache", item.Name)
	}

	if item.Age < 23*time.Hour {
		t.Errorf("item.Age = %v, expected >= 23h", item.Age)
	}

	if item.LastActive > 30*time.Minute {
		t.Errorf("item.LastActive = %v, expected <= 30m due to nested mtime propagation", item.LastActive)
	}

	if item.Cleanable {
		t.Errorf("item.Cleanable = true, want false because nested file was recently active")
	}

	if item.Category != model.CatActiveCache {
		t.Errorf("item.Category = %v, want CatActiveCache", item.Category)
	}
}

func TestScanTmpDirectoriesSubdirSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now()

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside target"), 0o644); err != nil {
		t.Fatalf("WriteFile outside failed: %v", err)
	}

	topSymlink := filepath.Join(tmpDir, "test-top-symlink")
	if err := os.Symlink(outsideFile, topSymlink); err != nil {
		t.Fatalf("Symlink top failed: %v", err)
	}

	realDir := filepath.Join(tmpDir, "test-real-dir")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("Mkdir realDir failed: %v", err)
	}
	subSymlink := filepath.Join(realDir, "link-to-outside")
	if err := os.Symlink(outsideFile, subSymlink); err != nil {
		t.Fatalf("Symlink inside failed: %v", err)
	}
	regularFile := filepath.Join(realDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("regular 1234"), 0o644); err != nil {
		t.Fatalf("WriteFile regular failed: %v", err)
	}

	items := ScanTmpDirectoriesIn(tmpDir, now, 12*time.Hour, []string{"test-*"})

	if len(items) != 1 {
		t.Fatalf("ScanTmpDirectoriesIn found %d items, want 1", len(items))
	}

	if items[0].Name != "test-real-dir" {
		t.Errorf("item name = %q, want test-real-dir", items[0].Name)
	}
}
