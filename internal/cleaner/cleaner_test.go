//go:build linux

package cleaner

import (
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
	io.Copy(&buf, r)
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
	io.Copy(&buf, r)
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
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Would reclaim 15.0 MB tmp, 40.0 MB swap") {
		t.Errorf("ExecuteCleanup output missing expected reclaim summary: %s", output)
	}
}
