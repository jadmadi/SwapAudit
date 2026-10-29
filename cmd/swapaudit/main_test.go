//go:build linux

package main_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var testBinaryPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "swapaudit-e2e-*")
	if err != nil {
		panic(err)
	}

	testBinaryPath = filepath.Join(tmpDir, "swapaudit")
	cmd := exec.Command("go", "build", "-o", testBinaryPath, "github.com/jadmadi/swapaudit/cmd/swapaudit")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		panic(string(out))
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(testBinaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "SWAPAUDIT_NO_UPDATE_CHECK=1")

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s %v: %v", testBinaryPath, args, err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

func TestE2E_Version(t *testing.T) {
	stdout, _, code := runCLI(t, "--version")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout, "swapaudit") {
		t.Errorf("expected version output to contain 'swapaudit', got: %q", stdout)
	}
}

func TestE2E_Help(t *testing.T) {
	stdout, _, code := runCLI(t, "--help")
	if code != 0 {
		t.Fatalf("expected exit code 0 on --help, got %d", code)
	}
	if !strings.Contains(stdout, "USAGE:") || !strings.Contains(stdout, "swapaudit") {
		t.Errorf("expected help output to contain USAGE and swapaudit, got: %q", stdout)
	}
}

func TestE2E_UpgradeHelp(t *testing.T) {
	stdout, _, code := runCLI(t, "upgrade", "--help")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout, "Usage: swapaudit upgrade") {
		t.Errorf("expected upgrade usage string, got: %q", stdout)
	}
}

func TestE2E_InvalidSort(t *testing.T) {
	_, stderr, code := runCLI(t, "--sort", "invalid_column")
	if code != 2 {
		t.Fatalf("expected exit code 2 on invalid sort, got %d", code)
	}
	if !strings.Contains(stderr, "invalid --sort value") {
		t.Errorf("expected error message in stderr, got: %q", stderr)
	}
}

func TestE2E_AuditReportExecution(t *testing.T) {
	stdout, _, code := runCLI(t, "--dry-run", "--limit", "3", "--no-color", "--ascii")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	requiredSubstrings := []string{
		"SWAP & IN-MEMORY CACHE AUDITOR",
		"Swap",
		"RAM",
		"/tmp",
		"Category Breakdown",
	}

	for _, req := range requiredSubstrings {
		if !strings.Contains(stdout, req) {
			t.Errorf("expected audit report to contain %q, but missing from:\n%s", req, stdout)
		}
	}

	// Verify no ANSI escape codes are emitted when --no-color is specified
	if strings.Contains(stdout, "\x1b[") {
		t.Errorf("expected no ANSI color codes in --no-color output")
	}
}

func TestE2E_DryRunCleanAll(t *testing.T) {
	stdout, _, code := runCLI(t, "--dry-run", "--clean-all", "--no-color", "--ascii")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout, "Cleanup Execution (Dry Run)") {
		t.Errorf("expected dry run execution banner, got:\n%s", stdout)
	}
}
