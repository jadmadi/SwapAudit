//go:build linux

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/cleaner"
	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
	"github.com/jadmadi/swapaudit/internal/report"
	"github.com/jadmadi/swapaudit/internal/term"
)

func withStdinTerminal(t *testing.T, value bool) {
	t.Helper()
	old := stdinIsTerminal
	stdinIsTerminal = func() bool { return value }
	t.Cleanup(func() { stdinIsTerminal = old })
}

func captureRun(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	oldOut := os.Stdout
	oldErr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wOut
	os.Stderr = wErr
	outCh := make(chan string, 1)
	errCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		outCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		errCh <- buf.String()
	}()
	code := Run(args)
	wOut.Close()
	wErr.Close()
	os.Stdout = oldOut
	os.Stderr = oldErr
	return <-outCh, <-errCh, code
}

func TestNonTTYCleanupRequiresDryRunOrYes(t *testing.T) {
	withStdinTerminal(t, false)
	_, stderr, code := captureRun(t, "--clean-all", "--no-color", "--ascii", "--limit", "1")
	if code != ExitUsage {
		t.Errorf("exit code = %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr, "cleanup requires --dry-run or --yes when stdin is not a terminal") {
		t.Errorf("stderr missing non-TTY error: %q", stderr)
	}
}

func TestNonTTYAuditWithoutCleanupIsAllowed(t *testing.T) {
	withStdinTerminal(t, false)
	_, _, code := captureRun(t, "--no-color", "--ascii", "--limit", "1")
	if code != ExitOK {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestDryRunWithoutCleanupWarns(t *testing.T) {
	_, stderr, code := captureRun(t, "--dry-run", "--no-color", "--ascii", "--limit", "1")
	if code != ExitOK {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "warning: --dry-run has no effect without a cleanup flag") {
		t.Errorf("stderr missing dry-run warning: %q", stderr)
	}
}

func TestYesWithoutCleanupWarns(t *testing.T) {
	_, stderr, code := captureRun(t, "--yes", "--no-color", "--ascii", "--limit", "1")
	if code != ExitOK {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "warning: --yes has no effect without a cleanup flag") {
		t.Errorf("stderr missing yes warning: %q", stderr)
	}
}

func TestOrphanCacheAgeValidation(t *testing.T) {
	_, stderr, code := captureRun(t, "--max-age", "1h", "--orphan-cache-age", "24h")
	if code != ExitUsage {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--orphan-cache-age") || !strings.Contains(stderr, "must be <=") {
		t.Errorf("stderr missing validation message: %q", stderr)
	}
}

func TestJSONAudit(t *testing.T) {
	stdout, stderr, code := captureRun(t, "--json", "--limit", "1")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}
	if strings.Contains(stdout, "\x1b[") {
		t.Errorf("JSON output contains ANSI escapes")
	}
	if strings.Contains(stdout, "SWAP & IN-MEMORY") {
		t.Errorf("JSON output contains the human banner")
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	if v, ok := doc["schema_version"].(float64); !ok || int(v) != 1 {
		t.Errorf("schema_version = %v, want 1", doc["schema_version"])
	}
	for _, key := range []string{"generated_at", "system", "attribution", "reclaim", "population", "categories", "items", "cleanup"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("JSON missing key %q", key)
		}
	}
	if doc["cleanup"] != nil {
		t.Errorf("cleanup = %v, want null for an audit-only run", doc["cleanup"])
	}
	pop := doc["population"].(map[string]any)
	displayed := pop["displayed"].(float64)
	hiddenLimit := pop["hidden_by_limit"].(float64)
	hiddenSize := pop["hidden_by_size_filter"].(float64)
	aggregated := pop["aggregated_rows"].(float64)
	if displayed <= 0 {
		t.Errorf("population.displayed = %v, want > 0", displayed)
	}
	if pop["aggregated_shells"].(float64) <= 0 || aggregated <= 0 {
		t.Errorf("shell aggregation population = (%v shells, %v rows), want > 0",
			pop["aggregated_shells"], aggregated)
	}
	if raw := pop["raw_items"].(float64); raw < displayed+hiddenLimit+hiddenSize {
		t.Errorf("population exceeds raw items: raw %v, displayed %v, hidden %v/%v",
			raw, displayed, hiddenLimit, hiddenSize)
	}
	attribution := doc["attribution"].(map[string]any)
	swap := attribution["swap"].(map[string]any)
	used := doc["system"].(map[string]any)["swap"].(map[string]any)["used_bytes"].(float64)
	attributed := swap["attributed_bytes"].(float64)
	unattributed := swap["unattributed_bytes"].(float64)
	if attributed+unattributed != used {
		t.Errorf("JSON swap does not reconcile: %v + %v != %v", attributed, unattributed, used)
	}
}

func TestJSONCleanupRequiresYesOrDryRun(t *testing.T) {
	withStdinTerminal(t, true)
	stdout, stderr, code := captureRun(t, "--json", "--clean-all", "--limit", "1")
	if code != ExitUsage {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("expected no stdout on rejected JSON cleanup, got %q", stdout)
	}
	if !strings.Contains(stderr, "--json live cleanup requires --dry-run or --yes") {
		t.Errorf("stderr missing JSON cleanup guard: %q", stderr)
	}
}

func TestJSONDryRunCleanup(t *testing.T) {
	stdout, stderr, code := captureRun(t, "--json", "--dry-run", "--clean-all", "--limit", "1")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	cleanup, ok := doc["cleanup"].(map[string]any)
	if !ok {
		t.Fatalf("cleanup = %v, want an object", doc["cleanup"])
	}
	if cleanup["dry_run"] != true {
		t.Errorf("cleanup.dry_run = %v, want true", cleanup["dry_run"])
	}
	if strings.Contains(stdout, "Cleanup Execution") {
		t.Errorf("JSON output contains human cleanup text")
	}
}

func TestJSONUpgradeRejected(t *testing.T) {
	_, stderr, code := captureRun(t, "--json", "upgrade")
	if code != ExitUsage {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--json is not supported for the upgrade command") {
		t.Errorf("stderr missing upgrade rejection: %q", stderr)
	}
}

func TestDryRunFlagOrderIndependent(t *testing.T) {
	if _, _, code := captureRun(t, "--clean-all", "--dry-run", "--no-color", "--ascii", "--limit", "1"); code != ExitOK {
		t.Errorf("--clean-all --dry-run exit = %d, want 0", code)
	}
	if _, _, code := captureRun(t, "--dry-run", "--clean-all", "--no-color", "--ascii", "--limit", "1"); code != ExitOK {
		t.Errorf("--dry-run --clean-all exit = %d, want 0", code)
	}
}

func TestExitRuntimeOnAuditFailure(t *testing.T) {
	old := collectAuditData
	collectAuditData = func(now time.Time, uptimeSec float64, maxAge, idleThreshold, orphanCacheAge time.Duration, tmpPatterns []string, allowGeneric, cleanOrphanedCaches bool) (ScanResult, error) {
		return ScanResult{}, errors.New("synthetic failure")
	}
	defer func() { collectAuditData = old }()

	_, stderr, code := captureRun(t, "--no-color", "--ascii", "--limit", "1")
	if code != ExitRuntime {
		t.Errorf("exit code = %d, want %d", code, ExitRuntime)
	}
	if !strings.Contains(stderr, "synthetic failure") {
		t.Errorf("stderr missing failure: %q", stderr)
	}
}

func TestExitCleanupPartial(t *testing.T) {
	old := executeCleanup
	executeCleanup = func(items []model.SwapItem, flags cleaner.CleanupFlags, opts cleaner.CleanupOptions) cleaner.CleanupResult {
		return cleaner.CleanupResult{DryRun: opts.DryRun, PartialFailure: true}
	}
	defer func() { executeCleanup = old }()

	_, _, code := captureRun(t, "--dry-run", "--clean-all", "--no-color", "--ascii", "--limit", "1")
	if code != ExitCleanupPartial {
		t.Errorf("exit code = %d, want %d", code, ExitCleanupPartial)
	}
}

func TestExpandFlag(t *testing.T) {
	term.SetTermWidth(120)
	defer term.SetTermWidth(100)
	old := collectAuditData
	collectAuditData = func(now time.Time, uptimeSec float64, maxAge, idleThreshold, orphanCacheAge time.Duration, tmpPatterns []string, allowGeneric, cleanOrphanedCaches bool) (ScanResult, error) {
		r := fixtureReport(t)
		r.Items = append(r.Items,
			model.SwapItem{Kind: model.KindCache, Name: "TestGoalsStatsFlagConflicts11102814295", Path: "/tmp/TestGoalsStatsFlagConflicts11102814295", Category: model.CatUntouchedCache, CacheKind: model.CacheRun, AllocatedBytes: 10 << 20, SizeBytes: 10 << 20, Cleanable: true, LastActive: time.Hour, LastActiveKnown: true},
			model.SwapItem{Kind: model.KindCache, Name: "TestGoalsStatsFlagConflicts22222222222", Path: "/tmp/TestGoalsStatsFlagConflicts22222222222", Category: model.CatUntouchedCache, CacheKind: model.CacheRun, AllocatedBytes: 10 << 20, SizeBytes: 10 << 20, Cleanable: true, LastActive: time.Hour, LastActiveKnown: true},
		)
		r.Population.Cleanable += 2
		r.Population.RawItems += 2
		return ScanResult{Items: r.Items, Report: r}, nil
	}
	defer func() { collectAuditData = old }()

	grouped, _, _ := captureRun(t, "--no-color", "--ascii", "--limit", "20")
	if !strings.Contains(grouped, "TestGoalsStatsFlagConflicts* ×2") {
		t.Errorf("expected grouped row:\n%s", grouped)
	}
	expanded, _, _ := captureRun(t, "--no-color", "--ascii", "--limit", "20", "--expand", "TestGoalsStatsFlagConflicts")
	if !strings.Contains(expanded, "11102814295") || !strings.Contains(expanded, "22222222222") || strings.Contains(expanded, "×2") {
		t.Errorf("--expand should show members:\n%s", expanded)
	}
}

func TestZeroSwapOrphanReachesCleanupFlag(t *testing.T) {
	old := collectAuditData
	collectAuditData = func(now time.Time, uptimeSec float64, maxAge, idleThreshold, orphanCacheAge time.Duration, tmpPatterns []string, allowGeneric, cleanOrphanedCaches bool) (ScanResult, error) {
		items := BuildProcessItems(now, []proc.ProcessSnapshot{{
			PID: 99999, Comm: "agy", VmSwap: 0, Orphaned: true,
			Orphan:    proc.OrphanEvidence{Reason: "reparented-terminal-scope"},
			IdleKnown: true,
			Idle:      time.Minute,
		}}, idleThreshold)
		r, err := report.Build(now, model.SystemSwap{}, items, report.Thresholds{IdleThreshold: idleThreshold})
		if err != nil {
			return ScanResult{}, err
		}
		r.Population.Agents = 1
		r.Population.AgentsZeroSwap = 1
		r.Population.AgentsOrphaned = 1
		return ScanResult{Items: items, Report: r}, nil
	}
	defer func() { collectAuditData = old }()

	out, _, code := captureRun(t, "--dry-run", "--clean-orphaned", "--no-color", "--ascii", "--limit", "1")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Orphaned Agent Processes (1 processes, 0 B)") {
		t.Errorf("zero-swap orphan did not reach the cleanup group:\n%s", out)
	}
}

func TestJSONYesCleanup(t *testing.T) {
	old := executeCleanup
	executeCleanup = func(items []model.SwapItem, flags cleaner.CleanupFlags, opts cleaner.CleanupOptions) cleaner.CleanupResult {
		return cleaner.CleanupResult{
			DryRun: false,
			Groups: []cleaner.CleanupGroupResult{{
				Group: "Untouched Build Caches", Targets: 1, Bytes: 1024,
				Pruned: 1, Reasons: []cleaner.ReasonCount{},
			}},
			Summary: cleaner.CleanupStats{Pruned: 1},
		}
	}
	defer func() { executeCleanup = old }()
	withStdinTerminal(t, true)

	stdout, stderr, code := captureRun(t, "--json", "--yes", "--clean-all", "--limit", "1")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	cleanup, ok := doc["cleanup"].(map[string]any)
	if !ok {
		t.Fatalf("cleanup = %v, want object", doc["cleanup"])
	}
	if cleanup["dry_run"] != false {
		t.Errorf("cleanup.dry_run = %v, want false", cleanup["dry_run"])
	}
	if cleanup["summary"].(map[string]any)["pruned"].(float64) != 1 {
		t.Errorf("cleanup summary does not reflect the structured result: %v", cleanup["summary"])
	}
}
