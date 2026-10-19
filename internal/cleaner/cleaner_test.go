//go:build linux

package cleaner

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jadmadi/swapaudit/internal/model"
)

func TestPruneTmpTargetsDryRun(t *testing.T) {
	targets := []model.SwapItem{
		{
			Path:      "/tmp/fake-test-run-1",
			SizeBytes: 1024 * 1024 * 10,
		},
		{
			Path:      "/tmp/fake-test-run-2",
			SizeBytes: 1024 * 1024 * 5,
		},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	reclaimed := PruneTmpTargets(targets, "Test Targets", "Prompt?", true, false, 40, "[dry-run] ")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if reclaimed != 1024*1024*15 {
		t.Errorf("PruneTmpTargets() reclaimed = %d, want %d", reclaimed, 1024*1024*15)
	}
	if !strings.Contains(output, "[dry-run]") {
		t.Errorf("PruneTmpTargets() output missing [dry-run] prefix: %s", output)
	}
	if !strings.Contains(output, "fake-test-run-1") {
		t.Errorf("PruneTmpTargets() output missing target: %s", output)
	}
}

func TestPruneTmpTargetsUnsafeGuards(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real file outside tmp
	outsideFile := filepath.Join(t.TempDir(), "outside-run")
	_ = os.WriteFile(outsideFile, []byte("data"), 0o644)

	// Create a symlink inside tmpDir pointing to outside
	symlinkPath := filepath.Join(tmpDir, "symlink-run")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Fatalf("Symlink creation failed: %v", err)
	}

	targets := []model.SwapItem{
		{Path: "", SizeBytes: 100},
		{Path: "/", SizeBytes: 100},
		{Path: "/tmp", SizeBytes: 100},
		{Path: "/var/tmp", SizeBytes: 100},
		{Path: "/etc/passwd", SizeBytes: 100},
		{Path: "/home/user/project", SizeBytes: 100},
		{Path: "/var/log/syslog", SizeBytes: 100},
		{Path: "relative/path", SizeBytes: 100},
		{Path: symlinkPath, SizeBytes: 100},
	}
	reclaimed := PruneTmpTargets(targets, "Unsafe Targets", "Prompt?", false, true, 40, "")
	if reclaimed != 0 {
		t.Errorf("PruneTmpTargets() reclaimed unsafe targets = %d, want 0", reclaimed)
	}

	// Verify the outside target file and symlink were preserved
	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Errorf("Symlink path should have been skipped and preserved: %v", err)
	}
	if _, err := os.Stat(outsideFile); err != nil {
		t.Errorf("Outside file should never be touched: %v", err)
	}
}

func TestKillProcessTargetsUnsafeGuards(t *testing.T) {
	targets := []model.SwapItem{
		{PID: 0, Name: "process-group-zero", SwapBytes: 1024 * 1024},
		{PID: 1, Name: "systemd-init", SwapBytes: 1024 * 1024},
		{PID: -1, Name: "broadcast-pid", SwapBytes: 1024 * 1024},
		{PID: os.Getpid(), Name: "swapaudit-self", SwapBytes: 1024 * 1024},
	}

	reclaimed := KillProcessTargets(targets, "Unsafe PIDs", "Prompt?", false, true, 40, "")
	if reclaimed != 0 {
		t.Errorf("KillProcessTargets() reclaimed unsafe PIDs = %d, want 0", reclaimed)
	}
}

func TestKillProcessTargetsDryRun(t *testing.T) {
	targets := []model.SwapItem{
		{
			PID:        99999,
			Name:       "dummy-agent",
			SwapBytes:  1024 * 1024 * 20,
			StartTicks: 12345,
		},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	reclaimed := KillProcessTargets(targets, "Agent Processes", "Kill?", true, false, 40, "[dry-run] ")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if reclaimed != 1024*1024*20 {
		t.Errorf("KillProcessTargets() reclaimed = %d, want %d", reclaimed, 1024*1024*20)
	}
	if !strings.Contains(output, "[dry-run]") {
		t.Errorf("KillProcessTargets() output missing [dry-run] prefix: %s", output)
	}
	if !strings.Contains(output, "dummy-agent") {
		t.Errorf("KillProcessTargets() output missing process name: %s", output)
	}
}

func TestExecuteCleanupDryRun(t *testing.T) {
	items := []model.SwapItem{
		{
			Category:  model.CatUntouchedCache,
			Cleanable: true,
			Path:      "/tmp/fake-cache",
			SizeBytes: 10 * 1024 * 1024,
		},
		{
			Category:  model.CatAbandonedRun,
			Cleanable: true,
			Path:      "/tmp/fake-run",
			SizeBytes: 5 * 1024 * 1024,
		},
		{
			Category:   model.CatAbandonedAgent,
			Cleanable:  true,
			PID:        88888,
			Name:       "dead-parent-agent",
			SwapBytes:  25 * 1024 * 1024,
			StartTicks: 999,
		},
		{
			Category:   model.CatDormantAgent,
			Cleanable:  true,
			PID:        88889,
			Name:       "dormant-agent",
			SwapBytes:  15 * 1024 * 1024,
			StartTicks: 1000,
		},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	ExecuteCleanup(items, true, true, true, true, true, true)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Would reclaim 15.0 MB tmp, 40.0 MB swap") {
		t.Errorf("ExecuteCleanup output missing expected reclaim summary: %s", output)
	}
}

func TestAskConfirmationReaderVariants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"yes_lowercase", "yes\n", true},
		{"yes_uppercase", "YES\n", true},
		{"yes_mixed", "Yes\n", true},
		{"y_lowercase", "y\n", true},
		{"y_uppercase", "Y\n", true},
		{"y_with_spaces", "  y  \n", true},
		{"no_lowercase", "no\n", false},
		{"no_uppercase", "NO\n", false},
		{"n_lowercase", "n\n", false},
		{"n_uppercase", "N\n", false},
		{"empty_enter", "\n", false},
		{"spaces_only", "   \n", false},
		{"arbitrary_text", "maybe\n", false},
		{"eof_empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			oldStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("os.Pipe failed: %v", err)
			}
			os.Stdout = w

			got := AskConfirmationReader(reader, "Continue?")

			w.Close()
			os.Stdout = oldStdout
			_, _ = io.Copy(io.Discard, r)

			if got != tt.expected {
				t.Errorf("AskConfirmationReader with input %q = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPruneTmpTargetsLiveExecution(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "file1.cache")
	if err := os.WriteFile(file1, []byte("cache data 1"), 0o644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}

	dir1 := filepath.Join(tempDir, "nested-dir")
	if err := os.MkdirAll(dir1, 0o755); err != nil {
		t.Fatalf("failed to create dir1: %v", err)
	}
	nestedFile := filepath.Join(dir1, "subfile.txt")
	if err := os.WriteFile(nestedFile, []byte("nested"), 0o644); err != nil {
		t.Fatalf("failed to create nestedFile: %v", err)
	}

	targets := []model.SwapItem{
		{
			Path:      file1,
			SizeBytes: 12,
		},
		{
			Path:      dir1,
			SizeBytes: 6,
		},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	reclaimed := PruneTmpTargets(targets, "Test Live Cleanup", "Prompt?", false, true, 40, "")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if reclaimed != 18 {
		t.Errorf("PruneTmpTargets() reclaimed = %d, want %d", reclaimed, 18)
	}
	if !strings.Contains(output, "pruned") {
		t.Errorf("PruneTmpTargets() output missing 'pruned': %s", output)
	}

	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Errorf("expected file1 to be removed, but still exists: %v", err)
	}
	if _, err := os.Stat(dir1); !os.IsNotExist(err) {
		t.Errorf("expected dir1 to be removed, but still exists: %v", err)
	}
}
