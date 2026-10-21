//go:build linux

package app

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/snapshot"
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

func captureRun(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr

	outFile, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	errFile, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}

	os.Stdout, os.Stderr = outFile, errFile
	code := Run(args)
	outFile.Close()
	errFile.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr

	outBytes, err := os.ReadFile(outFile.Name())
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	errBytes, err := os.ReadFile(errFile.Name())
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	return code, string(outBytes), string(errBytes)
}

func TestModeValidation(t *testing.T) {
	cases := []struct {
		name string
		flag modeFlags
		want string
	}{
		{"json cleanup", modeFlags{json: true, cleanup: true}, "--json is read-only and cannot be combined with cleanup flags"},
		{"json dry-run", modeFlags{json: true, dryRun: true}, "--json is read-only and cannot be combined with --dry-run"},
		{"json yes", modeFlags{json: true, yes: true}, "--json is read-only and cannot be combined with --yes"},
		{"no-command alone", modeFlags{noCommand: true}, "--no-command requires --json"},
		{"json interactive", modeFlags{json: true, interactive: true}, "--json cannot be combined with --interactive"},
		{"interactive cleanup", modeFlags{interactive: true, cleanup: true}, "--interactive cannot be combined with cleanup flags"},
		{"interactive yes", modeFlags{interactive: true, yes: true}, "--interactive cannot be combined with --yes"},
		{"interactive both", modeFlags{interactive: true, noInteractive: true}, "--interactive and --no-interactive are mutually exclusive"},
		{"valid json", modeFlags{json: true, noCommand: true}, ""},
		{"valid plain", modeFlags{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateModes(tc.flag)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("validateModes() = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tc.want {
				t.Fatalf("validateModes() = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRunJSONStreamPurity(t *testing.T) {
	code, stdout, stderr := captureRun(t, []string{"--json"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout)
	}
	if doc["schema_version"].(float64) != 1 {
		t.Error("schema_version must be 1")
	}
	for _, bad := range []string{"\x1b", "Cleanable", "SWAP & IN-MEMORY", "+--", "A new release"} {
		if strings.Contains(stdout, bad) {
			t.Errorf("stdout contains human output %q", bad)
		}
	}
	if !strings.HasSuffix(stdout, "}\n") {
		t.Error("stdout must end with exactly one trailing newline")
	}
}

func TestRunJSONNoCommand(t *testing.T) {
	code, stdout, stderr := captureRun(t, []string{"--json", "--no-command"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	var doc snapshot.Snapshot
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	for _, item := range doc.Items {
		if item.Command != "" {
			t.Errorf("command for %q = %q, want empty", item.Name, item.Command)
		}
	}
}

func TestRunJSONConflicts(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--json", "--clean-untouched"}, "--json is read-only and cannot be combined with cleanup flags"},
		{[]string{"--json", "--clean-abandoned"}, "--json is read-only and cannot be combined with cleanup flags"},
		{[]string{"--json", "--clean-dormant"}, "--json is read-only and cannot be combined with cleanup flags"},
		{[]string{"--json", "--clean-orphaned"}, "--json is read-only and cannot be combined with cleanup flags"},
		{[]string{"--json", "--clean-tmp"}, "--json is read-only and cannot be combined with cleanup flags"},
		{[]string{"--json", "--clean-all"}, "--json is read-only and cannot be combined with cleanup flags"},
		{[]string{"--json", "--dry-run"}, "--json is read-only and cannot be combined with --dry-run"},
		{[]string{"--json", "--yes"}, "--json is read-only and cannot be combined with --yes"},
		{[]string{"--json", "-y"}, "--json is read-only and cannot be combined with --yes"},
		{[]string{"--json", "--interactive"}, "--json cannot be combined with --interactive"},
		{[]string{"--interactive", "--yes"}, "--interactive cannot be combined with --yes"},
		{[]string{"--interactive", "--clean-all"}, "--interactive cannot be combined with cleanup flags"},
		{[]string{"--interactive", "--no-interactive"}, "--interactive and --no-interactive are mutually exclusive"},
		{[]string{"--no-command"}, "--no-command requires --json"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			code, stdout, stderr := captureRun(t, tc.args)
			if code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want substring %q", stderr, tc.want)
			}
		})
	}
}

func TestRunJSONHardFailure(t *testing.T) {
	old := procStatusPath
	procStatusPath = filepath.Join(t.TempDir(), "missing-status")
	defer func() { procStatusPath = old }()

	code, stdout, stderr := captureRun(t, []string{"--json"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	var doc snapshot.ErrorDocument
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not the error document: %v\n%s", err, stdout)
	}
	if doc.SchemaVersion != 1 || doc.Error.Code != "proc_unavailable" || doc.Error.Message != "cannot read /proc/self/status" {
		t.Errorf("unexpected error document: %+v", doc)
	}
}

func TestRunJSONAcceptsCosmeticFlags(t *testing.T) {
	code, stdout, stderr := captureRun(t, []string{"--json", "--no-color", "--ascii"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if strings.Contains(stdout, "\x1b") {
		t.Error("--ascii/--no-color must not alter or color the document")
	}
}

func TestRunJSONNeverExecutesCleanup(t *testing.T) {
	old := executeCleanup
	called := false
	executeCleanup = func([]model.SwapItem, bool, bool, bool, bool, bool, bool) { called = true }
	defer func() { executeCleanup = old }()

	code, _, stderr := captureRun(t, []string{"--json"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if called {
		t.Error("--json must never reach the cleanup executor")
	}
}

func TestRunJSONSuppressesUpdateNotice(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	statePath := filepath.Join(tmpDir, "swapaudit", "update_check.json")
	if err := updater.SaveState(statePath, &updater.State{
		LastChecked:   time.Now(),
		LatestVersion: "v99.0.0",
	}); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	code, stdout, stderr := captureRun(t, []string{"--json"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "A new release") {
		t.Error("JSON mode must suppress the update notice")
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
}

func TestRunForcedInteractiveRequiresTTY(t *testing.T) {
	code, stdout, stderr := captureRun(t, []string{"--interactive"})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "error: --interactive requires a terminal on stdin and stdout") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunSkipsPickerOnNonTTY(t *testing.T) {
	code, stdout, stderr := captureRun(t, []string{"--no-update-check", "--limit", "1", "--no-color", "--ascii"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "ACTION PICKER") {
		t.Error("picker must not appear when stdin/stdout are not terminals")
	}
	if !strings.Contains(stdout, "Quick Maintenance Commands") {
		t.Error("redirected output must still contain the quick commands")
	}
}

func TestPickerEligibility(t *testing.T) {
	if !pickerEligible(true, false) {
		t.Error("--interactive must force eligibility after the TTY check")
	}
	if pickerEligible(false, true) {
		t.Error("--no-interactive must disable the picker")
	}
	t.Setenv("SWAPAUDIT_NO_INTERACTIVE", "1")
	if pickerEligible(false, false) {
		t.Error("SWAPAUDIT_NO_INTERACTIVE must disable the default picker")
	}
	if !pickerEligible(true, false) {
		t.Error("explicit --interactive must not be overridden by the environment")
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
	io.Copy(&buf, r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Run() exit code = %d, want 0", code)
	}
	if !strings.Contains(output, "A new release of swapaudit is available") {
		t.Errorf("expected update notice for cached newer version, got: %q", output)
	}
}
