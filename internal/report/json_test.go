package report

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/cleaner"
	"github.com/jadmadi/swapaudit/internal/model"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func jsonFixture(t *testing.T) AuditReport {
	t.Helper()
	now := time.Date(2026, 10, 19, 22, 30, 0, 0, time.UTC)
	items := []model.SwapItem{
		{
			ID: "aaaa1111", Kind: model.KindProcess, PID: 101, StartTicks: 5000, UID: 1000,
			Name: "agy", Category: model.CatDormantAgent, SwapBytes: 500 << 20,
			Age: 48 * time.Hour, LastActive: 3 * time.Hour, LastActiveKnown: true,
			Cleanable: true, Agent: true, Signal: "SIGTERM", ParentPID: 1, ParentComm: "systemd",
			OrphanReason: "idle-threshold", Evidence: "idle=3h", TTYNr: 123,
			Cgroup: "/user.slice/session-1.scope",
		},
		{
			ID: "bbbb2222", Kind: model.KindProcess, PID: 102, StartTicks: 6000, UID: 1000,
			Name: "zero-agent", Category: model.CatActiveProcess, SwapBytes: 0,
			Age: time.Hour, LastActive: 10 * time.Minute, LastActiveKnown: true,
			Agent: true, ParentPID: 1, ParentComm: "systemd", Evidence: "idle=10m",
		},
		{
			ID: "cccc3333", Kind: model.KindCache, Name: "go-build1234", Path: "/tmp/go-build1234",
			Category: model.CatUntouchedCache, CacheKind: model.CacheBuild,
			AllocatedBytes: 300 << 20, SizeBytes: 300 << 20, ApparentBytes: 320 << 20,
			Age: 48 * time.Hour, LastActive: 13 * time.Hour, LastActiveKnown: true,
			Cleanable: true, Evidence: "idle=13h handles=0 kind=build",
		},
		{
			ID: "dddd4444", Kind: model.KindCache, Name: "orphan-cache", Path: "/tmp/orphan-cache",
			Category: model.CatOrphanedCache, CacheKind: model.CacheBuild,
			AllocatedBytes: 100 << 20, SizeBytes: 100 << 20, ApparentBytes: 100 << 20,
			Age: 48 * time.Hour, LastActive: 2 * time.Hour, LastActiveKnown: true,
			Evidence: "idle=2h handles=0 kind=build; use --clean-orphaned-caches",
		},
		{
			ID: "eeee5555", Kind: model.KindCache, Name: "in-use-cache", Path: "/tmp/in-use-cache",
			Category: model.CatInUseCache, CacheKind: model.CacheGeneric,
			AllocatedBytes: 50 << 20, SizeBytes: 50 << 20, Age: time.Hour,
			LastActive: time.Minute, LastActiveKnown: true, OpenHandles: 2,
			Evidence: "idle=1m handles=2",
		},
	}
	system := model.SystemSwap{
		SwapTotal: 10 << 30, SwapUsed: 5 << 30, SwapFree: 5 << 30,
		MemTotal: 16 << 30, MemUsed: 8 << 30, MemAvail: 8 << 30,
		TmpfsSize: 20 << 30, TmpfsUsed: 10 << 30, TmpfsFree: 10 << 30, TmpfsAvail: 10 << 30,
	}
	r, err := BuildWithScan(now, system, items, Thresholds{
		MaxAge: 12 * time.Hour, IdleThreshold: 2 * time.Hour, OrphanCacheAge: time.Hour,
	}, Scan{
		TmpfsReasons: []ReasonBytes{{Reason: "other-uid", Bytes: 1 << 20, Count: 1}},
		SwapReasons:  []ReasonBytes{{Reason: "zero-swap", Count: 3}},
		InUseCaches:  1, OrphanedCaches: 1, RecentCaches: 0,
	})
	if err != nil {
		t.Fatalf("BuildWithScan: %v", err)
	}
	r.Host = Host{KernelRelease: "6.8.0-test", UptimeSeconds: 123456}
	r.Population.Processes = 200
	r.Population.ProcessesWithSwap = 20
	r.Population.Agents = 2
	r.Population.AgentsWithSwap = 1
	r.Population.AgentsZeroSwap = 1
	r.Population.AgentsDormant = 1
	r.Population.AgentsOrphaned = 0
	r.Population.AgentsParentAlive = 2
	r.Population.AgentsIdleUnknown = 0
	r.Population.AgentsActive = 1
	return r
}

func TestJSONGolden(t *testing.T) {
	r := jsonFixture(t)
	jr := ToJSON(r)
	got, err := json.MarshalIndent(jr, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "audit-v1.golden.json")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("JSON golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestJSONDeterministic(t *testing.T) {
	r := jsonFixture(t)
	var a, b bytes.Buffer
	if err := WriteJSON(&a, r, nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(&b, r, nil); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Errorf("WriteJSON is not deterministic")
	}
	if strings.Count(a.String(), "\n") != 1 {
		t.Errorf("WriteJSON should emit exactly one newline, got %d", strings.Count(a.String(), "\n"))
	}
}

func TestJSONFullNames(t *testing.T) {
	r := jsonFixture(t)
	longName := "TestGoalsStatsFlagConflicts11102814295-with-a-very-long-distinguishing-suffix"
	r.Items = append(r.Items, model.SwapItem{
		Kind: model.KindCache, Name: longName, Path: "/tmp/" + longName,
		Category: model.CatRecentCache, CacheKind: model.CacheRun,
		AllocatedBytes: 1024, SizeBytes: 1024, LastActiveKnown: true,
	})
	jr := ToJSON(r)
	found := false
	for _, it := range jr.Items {
		if it.Name == longName && it.Path == "/tmp/"+longName {
			found = true
		}
		if strings.Contains(it.Name, "…") || strings.Contains(it.Name, "...") {
			t.Errorf("JSON item name was truncated: %q", it.Name)
		}
	}
	if !found {
		t.Errorf("full name and path not preserved in JSON")
	}
}

func TestJSONNoANSI(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, jsonFixture(t), nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "\x1b") {
		t.Errorf("JSON output contains ANSI escapes")
	}
}

func TestJSONSchemaFields(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, jsonFixture(t), nil); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["schema_version"].(float64) != 1 {
		t.Errorf("schema_version = %v, want 1", doc["schema_version"])
	}
	for _, key := range []string{"generated_at", "host", "thresholds", "system", "attribution", "reclaim", "population", "scan", "categories", "groups", "items", "cleanup"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("missing top-level key %q", key)
		}
	}
	swap := doc["attribution"].(map[string]any)["swap"].(map[string]any)
	for _, key := range []string{"total_bytes", "attributed_bytes", "unattributed_bytes", "overlap_bytes", "reasons"} {
		if _, ok := swap[key]; !ok {
			t.Errorf("missing swap attribution key %q", key)
		}
	}
	if _, ok := swap["unattributed_bytes"].(float64); !ok {
		t.Errorf("unattributed_bytes must be a JSON number")
	}
	items := doc["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected items")
	}
	first := items[0].(map[string]any)
	for _, key := range []string{"kind", "id", "pid", "path", "category", "cleanable", "swap_bytes", "allocated_bytes", "idle_seconds", "idle_known", "evidence"} {
		if _, ok := first[key]; !ok {
			t.Errorf("missing item key %q", key)
		}
	}
}

func TestJSONReconciliation(t *testing.T) {
	r := jsonFixture(t)
	jr := ToJSON(r)
	if jr.Attribution.Swap.AttributedBytes+jr.Attribution.Swap.UnattributedBytes != jr.System.Swap.UsedBytes {
		t.Errorf("swap attribution does not tie to system swap")
	}
	if jr.Attribution.Tmpfs.AttributedBytes+jr.Attribution.Tmpfs.UnattributedBytes != jr.System.Tmpfs.UsedBytes {
		t.Errorf("tmpfs attribution does not tie to system tmpfs")
	}
}

func TestJSONCleanup(t *testing.T) {
	cr := &cleaner.CleanupResult{
		DryRun: true,
		Groups: []cleaner.CleanupGroupResult{{
			Group: "Untouched Build Caches", Targets: 2, Bytes: 100,
			WouldPrune: 2, Skipped: 1,
			Reasons: []cleaner.ReasonCount{{Reason: "open-handle", Count: 1}},
		}},
		Summary: cleaner.CleanupStats{Pruned: 2, Skipped: 1, Reasons: map[string]int{"open-handle": 1}},
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, jsonFixture(t), cr); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	cleanup := doc["cleanup"].(map[string]any)
	if cleanup["dry_run"] != true {
		t.Errorf("cleanup.dry_run = %v, want true", cleanup["dry_run"])
	}
	groups := cleanup["groups"].([]any)
	if len(groups) != 1 {
		t.Fatalf("cleanup groups = %d, want 1", len(groups))
	}
	summary := cleanup["summary"].(map[string]any)
	if summary["pruned"].(float64) != 2 || summary["skipped"].(float64) != 1 {
		t.Errorf("cleanup summary = %v", summary)
	}
}
