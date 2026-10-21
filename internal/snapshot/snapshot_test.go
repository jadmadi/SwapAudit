//go:build linux

package snapshot

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

var update = flag.Bool("update", false, "rewrite golden files")

func fixedClock() time.Time {
	return time.Date(2026, 10, 21, 9, 15, 4, 0, time.FixedZone("EEST", 3*60*60))
}

func fixedSystem() model.SystemSwap {
	return model.SystemSwap{
		SwapTotal: 18321752064,
		SwapUsed:  8971677696,
		SwapFree:  9350074368,
		MemTotal:  33549176832,
		MemUsed:   15874695168,
		MemAvail:  17674481664,
		TmpfsSize: 16777216000,
		TmpfsUsed: 7838310400,
	}
}

func fixedItems(now time.Time) []model.SwapItem {
	return []model.SwapItem{
		{
			PID: 1618347, StartTicks: 123456789, Name: "chrome-devtools",
			Command: "chrome-devtools-mcp --port 9222", Category: model.CatDormantAgent,
			SwapBytes: 94057267, Age: 104880 * time.Second, LastActive: 43980 * time.Second,
			LastActiveAt: now.Add(-43980 * time.Second), Cleanable: true, Count: 1,
		},
		{
			PID: 4242, StartTicks: 99, Name: "gopls", Command: "gopls serve",
			Category: model.CatActiveProcess, SwapBytes: 5242880, Age: 3600 * time.Second,
			LastActive: -1, LastActiveAt: now.Add(time.Second), Cleanable: false, Count: 1,
		},
		{
			Name: "go-build123", Command: "/tmp/go-build123", Path: "/tmp/go-build123",
			Category: model.CatUntouchedCache, SizeBytes: 3145728, Age: 48 * time.Hour,
			LastActive: 13 * time.Hour, LastActiveAt: now.Add(-13 * time.Hour), Cleanable: true, Count: 1,
		},
		{
			Name: "active-cache", Command: "/tmp/active-cache", Path: "/tmp/active-cache",
			Category: model.CatActiveCache, SizeBytes: 1024, Age: time.Minute,
			LastActive: time.Second, LastActiveAt: now.Add(-time.Second), Cleanable: false, Count: 1,
		},
		{
			Name: "cfg&<x>", Command: "/tmp/cfg&<x>-é", Path: "/tmp/cfg&<x>-é",
			Category: model.CatAbandonedRun, SizeBytes: 2048, Age: 14 * time.Hour,
			LastActive: 14 * time.Hour, LastActiveAt: now.Add(-14 * time.Hour), Cleanable: true, Count: 1,
		},
	}
}

func buildFixed(noCommand bool) Snapshot {
	now := fixedClock()
	return BuildSnapshot(now, 123456.78, fixedSystem(), fixedItems(now), 12*time.Hour, 2*time.Hour, noCommand)
}

func TestGolden(t *testing.T) {
	path := filepath.Join("testdata", "snapshot_v1.json")
	var buf bytes.Buffer
	if err := Write(&buf, buildFixed(false)); err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) failed: %v (run go test -run TestGolden -update)", path, err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), string(want))
	}
}

func TestNullability(t *testing.T) {
	doc := buildFixed(false)
	byName := make(map[string]Item, len(doc.Items))
	for _, item := range doc.Items {
		byName[item.Name] = item
	}

	proc := byName["chrome-devtools"]
	if proc.PID == nil || proc.StartTicks == nil {
		t.Fatal("process identity must not be null")
	}
	if proc.Path != nil {
		t.Error("process path must be null")
	}
	if proc.IdleSeconds == nil || *proc.IdleSeconds != 43980 {
		t.Errorf("process idle = %v, want 43980", proc.IdleSeconds)
	}
	if proc.LastActiveAt == nil || *proc.LastActiveAt != "2026-10-20T21:02:04+03:00" {
		t.Errorf("last_active_at = %v", proc.LastActiveAt)
	}

	unknown := byName["gopls"]
	if unknown.IdleSeconds != nil {
		t.Errorf("unknown idle must be null, got %v", *unknown.IdleSeconds)
	}
	if unknown.LastActiveAt != nil {
		t.Errorf("last_active_at must be null when idle is null, got %v", *unknown.LastActiveAt)
	}
	if unknown.AgeSeconds == nil || *unknown.AgeSeconds != 3600 {
		t.Errorf("age = %v, want 3600", unknown.AgeSeconds)
	}

	cache := byName["go-build123"]
	if cache.PID != nil || cache.StartTicks != nil {
		t.Error("cache pid/start_ticks must be null")
	}
	if cache.Path == nil || *cache.Path != "/tmp/go-build123" {
		t.Errorf("cache path = %v", cache.Path)
	}
	if cache.IdleSeconds == nil || *cache.IdleSeconds != 46800 {
		t.Errorf("cache idle = %v, want 46800", cache.IdleSeconds)
	}
}

func TestEmptyScan(t *testing.T) {
	doc := BuildSnapshot(fixedClock(), 1, model.SystemSwap{}, nil, 12*time.Hour, 2*time.Hour, false)
	if doc.Summary == nil || doc.Items == nil || doc.Warnings == nil {
		t.Fatal("summary/items/warnings must be empty slices, not nil")
	}
	if len(doc.Summary) != 0 || len(doc.Items) != 0 || len(doc.Warnings) != 0 {
		t.Fatal("empty scan must produce empty arrays")
	}
	if doc.Totals != (Totals{}) {
		t.Errorf("totals = %+v, want zero", doc.Totals)
	}
}

func TestDeterminism(t *testing.T) {
	var a, b bytes.Buffer
	if err := Write(&a, buildFixed(false)); err != nil {
		t.Fatal(err)
	}
	if err := Write(&b, buildFixed(false)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("two builds with identical inputs must be byte-identical")
	}
}

func TestCanonicalOrdering(t *testing.T) {
	now := fixedClock()
	items := fixedItems(now)
	for i := 0; i < len(items)/2; i++ {
		j := len(items) - 1 - i
		items[i], items[j] = items[j], items[i]
	}

	doc := BuildSnapshot(now, 1, fixedSystem(), items, 12*time.Hour, 2*time.Hour, false)
	for i := 1; i < len(doc.Items); i++ {
		prev, cur := doc.Items[i-1], doc.Items[i]
		if prev.Category > cur.Category {
			t.Fatalf("category order violated at %d: %q > %q", i, prev.Category, cur.Category)
		}
		if prev.Category == cur.Category {
			if prev.EffectiveSizeBytes < cur.EffectiveSizeBytes {
				t.Fatalf("size order violated at %d", i)
			}
			if prev.EffectiveSizeBytes == cur.EffectiveSizeBytes && prev.Name > cur.Name {
				t.Fatalf("name order violated at %d", i)
			}
		}
	}
	for i := 1; i < len(doc.Summary); i++ {
		if doc.Summary[i-1].Category >= doc.Summary[i].Category {
			t.Fatalf("summary not sorted ascending: %q >= %q", doc.Summary[i-1].Category, doc.Summary[i].Category)
		}
	}
}

func TestNoCommand(t *testing.T) {
	withCommands := buildFixed(false)
	withoutCommands := buildFixed(true)
	for _, item := range withoutCommands.Items {
		if item.Command != "" {
			t.Errorf("command for %q = %q, want empty", item.Name, item.Command)
		}
	}
	for i := range withCommands.Items {
		withCommands.Items[i].Command = ""
	}
	if !reflect.DeepEqual(withCommands, withoutCommands) {
		t.Error("--no-command must change only the command fields")
	}
}

func TestHTMLNotEscaped(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, buildFixed(false)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "/tmp/cfg&<x>-é") {
		t.Errorf("path was escaped or mangled: %s", out)
	}
	if strings.Contains(out, `\u003c`) || strings.Contains(out, `\u0026`) {
		t.Error("HTML escaping must be disabled")
	}
}

func TestErrorDocument(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteError(&buf, "proc_unavailable", "cannot read /proc/self/status"); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"schema_version\": 1,\n  \"error\": {\n    \"code\": \"proc_unavailable\",\n    \"message\": \"cannot read /proc/self/status\"\n  }\n}\n"
	if buf.String() != want {
		t.Errorf("error document = %q, want %q", buf.String(), want)
	}
}

func TestRoundTripTypes(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, buildFixed(false)); err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if raw["schema_version"].(float64) != 1 {
		t.Error("schema_version must be 1")
	}
	if _, ok := raw["summary"].([]any); !ok {
		t.Error("summary must be an array")
	}
	if _, ok := raw["items"].([]any); !ok {
		t.Error("items must be an array")
	}
	if _, ok := raw["warnings"].([]any); !ok {
		t.Error("warnings must be an array")
	}
	items := raw["items"].([]any)
	for _, entry := range items {
		item := entry.(map[string]any)
		if _, ok := item["effective_size_bytes"]; !ok {
			t.Error("effective_size_bytes missing")
		}
		if _, ok := item["cleanable"].(bool); !ok {
			t.Error("cleanable must be a boolean")
		}
		for _, key := range []string{"pid", "age_seconds", "idle_seconds", "last_active_at", "path"} {
			if _, ok := item[key]; !ok {
				t.Errorf("nullable key %q must be present even when null", key)
			}
		}
	}
}

func TestProductionSizeCeiling(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("caller unavailable")
	}
	src := filepath.Join(filepath.Dir(file), "snapshot.go")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Count(string(data), "\n")
	if lines > 300 {
		t.Errorf("snapshot.go has %d lines, ceiling is 300", lines)
	}
}

func TestMarshalLargeScan(t *testing.T) {
	now := fixedClock()
	items := make([]model.SwapItem, 0, 500)
	for i := 0; i < 500; i++ {
		items = append(items, model.SwapItem{
			PID: 1000 + i, StartTicks: int64(i + 1), Name: "worker", Command: "worker --serve",
			Category: model.CatActiveProcess, SwapBytes: int64(i) * 4096,
			Age: time.Hour, LastActive: time.Minute, LastActiveAt: now.Add(-time.Minute), Count: 1,
		})
	}
	doc := BuildSnapshot(now, 1, fixedSystem(), items, 12*time.Hour, 2*time.Hour, false)
	if err := Write(io.Discard, doc); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkMarshal500Items(b *testing.B) {
	now := fixedClock()
	items := make([]model.SwapItem, 0, 500)
	for i := 0; i < 500; i++ {
		items = append(items, model.SwapItem{
			PID: 1000 + i, StartTicks: int64(i + 1), Name: "worker", Command: "worker --serve",
			Category: model.CatActiveProcess, SwapBytes: int64(i) * 4096,
			Age: time.Hour, LastActive: time.Minute, LastActiveAt: now.Add(-time.Minute), Count: 1,
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc := BuildSnapshot(now, 1, fixedSystem(), items, 12*time.Hour, 2*time.Hour, false)
		if err := Write(io.Discard, doc); err != nil {
			b.Fatal(err)
		}
	}
}
