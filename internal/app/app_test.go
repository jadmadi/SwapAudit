//go:build linux

package app

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

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

func TestRunDefaultMakesNoNetworkRequest(t *testing.T) {
	oldCheck := checkForUpdate
	called := false
	checkForUpdate = func(ctx context.Context, current string) (*updater.UpdateInfo, error) {
		called = true
		return nil, nil
	}
	defer func() { checkForUpdate = oldCheck }()

	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("SWAPAUDIT_CHECK_UPDATES", "")

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

	if code != 0 {
		t.Errorf("Run() exit code = %d, want 0", code)
	}
	if called {
		t.Error("default run must not perform an update check")
	}
}

func TestRunCheckUpdatesFlag(t *testing.T) {
	oldCheck := checkForUpdate
	called := 0
	checkForUpdate = func(ctx context.Context, current string) (*updater.UpdateInfo, error) {
		called++
		return &updater.UpdateInfo{Current: current, Latest: "v99.0.0"}, nil
	}
	defer func() { checkForUpdate = oldCheck }()

	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("SWAPAUDIT_CHECK_UPDATES", "")

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--check-updates", "--dry-run", "--limit", "1", "--no-color", "--ascii"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run() exit code = %d, want 0", code)
	}
	if called != 1 {
		t.Errorf("checkForUpdate calls = %d, want 1", called)
	}
	if !strings.Contains(output, "A new release of swapaudit is available") {
		t.Errorf("expected update notice, got: %q", output)
	}
}

func TestRunCheckUpdatesEnv(t *testing.T) {
	oldCheck := checkForUpdate
	called := 0
	checkForUpdate = func(ctx context.Context, current string) (*updater.UpdateInfo, error) {
		called++
		return &updater.UpdateInfo{Current: current, Latest: "v99.0.0"}, nil
	}
	defer func() { checkForUpdate = oldCheck }()

	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("SWAPAUDIT_CHECK_UPDATES", "1")

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	code := Run([]string{"--dry-run", "--limit", "1", "--no-color", "--ascii"})

	w.Close()
	os.Stdout = oldStdout
	_, _ = io.Copy(io.Discard, r)

	if code != 0 {
		t.Errorf("Run() exit code = %d, want 0", code)
	}
	if called != 1 {
		t.Errorf("checkForUpdate calls = %d, want 1", called)
	}
}

func TestRunVersionSkipsUpdateCheck(t *testing.T) {
	oldCheck := checkForUpdate
	called := false
	checkForUpdate = func(ctx context.Context, current string) (*updater.UpdateInfo, error) {
		called = true
		return nil, nil
	}
	defer func() { checkForUpdate = oldCheck }()

	if code := Run([]string{"--version"}); code != 0 {
		t.Errorf("Run(--version) exit code = %d, want 0", code)
	}
	if called {
		t.Error("--version must not perform an update check")
	}
}

func TestRunUpgradeHelpSkipsUpdateCheck(t *testing.T) {
	oldCheck := checkForUpdate
	called := false
	checkForUpdate = func(ctx context.Context, current string) (*updater.UpdateInfo, error) {
		called = true
		return nil, nil
	}
	defer func() { checkForUpdate = oldCheck }()

	if code := Run([]string{"upgrade", "--help"}); code != 0 {
		t.Errorf("Run(upgrade --help) exit code = %d, want 0", code)
	}
	if called {
		t.Error("upgrade --help must not perform an update check")
	}
}

func TestRunInvalidSortSkipsUpdateCheck(t *testing.T) {
	oldCheck := checkForUpdate
	called := false
	checkForUpdate = func(ctx context.Context, current string) (*updater.UpdateInfo, error) {
		called = true
		return nil, nil
	}
	defer func() { checkForUpdate = oldCheck }()

	if code := Run([]string{"--sort", "invalid", "--check-updates"}); code != 2 {
		t.Errorf("Run(invalid sort) exit code = %d, want 2", code)
	}
	if called {
		t.Error("invalid sort must be rejected before an update check")
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

	code := Run([]string{"--dry-run", "--clean-all", "--no-color", "--ascii", "--limit", "1"})

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

	code := Run([]string{"--dry-run", "--clean-all", "--include-orphaned", "--no-color", "--ascii", "--limit", "1"})

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
