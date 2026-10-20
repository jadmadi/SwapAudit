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
	"time"

	"github.com/jadmadi/swapaudit/internal/cache"
	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
)

// stubCleanupSeams replaces /proc and kill side effects with deterministic test doubles.
func stubCleanupSeams(t *testing.T) {
	t.Helper()
	oldUID := readProcessUID
	oldParent := readParentInfo
	oldOrphan := reclassifyOrphan
	oldTicks := readStartTicks
	oldIdle := readIdle
	oldKill := killProcess
	oldIndex := buildOpenIndex

	readProcessUID = func(pid int) (uint32, error) { return uint32(os.Getuid()), nil }
	readParentInfo = func(pid int) (int, string, bool) { return 1, "systemd", false }
	reclassifyOrphan = func(pid, ppid int) (proc.OrphanEvidence, bool) {
		return proc.OrphanEvidence{Reason: "reparented-terminal-scope"}, true
	}
	ticks := map[int]int64{99999: 12345, 88888: 999, 88889: 1000}
	readStartTicks = func(pid int) int64 {
		if v, ok := ticks[pid]; ok {
			return v
		}
		return 1
	}
	readIdle = func(pid int) (time.Duration, bool) { return 3 * time.Hour, true }
	killProcess = func(pid int) error { return nil }
	buildOpenIndex = func(root string, roots []string) cache.OpenIndex {
		return cache.OpenIndex{Paths: map[string]int{}, Cwds: map[string]int{}, Mapped: map[string]int{}}
	}

	t.Cleanup(func() {
		readProcessUID = oldUID
		readParentInfo = oldParent
		reclassifyOrphan = oldOrphan
		readStartTicks = oldTicks
		readIdle = oldIdle
		killProcess = oldKill
		buildOpenIndex = oldIndex
	})
}

func TestPruneTmpTargetsDryRun(t *testing.T) {
	stubCleanupSeams(t)
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

	reclaimed, _ := PruneTmpTargets(targets, "Test Targets", "Prompt?", CleanupOptions{DryRun: true, PathWidth: 40, Prefix: "[dry-run] "})

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
	stubCleanupSeams(t)
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
	reclaimed, _ := PruneTmpTargets(targets, "Unsafe Targets", "Prompt?", CleanupOptions{Yes: true, PathWidth: 40})
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
	stubCleanupSeams(t)
	targets := []model.SwapItem{
		{PID: 0, Name: "process-group-zero", SwapBytes: 1024 * 1024},
		{PID: 1, Name: "systemd-init", SwapBytes: 1024 * 1024},
		{PID: -1, Name: "broadcast-pid", SwapBytes: 1024 * 1024},
		{PID: os.Getpid(), Name: "swapaudit-self", SwapBytes: 1024 * 1024},
	}

	reclaimed, _ := KillProcessTargets(targets, "Unsafe PIDs", "Prompt?", CleanupOptions{Yes: true, PathWidth: 40})
	if reclaimed != 0 {
		t.Errorf("KillProcessTargets() reclaimed unsafe PIDs = %d, want 0", reclaimed)
	}
}

func TestKillProcessTargetsDryRun(t *testing.T) {
	stubCleanupSeams(t)
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

	oldReadUID := readProcessUID
	readProcessUID = func(pid int) (uint32, error) { return uint32(os.Getuid()), nil }
	defer func() { readProcessUID = oldReadUID }()

	reclaimed, _ := KillProcessTargets(targets, "Agent Processes", "Kill?", CleanupOptions{DryRun: true, PathWidth: 40, Prefix: "[dry-run] "})

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
	stubCleanupSeams(t)
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

	oldReadUID := readProcessUID
	readProcessUID = func(pid int) (uint32, error) { return uint32(os.Getuid()), nil }
	defer func() { readProcessUID = oldReadUID }()

	ExecuteCleanup(items, CleanupFlags{Untouched: true, Abandoned: true, Dormant: true, Orphaned: true}, CleanupOptions{DryRun: true, Yes: true})

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

func TestKillProcessTargetsUIDMismatch(t *testing.T) {
	stubCleanupSeams(t)
	oldReadUID := readProcessUID
	readProcessUID = func(pid int) (uint32, error) { return uint32(os.Getuid()) + 1, nil }
	defer func() { readProcessUID = oldReadUID }()

	targets := []model.SwapItem{
		{PID: 99999, Name: "other-user-agent", SwapBytes: 1024 * 1024 * 20, StartTicks: 12345},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	reclaimed, stats := KillProcessTargets(targets, "UID Mismatch", "Kill?", CleanupOptions{Yes: true, PathWidth: 40})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if reclaimed != 0 {
		t.Errorf("reclaimed = %d, want 0", reclaimed)
	}
	if stats.Reasons["uid-mismatch"] != 1 {
		t.Errorf("uid-mismatch count = %d, want 1; stats=%+v", stats.Reasons["uid-mismatch"], stats)
	}
	if !strings.Contains(output, "owned by UID") {
		t.Errorf("expected UID mismatch output, got: %s", output)
	}
}

func TestKillProcessTargetsAllUsersDryRun(t *testing.T) {
	stubCleanupSeams(t)
	oldReadUID := readProcessUID
	readProcessUID = func(pid int) (uint32, error) { return uint32(os.Getuid()) + 1, nil }
	defer func() { readProcessUID = oldReadUID }()

	targets := []model.SwapItem{
		{PID: 99999, Name: "other-user-agent", SwapBytes: 1024 * 1024 * 20, StartTicks: 12345},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	reclaimed, stats := KillProcessTargets(targets, "All Users", "Kill?", CleanupOptions{
		DryRun: true, Yes: true, AllUsers: true, PathWidth: 40, Prefix: "[dry-run] ",
	})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if reclaimed != 1024*1024*20 {
		t.Errorf("reclaimed = %d, want %d", reclaimed, 1024*1024*20)
	}
	if stats.Ended != 1 {
		t.Errorf("ended = %d, want 1", stats.Ended)
	}
	if !strings.Contains(output, "WARNING: --all-users") || !strings.Contains(output, "would-signal SIGTERM") {
		t.Errorf("expected all-users warning and signal row, got: %s", output)
	}
}

func TestKillProcessTargetsUIDUnreadable(t *testing.T) {
	stubCleanupSeams(t)
	oldReadUID := readProcessUID
	readProcessUID = func(pid int) (uint32, error) { return 0, os.ErrNotExist }
	defer func() { readProcessUID = oldReadUID }()

	targets := []model.SwapItem{
		{PID: 99999, Name: "unknown-owner", SwapBytes: 1024 * 1024},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	reclaimed, stats := KillProcessTargets(targets, "Unreadable UID", "Kill?", CleanupOptions{DryRun: true, Yes: true, PathWidth: 40})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if reclaimed != 0 {
		t.Errorf("reclaimed = %d, want 0", reclaimed)
	}
	if stats.Reasons["uid-unreadable"] != 1 {
		t.Errorf("uid-unreadable count = %d, want 1; stats=%+v", stats.Reasons["uid-unreadable"], stats)
	}
	if !strings.Contains(output, "UID unavailable") {
		t.Errorf("expected UID unavailable output, got: %s", output)
	}
}

func TestExecuteCleanupSummaryReasons(t *testing.T) {
	stubCleanupSeams(t)
	items := []model.SwapItem{
		{Category: model.CatUntouchedCache, Cleanable: true, Path: "/etc/passwd", SizeBytes: 10},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	ExecuteCleanup(items, CleanupFlags{Untouched: true}, CleanupOptions{DryRun: true, Yes: true})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Cleanup summary (dry-run)") {
		t.Errorf("expected dry-run cleanup summary, got: %s", output)
	}
	if !strings.Contains(output, "path-rejected") {
		t.Errorf("expected path-rejected skip reason, got: %s", output)
	}
}

func TestPruneTmpTargetsLiveExecution(t *testing.T) {
	stubCleanupSeams(t)
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

	reclaimed, _ := PruneTmpTargets(targets, "Test Live Cleanup", "Prompt?", CleanupOptions{Yes: true, PathWidth: 40})

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

func TestPruneFooterWithSkips(t *testing.T) {
	stubCleanupSeams(t)
	base := t.TempDir()
	var targets []model.SwapItem
	for i := 0; i < 11; i++ {
		path := filepath.Join(base, "target-"+string(rune('a'+i)))
		if err := os.WriteFile(path, []byte(strings.Repeat("x", 16)), 0o644); err != nil {
			t.Fatal(err)
		}
		targets = append(targets, model.SwapItem{Path: path, SizeBytes: 16})
	}
	targets = append(targets, model.SwapItem{Path: "/etc/passwd", SizeBytes: 1 << 20})

	out := captureCleanerOutput(t, func() {
		PruneTmpTargets(targets, "Footer Targets", "Prompt?", CleanupOptions{Yes: true})
	})
	if !strings.Contains(out, "and 1 smaller targets") {
		t.Errorf("expected exactly one hidden non-skipped target:\n%s", out)
	}
	if strings.Contains(out, "and 2 smaller targets") {
		t.Errorf("skipped target inflated the hidden count:\n%s", out)
	}
	if !strings.Contains(out, "and 1 smaller targets (16 B)") {
		t.Errorf("hidden byte total includes a skipped target or is wrong:\n%s", out)
	}
}

func TestPruneSkipsOpenHandleAndOpenedSinceScan(t *testing.T) {
	stubCleanupSeams(t)
	base := t.TempDir()
	path := filepath.Join(base, "go-build1234")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	buildOpenIndex = func(root string, roots []string) cache.OpenIndex {
		return cache.OpenIndex{Paths: map[string]int{path: 1}, Cwds: map[string]int{}, Mapped: map[string]int{}}
	}

	_, stats := PruneTmpTargets([]model.SwapItem{{Path: path, SizeBytes: 1, OpenHandles: 1}}, "Open", "?", CleanupOptions{DryRun: true, Yes: true})
	if stats.Reasons["open-handle"] != 1 {
		t.Errorf("open-handle count = %d, want 1", stats.Reasons["open-handle"])
	}

	_, stats = PruneTmpTargets([]model.SwapItem{{Path: path, SizeBytes: 1}}, "Opened", "?", CleanupOptions{DryRun: true, Yes: true})
	if stats.Reasons["opened-since-scan"] != 1 {
		t.Errorf("opened-since-scan count = %d, want 1", stats.Reasons["opened-since-scan"])
	}
}

func TestPruneSkipsEvidenceIncomplete(t *testing.T) {
	stubCleanupSeams(t)
	buildOpenIndex = func(root string, roots []string) cache.OpenIndex {
		return cache.OpenIndex{Paths: map[string]int{}, Cwds: map[string]int{}, Mapped: map[string]int{}, Errors: 1}
	}
	path := filepath.Join(t.TempDir(), "go-build1234")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	_, stats := PruneTmpTargets([]model.SwapItem{{Path: path, SizeBytes: 1}}, "Incomplete", "?", CleanupOptions{DryRun: true, Yes: true})
	if stats.Reasons["evidence-incomplete"] != 1 {
		t.Errorf("evidence-incomplete count = %d, want 1", stats.Reasons["evidence-incomplete"])
	}
}

func TestDryRunNoRemove(t *testing.T) {
	stubCleanupSeams(t)
	path := filepath.Join(t.TempDir(), "cache-file")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = PruneTmpTargets([]model.SwapItem{{Path: path, SizeBytes: 4}}, "Dry", "?", CleanupOptions{DryRun: true, Yes: true})
	if _, err := os.Stat(path); err != nil {
		t.Errorf("dry-run removed the target: %v", err)
	}
}

func TestParentAliveSkipAndOverride(t *testing.T) {
	stubCleanupSeams(t)
	readParentInfo = func(pid int) (int, string, bool) { return 42, "bash", true }
	targets := []model.SwapItem{{PID: 99999, Name: "agent", Category: model.CatDormantAgent, SwapBytes: 100, StartTicks: 12345}}

	_, stats := KillProcessTargets(targets, "Parent", "?", CleanupOptions{DryRun: true, Yes: true})
	if stats.Reasons["parent-alive"] != 1 {
		t.Errorf("parent-alive count = %d, want 1", stats.Reasons["parent-alive"])
	}

	out := captureCleanerOutput(t, func() {
		_, stats = KillProcessTargets(targets, "Parent", "?", CleanupOptions{DryRun: true, Yes: true, AllowLiveParent: true})
	})
	if stats.Ended != 1 {
		t.Errorf("allow-live-parent should end the target; stats=%+v", stats)
	}
	if !strings.Contains(out, "WARNING: PID 99999 has a live parent") {
		t.Errorf("expected live-parent warning:\n%s", out)
	}
}

func TestOrphanEvidenceChanged(t *testing.T) {
	stubCleanupSeams(t)
	reclassifyOrphan = func(pid, ppid int) (proc.OrphanEvidence, bool) {
		return proc.OrphanEvidence{Reason: "parent-alive"}, false
	}
	targets := []model.SwapItem{{PID: 99999, Name: "orphan", Category: model.CatAbandonedAgent, SwapBytes: 100, StartTicks: 12345}}
	_, stats := KillProcessTargets(targets, "Orphan", "?", CleanupOptions{DryRun: true, Yes: true})
	if stats.Reasons["orphan-evidence-changed"] != 1 {
		t.Errorf("orphan-evidence-changed count = %d, want 1", stats.Reasons["orphan-evidence-changed"])
	}
}

func TestIdleUnknownSkip(t *testing.T) {
	stubCleanupSeams(t)
	readIdle = func(pid int) (time.Duration, bool) { return -1, false }
	targets := []model.SwapItem{{PID: 99999, Name: "dormant", Category: model.CatDormantAgent, SwapBytes: 100, StartTicks: 12345}}
	_, stats := KillProcessTargets(targets, "Idle", "?", CleanupOptions{DryRun: true, Yes: true})
	if stats.Reasons["idle-unknown"] != 1 {
		t.Errorf("idle-unknown count = %d, want 1", stats.Reasons["idle-unknown"])
	}
}

func TestSignalPrinted(t *testing.T) {
	stubCleanupSeams(t)
	targets := []model.SwapItem{{PID: 99999, Name: "agent", Category: model.CatDormantAgent, SwapBytes: 100, StartTicks: 12345}}
	dry := captureCleanerOutput(t, func() {
		KillProcessTargets(targets, "Signal", "?", CleanupOptions{DryRun: true, Yes: true})
	})
	if !strings.Contains(dry, "would-signal SIGTERM") {
		t.Errorf("dry-run output missing signal text:\n%s", dry)
	}
	live := captureCleanerOutput(t, func() {
		KillProcessTargets(targets, "Signal", "?", CleanupOptions{Yes: true})
	})
	if !strings.Contains(live, "signaled") || !strings.Contains(live, "SIGTERM") {
		t.Errorf("live output missing signal text:\n%s", live)
	}
}

func TestCleanupMiddleTruncation(t *testing.T) {
	stubCleanupSeams(t)
	prefix := "/tmp/TestSharedPrefixThatIsVeryLongAndIdenticalForEveryTarget-"
	suffixes := []string{"aaaa1111", "bbbb2222", "cccc3333", "dddd4444", "eeee5555"}
	var targets []model.SwapItem
	for _, suffix := range suffixes {
		targets = append(targets, model.SwapItem{Path: prefix + suffix, SizeBytes: 1024})
	}
	out := captureCleanerOutput(t, func() {
		PruneTmpTargets(targets, "Truncation", "?", CleanupOptions{DryRun: true, Yes: true, PathWidth: 32})
	})
	for _, suffix := range suffixes {
		if !strings.Contains(out, suffix) {
			t.Errorf("output lost distinguishing suffix %q:\n%s", suffix, out)
		}
	}
}

func TestCleanupResultMatchesSummary(t *testing.T) {
	stubCleanupSeams(t)
	base := t.TempDir()
	path := filepath.Join(base, "cache")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	items := []model.SwapItem{
		{Category: model.CatUntouchedCache, Cleanable: true, Path: path, SizeBytes: 4},
		{Category: model.CatDormantAgent, Cleanable: true, PID: 99999, Name: "agent", SwapBytes: 100, StartTicks: 12345},
	}
	var result CleanupResult
	captureCleanerOutput(t, func() {
		result = ExecuteCleanup(items, CleanupFlags{Untouched: true, Dormant: true}, CleanupOptions{DryRun: true, Yes: true})
	})
	var pruned, ended, skipped int
	for _, g := range result.Groups {
		pruned += g.WouldPrune
		ended += g.WouldEnd
		skipped += g.Skipped
	}
	if pruned != result.Summary.Pruned || ended != result.Summary.Ended || skipped != result.Summary.Skipped {
		t.Errorf("group totals do not match summary: groups=(%d,%d,%d) summary=%+v", pruned, ended, skipped, result.Summary)
	}
}

func TestNoSIGKILL(t *testing.T) {
	data, err := os.ReadFile("cleaner.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "SIGKILL") {
		t.Errorf("cleaner.go must not reference SIGKILL")
	}
}

func captureCleanerOutput(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestCleanupGroupSummary(t *testing.T) {
	stubCleanupSeams(t)
	targets := []model.SwapItem{
		{Name: "TestGoalsStatsFlagConflicts11102814295", Path: "/tmp/TestGoalsStatsFlagConflicts11102814295", SizeBytes: 1024},
		{Name: "TestGoalsStatsFlagConflicts22222222222", Path: "/tmp/TestGoalsStatsFlagConflicts22222222222", SizeBytes: 2048},
	}
	out := captureCleanerOutput(t, func() {
		PruneTmpTargets(targets, "Grouped", "?", CleanupOptions{DryRun: true, Yes: true})
	})
	if !strings.Contains(out, "Groups: TestGoalsStatsFlagConflicts* ×2") {
		t.Errorf("expected grouped cleanup footer:\n%s", out)
	}
}

func TestZeroSwapOrphanCleanupVisible(t *testing.T) {
	stubCleanupSeams(t)
	targets := []model.SwapItem{{
		PID: 99999, Name: "zero-swap-orphan", Category: model.CatAbandonedAgent,
		SwapBytes: 0, StartTicks: 12345,
	}}
	out := captureCleanerOutput(t, func() {
		_, stats := KillProcessTargets(targets, "Zero Swap", "?", CleanupOptions{DryRun: true, Yes: true})
		if stats.Ended != 1 {
			t.Errorf("zero-swap orphan should be listed as would-signal; stats=%+v", stats)
		}
	})
	if !strings.Contains(out, "zero-swap-orphan") || !strings.Contains(out, "would-signal") {
		t.Errorf("zero-swap orphan missing from cleanup preview:\n%s", out)
	}
}
