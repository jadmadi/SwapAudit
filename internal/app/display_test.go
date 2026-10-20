//go:build linux

package app

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
	"github.com/jadmadi/swapaudit/internal/report"
	"github.com/jadmadi/swapaudit/internal/term"
)

func captureOutput(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func captureOutputAsync(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	ch := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		ch <- buf.String()
	}()
	f()
	w.Close()
	os.Stdout = old
	return <-ch
}

func fixtureReport(t *testing.T) report.AuditReport {
	t.Helper()
	now := time.Unix(1700000000, 0)
	items := []model.SwapItem{
		{
			Kind: model.KindProcess, PID: 101, Name: "agy", Category: model.CatDormantAgent,
			SwapBytes: 500 << 20, Age: 48 * time.Hour, LastActive: 3 * time.Hour, LastActiveKnown: true,
			Cleanable: true, Agent: true, Signal: "SIGTERM", ParentPID: 1, ParentComm: "systemd",
			TTYNr: 123, Cgroup: "/user.slice/session-1.scope", OrphanReason: "idle-threshold",
			Evidence: "idle=3h parent=dead",
		},
		{
			Kind: model.KindProcess, PID: 102, Name: "big-app", Category: model.CatActiveProcess,
			SwapBytes: 200 << 20, Age: 24 * time.Hour, LastActive: 10 * time.Minute, LastActiveKnown: true,
		},
		{
			Kind: model.KindCache, Name: "go-build1234", Path: "/tmp/go-build1234",
			Category: model.CatUntouchedCache, CacheKind: model.CacheBuild,
			AllocatedBytes: 300 << 20, SizeBytes: 300 << 20, ApparentBytes: 300 << 20,
			Age: 48 * time.Hour, LastActive: 13 * time.Hour, LastActiveKnown: true, Cleanable: true,
			Evidence: "idle=13h handles=0 kind=build",
		},
		{
			Kind: model.KindCache, Name: "inuse-cache", Path: "/tmp/inuse-cache",
			Category: model.CatInUseCache, CacheKind: model.CacheGeneric,
			AllocatedBytes: 100 << 20, SizeBytes: 100 << 20, Age: 48 * time.Hour,
			LastActive: time.Hour, LastActiveKnown: true, OpenHandles: 2, Evidence: "handles=2",
		},
		{
			Kind: model.KindCache, Name: "small-protected", Path: "/tmp/small-protected",
			Category: model.CatRecentCache, CacheKind: model.CacheRun,
			AllocatedBytes: 1 << 20, SizeBytes: 1 << 20, Age: time.Hour,
			LastActive: 10 * time.Minute, LastActiveKnown: true,
		},
	}
	system := model.SystemSwap{
		SwapTotal: 10 << 30, SwapUsed: 5 << 30, SwapFree: 5 << 30,
		MemTotal: 16 << 30, MemUsed: 8 << 30, MemAvail: 8 << 30,
		TmpfsSize: 20 << 30, TmpfsUsed: 10 << 30, TmpfsFree: 10 << 30,
	}
	r, err := report.Build(now, system, items, report.Thresholds{
		MaxAge: 12 * time.Hour, IdleThreshold: 2 * time.Hour, OrphanCacheAge: time.Hour,
	})
	if err != nil {
		t.Fatalf("report.Build: %v", err)
	}
	r.Host = report.Host{KernelRelease: "test", UptimeSeconds: 1000}
	r.Population = report.Population{
		RawItems: len(items), Cleanable: 2, Protected: 3,
		Processes: 200, ProcessesWithSwap: 20, Agents: 3, AgentsWithSwap: 1,
		AgentsZeroSwap: 2, AgentsDormant: 1, AgentsOrphaned: 0, AgentsParentAlive: 1,
		AgentsIdleUnknown: 1, AgentsActive: 1,
	}
	r.Scan = report.Scan{InUseCaches: 1, OrphanedCaches: 0, RecentCaches: 1}
	return r
}

func TestFormatVerdict(t *testing.T) {
	r := fixtureReport(t)
	got := FormatVerdict(r)
	for _, want := range []string{"Verdict:", "reclaimable", "tmpfs", "swap", "biggest win", "thresholds:", "upper bound"} {
		if !strings.Contains(got, want) {
			t.Errorf("FormatVerdict() missing %q: %s", want, got)
		}
	}

	empty := fixtureReport(t)
	empty.Items = nil
	empty.Reclaim = report.Reclaim{}
	empty.Population.Cleanable = 0
	got = FormatVerdict(empty)
	if !strings.Contains(got, "nothing safe to reclaim") {
		t.Errorf("empty verdict = %q, want nothing safe to reclaim", got)
	}
}

func TestNoProcessSwapRAMLabel(t *testing.T) {
	r := fixtureReport(t)
	out := captureOutput(t, func() {
		PrintHeader(r)
		PrintCategorySummary(r)
	})
	banned := "Process Swap " + "(RAM)"
	if strings.Contains(out, banned) {
		t.Errorf("banned process-swap RAM label found")
	}
	if !strings.Contains(out, "Process swap (VmSwap)") {
		t.Errorf("expected Process swap (VmSwap) label")
	}
	retired := "active-" + "caches"
	if strings.Contains(out, retired) {
		t.Errorf("banned retired cache label found")
	}
}

func TestCleanableSplit(t *testing.T) {
	r := fixtureReport(t)
	out := captureOutput(t, func() { PrintActionableSection(r, ViewOptions{Limit: 12}) })
	for _, want := range []string{"Reclaimable:", "tmpfs", "swap", "upper bound"} {
		if !strings.Contains(out, want) {
			t.Errorf("actionable output missing %q:\n%s", want, out)
		}
	}
}

func TestActionablePopulationLine(t *testing.T) {
	r := fixtureReport(t)
	out := captureOutput(t, func() { PrintActionableSection(r, ViewOptions{Limit: 1}) })
	if !strings.Contains(out, "Actionable population: 2 cleanable") {
		t.Errorf("missing actionable population line:\n%s", out)
	}
	if !strings.Contains(out, "hidden") {
		t.Errorf("expected hidden count with limit 1:\n%s", out)
	}
}

func TestAllModeNoHiddenFooter(t *testing.T) {
	r := fixtureReport(t)
	out := captureOutput(t, func() {
		PrintActionableSection(r, ViewOptions{ShowAll: true, Limit: 0})
		PrintProtectedTable(r, ViewOptions{ShowAll: true, Limit: 0})
	})
	if strings.Contains(out, "more groups") || strings.Contains(out, "more hidden") {
		t.Errorf("--all output must not contain a hidden footer:\n%s", out)
	}
}

func TestAllModeShowsSmallItems(t *testing.T) {
	r := fixtureReport(t)
	normal := captureOutput(t, func() { PrintProtectedTable(r, ViewOptions{Limit: 12}) })
	if strings.Contains(normal, "small-protected") {
		t.Errorf("small protected item should be filtered without --all:\n%s", normal)
	}
	all := captureOutput(t, func() { PrintProtectedTable(r, ViewOptions{ShowAll: true, Limit: 0}) })
	if !strings.Contains(all, "small-protected") {
		t.Errorf("--all should show the small protected item:\n%s", all)
	}
}

func TestNoDuplicateRows(t *testing.T) {
	r := fixtureReport(t)
	actionable := captureOutput(t, func() { PrintActionableSection(r, ViewOptions{Limit: 12}) })
	protected := captureOutput(t, func() { PrintProtectedTable(r, ViewOptions{Limit: 12}) })
	if strings.Contains(protected, "agy") {
		t.Errorf("cleanable item 'agy' appears in the protected table")
	}
	if strings.Contains(actionable, "inuse-cache") {
		t.Errorf("protected item 'inuse-cache' appears in the actionable table")
	}
}

func TestThresholdLabelsAndPressureLegend(t *testing.T) {
	r := fixtureReport(t)
	out := captureOutput(t, func() {
		PrintHeader(r)
		PrintCategorySummary(r)
	})
	for _, want := range []string{"green <50%", "red ≥80%", "dormant > 2h", "stale > 12h", "orphan-cache > 1h"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestWideMode(t *testing.T) {
	term.SetTermWidth(80)
	term.SetAsciiMode(true)
	term.SetColorEnabled(false)
	defer term.SetTermWidth(100)

	r := fixtureReport(t)
	longName := "go-build-very-long-distinguishing-suffix-1234567890"
	r.Items = append(r.Items, model.SwapItem{
		Kind: model.KindCache, Name: longName, Path: "/tmp/" + longName,
		Category: model.CatRecentCache, CacheKind: model.CacheBuild,
		AllocatedBytes: 50 << 20, SizeBytes: 50 << 20, Age: time.Hour,
		LastActive: time.Minute, LastActiveKnown: true,
	})
	r.Population.RawItems++

	normal := captureOutput(t, func() { PrintProtectedTable(r, ViewOptions{Limit: 20}) })
	if !strings.Contains(normal, "...") && !strings.Contains(normal, "…") {
		t.Errorf("expected middle truncation in normal mode:\n%s", normal)
	}
	term.SetTableOverflow(true)
	wide := captureOutput(t, func() { PrintProtectedTable(r, ViewOptions{Limit: 20, Wide: true}) })
	term.SetTableOverflow(false)
	if !strings.Contains(wide, longName) {
		t.Errorf("--wide should show the full name:\n%s", wide)
	}
}

func TestNoGroupMode(t *testing.T) {
	term.SetTermWidth(120)
	defer term.SetTermWidth(100)
	r := fixtureReport(t)
	r.Items = append(r.Items,
		model.SwapItem{Kind: model.KindCache, Name: "TestGoalsStatsFlagConflicts11102814295", Path: "/tmp/TestGoalsStatsFlagConflicts11102814295", Category: model.CatUntouchedCache, CacheKind: model.CacheRun, AllocatedBytes: 10 << 20, SizeBytes: 10 << 20, Cleanable: true, LastActive: time.Hour, LastActiveKnown: true},
		model.SwapItem{Kind: model.KindCache, Name: "TestGoalsStatsFlagConflicts22222222222", Path: "/tmp/TestGoalsStatsFlagConflicts22222222222", Category: model.CatUntouchedCache, CacheKind: model.CacheRun, AllocatedBytes: 10 << 20, SizeBytes: 10 << 20, Cleanable: true, LastActive: time.Hour, LastActiveKnown: true},
	)
	r.Population.Cleanable += 2
	r.Population.RawItems += 2
	grouped := captureOutput(t, func() { PrintActionableSection(r, ViewOptions{Limit: 20}) })
	if !strings.Contains(grouped, "TestGoalsStatsFlagConflicts* ×2") {
		t.Errorf("expected grouped row:\n%s", grouped)
	}
	expanded := captureOutput(t, func() { PrintActionableSection(r, ViewOptions{Limit: 20, NoGroup: true}) })
	if !strings.Contains(expanded, "11102814295") || !strings.Contains(expanded, "22222222222") || strings.Contains(expanded, "×2") {
		t.Errorf("--no-group should expand members:\n%s", expanded)
	}
}

func TestOrphanZeroRow(t *testing.T) {
	r := fixtureReport(t)
	out := captureOutput(t, func() { PrintCategorySummary(r) })
	for _, want := range []string{"abandoned-agents", "orphaned-caches"} {
		if !strings.Contains(out, want) {
			t.Errorf("category summary missing zero row %q:\n%s", want, out)
		}
	}
}

func TestExplainAgents(t *testing.T) {
	r := fixtureReport(t)
	out := captureOutput(t, func() { PrintAgentEvidence(r, ViewOptions{Limit: 12}) })
	for _, want := range []string{"PID 101", "parent", "tty", "cgroup", "reason", "decision", "idle"} {
		if !strings.Contains(out, want) {
			t.Errorf("agent evidence missing %q:\n%s", want, out)
		}
	}
}

func TestTableWidthAndNoTrailingWhitespace(t *testing.T) {
	defer term.SetTermWidth(100)
	for _, width := range []int{80, 100, 120} {
		term.SetTermWidth(width)
		term.SetAsciiMode(true)
		term.SetColorEnabled(false)
		r := fixtureReport(t)
		out := captureOutput(t, func() {
			PrintHeader(r)
			PrintActionableSection(r, ViewOptions{Limit: 12})
			PrintProtectedTable(r, ViewOptions{Limit: 12})
			PrintCategorySummary(r)
		})
		for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if got := term.StrWidth(line); got > width {
				t.Errorf("width %d: line %d is %d cells wide: %q", width, i, got, line)
			}
			if strings.HasSuffix(line, " ") {
				t.Errorf("width %d: line %d has trailing whitespace: %q", width, i, line)
			}
			if strings.Contains(line, " | ") || strings.Contains(line, "-+-") {
				if got := term.StrWidth(line); got != width {
					t.Errorf("width %d: table line %d is %d cells wide: %q", width, i, got, line)
				}
			}
		}
	}
}

func TestZeroSwapOrphanVisible(t *testing.T) {
	now := time.Unix(1700000000, 0)
	procs := []proc.ProcessSnapshot{
		{
			PID: 500, Comm: "agy", VmSwap: 0, Orphaned: true,
			Orphan:      proc.OrphanEvidence{Reason: "reparented-terminal-scope"},
			IdleKnown:   true,
			Idle:        time.Minute,
			ParentAlive: false,
		},
		{PID: 501, Comm: "bash", VmSwap: 0, IdleKnown: true, Idle: time.Minute},
	}
	items := BuildProcessItems(now, procs, 2*time.Hour)
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1 (agent retained, zero-swap non-agent dropped)", len(items))
	}
	item := items[0]
	if item.Category != model.CatAbandonedAgent || !item.Cleanable {
		t.Errorf("zero-swap orphan = (%v, cleanable=%v), want abandoned-agents, true", item.Category, item.Cleanable)
	}
	if item.SwapBytes != 0 || item.Signal != "SIGTERM" {
		t.Errorf("item swap = %d, signal = %q; want 0, SIGTERM", item.SwapBytes, item.Signal)
	}

	r, err := report.Build(now, model.SystemSwap{}, items, report.Thresholds{IdleThreshold: 2 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	r.Population.Agents = 1
	r.Population.AgentsZeroSwap = 1
	r.Population.AgentsOrphaned = 1
	if r.CleanableCount() != 1 {
		t.Errorf("cleanable count = %d, want 1", r.CleanableCount())
	}
	out := captureOutput(t, func() {
		PrintActionableSection(r, ViewOptions{Limit: 12})
		PrintCategorySummary(r)
	})
	if !strings.Contains(out, "agy") {
		t.Errorf("zero-swap orphan missing from actionable output:\n%s", out)
	}
	if !strings.Contains(out, "abandoned-agents") {
		t.Errorf("zero-row/orphan category missing:\n%s", out)
	}
}

func TestNoDashID(t *testing.T) {
	r := fixtureReport(t)
	out := captureOutput(t, func() {
		PrintActionableSection(r, ViewOptions{Limit: 20})
		PrintProtectedTable(r, ViewOptions{Limit: 20})
	})
	if strings.Contains(out, "| - ") || strings.Contains(out, "|  -") {
		t.Errorf("ID column should never render a bare dash:\n%s", out)
	}
}

func TestReconciliationFailureVisible(t *testing.T) {
	r := fixtureReport(t)
	r.Swap = report.Attribution{TotalBytes: 10, AttributedBytes: 20, OverlapBytes: 0}
	out := captureOutput(t, func() { PrintHeader(r) })
	if !strings.Contains(out, "FAIL") || !strings.Contains(out, "WARN") {
		t.Errorf("invalid report should show a WARN/FAIL reconciliation indicator:\n%s", out)
	}
}

func TestAllModeWithExplicitLimit(t *testing.T) {
	r := fixtureReport(t)
	r.Items = nil
	r.Population = report.Population{}
	for i := 0; i < 5; i++ {
		r.Items = append(r.Items, model.SwapItem{
			Kind: model.KindCache, Name: "cache-" + string(rune('a'+i)), Path: "/tmp/cache-" + string(rune('a'+i)),
			Category: model.CatUntouchedCache, CacheKind: model.CacheBuild,
			AllocatedBytes: int64(i+1) << 20, SizeBytes: int64(i+1) << 20,
			Cleanable: true, LastActive: time.Hour, LastActiveKnown: true,
		})
	}
	r.Population.RawItems = 5
	r.Population.Cleanable = 5
	out := captureOutput(t, func() {
		PrintActionableSection(r, ViewOptions{ShowAll: true, Limit: 3})
	})
	if !strings.Contains(out, "3 shown") || !strings.Contains(out, "2 hidden") {
		t.Errorf("--all with explicit --limit 3 should show 3 and hide 2:\n%s", out)
	}
	if !strings.Contains(out, "more groups (2 items") {
		t.Errorf("expected a footer for the two hidden items:\n%s", out)
	}
}

func TestPopulationExactCounts(t *testing.T) {
	r := fixtureReport(t)
	r.Items = nil
	r.Population = report.Population{}
	for i := 0; i < 3; i++ {
		r.Items = append(r.Items, model.SwapItem{
			Kind: model.KindProcess, PID: 1000 + i, Name: "bash", Category: model.CatActiveProcess,
			SwapBytes: 5 << 20, LastActive: time.Minute, LastActiveKnown: true,
		})
	}
	for i := 0; i < 12; i++ {
		r.Items = append(r.Items, model.SwapItem{
			Kind: model.KindCache, Name: "big-cache-" + string(rune('a'+i)), Path: "/tmp/big-cache-" + string(rune('a'+i)),
			Category: model.CatRecentCache, AllocatedBytes: 11 << 20, SizeBytes: 11 << 20,
			LastActive: time.Hour, LastActiveKnown: true,
		})
	}
	for i := 0; i < 5; i++ {
		r.Items = append(r.Items, model.SwapItem{
			Kind: model.KindCache, Name: "small-cache-" + string(rune('a'+i)), Path: "/tmp/small-cache-" + string(rune('a'+i)),
			Category: model.CatRecentCache, AllocatedBytes: 1 << 20, SizeBytes: 1 << 20,
			LastActive: time.Hour, LastActiveKnown: true,
		})
	}
	r.Population.RawItems = 20
	r.Population.Protected = 20

	out := captureOutput(t, func() {
		PrintProtectedTable(r, ViewOptions{Limit: 4})
	})
	flat := strings.Join(strings.Fields(out), " ")
	if !strings.Contains(flat, "Population: 20 raw · 18 after shell aggregation · 4 shown · 5 hidden below 10 MB · 9 beyond --limit") {
		t.Errorf("population arithmetic does not match the fixture:\n%s", out)
	}
}

func TestPermissionWarning(t *testing.T) {
	r := fixtureReport(t)
	r.Population.PermissionDenied = 3
	out := captureOutput(t, func() { PrintCategorySummary(r) })
	if !strings.Contains(out, "WARN: 3 processes could not be inspected") {
		t.Errorf("permission warning missing:\n%s", out)
	}
}
