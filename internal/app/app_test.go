//go:build linux

package app

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
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
	io.Copy(&buf, r)
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
	io.Copy(&buf, r)
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
	io.Copy(&buf, r)
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
	io.Copy(&buf, r)
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
	io.Copy(&buf, r)
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
	io.Copy(&buf, r)

	if code != 0 {
		t.Errorf("Run(--no-update-check) exit code = %d, want 0", code)
	}
}
