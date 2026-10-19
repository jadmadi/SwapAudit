//go:build linux

package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/cleaner"
	"github.com/jadmadi/swapaudit/internal/updater"
)

func TestRunVersion(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--version"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run(--version) exit code = %d, want 0", code)
	}
	if !strings.Contains(output, "swapaudit") {
		t.Errorf("Run(--version) output missing 'swapaudit': %q", output)
	}
}

func TestRunInvalidSort(t *testing.T) {
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stderr = w

	code := Run([]string{"--sort", "invalid"})

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 2 {
		t.Errorf("Run(--sort invalid) exit code = %d, want 2", code)
	}
	if !strings.Contains(output, "invalid --sort value") {
		t.Errorf("Run(--sort invalid) stderr missing expected error message: %q", output)
	}
}

func TestRunDryRunCleanAll(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--dry-run", "--clean-all", "--no-color", "--ascii"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run(--dry-run --clean-all) exit code = %d, want 0", code)
	}
	if !strings.Contains(output, "Cleanup Execution (Dry Run)") {
		t.Errorf("Run(--dry-run --clean-all) output missing dry run header: %q", output)
	}
}

func TestRunDefaultExecution(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--sort", "age", "--limit", "3", "--no-color", "--ascii", "--tmp-pattern", "test-*"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run() exit code = %d, want 0", code)
	}
	if !strings.Contains(output, "SWAP & IN-MEMORY CACHE AUDITOR") {
		t.Errorf("Run() output missing audit header: %q", output)
	}
	if !strings.Contains(output, "Quick Maintenance Commands") {
		t.Errorf("Run() without clean flags missing quick commands: %q", output)
	}
}

func TestStringSliceFlag(t *testing.T) {
	var s stringSliceFlag
	if err := s.Set("first"); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}
	if err := s.Set("second"); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}
	if s.String() != "first, second" {
		t.Errorf("String() = %q, want %q", s.String(), "first, second")
	}
}

func TestRunUpgradeHelp(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"upgrade", "--help"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run(upgrade --help) exit code = %d, want 0", code)
	}
	if !strings.Contains(output, "Usage: swapaudit upgrade") {
		t.Errorf("Run(upgrade --help) missing usage line: %q", output)
	}
}

func TestRunNoUpdateCheck(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--no-update-check", "--dry-run", "--limit", "1", "--no-color", "--ascii"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if code != 0 {
		t.Errorf("Run(--no-update-check) exit code = %d, want 0", code)
	}
}

func TestRunCachedUpdateNotice(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	t.Setenv("SWAPAUDIT_NO_UPDATE_CHECK", "")
	t.Setenv("CI", "")

	statePath := filepath.Join(tmpDir, "swapaudit", "update_check.json")
	state := &updater.State{
		LastChecked:   time.Now(),
		LatestVersion: "v99.0.0",
		ReleaseURL:    "https://github.com/jadmadi/swapaudit/releases/tag/v99.0.0",
	}
	if err := updater.SaveState(statePath, state); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--dry-run", "--limit", "1", "--no-color", "--ascii"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run() exit code = %d, want 0", code)
	}
	if !strings.Contains(output, "A new release of swapaudit is available") {
		t.Errorf("expected update notice for cached newer version, got: %q", output)
	}
}

func TestResolveCleanupFlags(t *testing.T) {
	tests := []struct {
		name      string
		cleanAll  bool
		untouched bool
		abandoned bool
		dormant   bool
		orphaned  bool
		include   bool
		want      cleaner.CleanupFlags
	}{
		{"none", false, false, false, false, false, false, cleaner.CleanupFlags{}},
		{"explicit untouched", false, true, false, false, false, false, cleaner.CleanupFlags{Untouched: true}},
		{"clean all excludes orphan", true, false, false, false, false, false, cleaner.CleanupFlags{Untouched: true, Abandoned: true, Dormant: true}},
		{"clean all includes orphan on opt-in", true, false, false, false, false, true, cleaner.CleanupFlags{Untouched: true, Abandoned: true, Dormant: true, Orphaned: true}},
		{"explicit orphan", false, false, false, false, true, false, cleaner.CleanupFlags{Orphaned: true}},
		{"include only is inert", false, false, false, false, false, true, cleaner.CleanupFlags{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveCleanupFlags(tt.cleanAll, tt.untouched, tt.abandoned, tt.dormant, tt.orphaned, tt.include)
			if got != tt.want {
				t.Errorf("ResolveCleanupFlags() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRunCleanAllExcludesOrphanGroup(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--dry-run", "--clean-all", "--no-update-check", "--no-color", "--ascii", "--limit", "1"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run() exit code = %d, want 0", code)
	}
	if strings.Contains(output, "Orphaned Agent Processes") {
		t.Errorf("--clean-all must not include orphaned cleanup, got:\n%s", output)
	}
}

func TestRunCleanAllIncludeOrphaned(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--dry-run", "--clean-all", "--include-orphaned", "--no-update-check", "--no-color", "--ascii", "--limit", "1"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run() exit code = %d, want 0", code)
	}
	if !strings.Contains(output, "Orphaned Agent Processes") {
		t.Errorf("--include-orphaned should include the orphan group, got:\n%s", output)
	}
}

func TestHelpMentionsSafetyFlags(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--help"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run(--help) exit code = %d, want 0", code)
	}
	for _, flag := range []string{"--all-users", "--include-orphaned"} {
		if !strings.Contains(output, flag) {
			t.Errorf("help output missing %s:\n%s", flag, output)
		}
	}
}
