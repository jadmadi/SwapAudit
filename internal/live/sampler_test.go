//go:build linux

package live

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/snapshot"
)

func testItems() []model.SwapItem {
	return []model.SwapItem{
		{PID: 1234, StartTicks: 42, Name: "gopls", Command: "gopls serve", Category: model.CatDormantAgent,
			SwapBytes: 200 * testMiB, Age: time.Hour, LastActive: 3 * time.Hour, LastActiveAt: time.Unix(0, 0), Cleanable: true},
		{Name: "go-build1", Path: "/tmp/go-build1", Category: model.CatUntouchedCache,
			SizeBytes: 100 * testMiB, Age: 15 * time.Hour, LastActive: 14 * time.Hour, LastActiveAt: time.Unix(0, 0), Cleanable: true},
		{PID: 999, StartTicks: 9, Name: "active", Category: model.CatActiveProcess,
			SwapBytes: 10 * testMiB, Age: time.Hour, LastActive: time.Minute},
	}
}

func testSystem() model.SystemSwap {
	return model.SystemSwap{
		SwapTotal: 8 * testGiB, SwapUsed: 2 * testGiB, SwapFree: 6 * testGiB,
		MemTotal: 32 * testGiB, MemUsed: 16 * testGiB, MemAvail: 16 * testGiB,
		TmpfsSize: 4 * testGiB, TmpfsUsed: testGiB,
	}
}

func samplerConfig(out *bytes.Buffer, items []model.SwapItem, sys model.SystemSwap) Config {
	snap := snapshot.BuildSnapshot(time.Unix(0, 0), 1, sys, items, 12*time.Hour, 2*time.Hour, false)
	return Config{
		Interval: 10 * time.Millisecond, Window: time.Minute, TmpInterval: 20 * time.Millisecond,
		ScanFast: NewFixedScanner(items, sys).Scan,
		ScanTmp:  func() []model.SwapItem { return items },
		Snapshot: func(time.Time) snapshot.Snapshot { return snap },
		Out:      out,
	}
}

func TestRunOnce(t *testing.T) {
	var out bytes.Buffer
	cfg := samplerConfig(&out, testItems(), testSystem())
	cfg.Once = true
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	var doc onceDocument
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if doc.Type != "once" || doc.Window.Samples != 1 {
		t.Errorf("type/samples = %q/%d", doc.Type, doc.Window.Samples)
	}
	if doc.Window.TmpfsCleanableBytes != 100*testMiB || doc.Window.TmpfsCleanableItems != 1 {
		t.Errorf("cleanable = %d/%d", doc.Window.TmpfsCleanableBytes, doc.Window.TmpfsCleanableItems)
	}
	if doc.Snapshot.SchemaVersion != 1 {
		t.Errorf("snapshot schema = %d", doc.Snapshot.SchemaVersion)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Error("output contains ANSI escapes")
	}
}

func TestRunOnceRequiresSnapshot(t *testing.T) {
	var out bytes.Buffer
	cfg := samplerConfig(&out, testItems(), testSystem())
	cfg.Once = true
	cfg.Snapshot = nil
	if err := Run(context.Background(), cfg); err == nil {
		t.Fatal("Run must reject --once without a snapshot builder")
	}
}

func TestRunStream(t *testing.T) {
	var out bytes.Buffer
	cfg := samplerConfig(&out, testItems(), testSystem())
	cfg.Duration = 60 * time.Millisecond
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 4 {
		t.Fatalf("stream emitted %d lines, want >= 4\n%s", len(lines), out.String())
	}
	for i, line := range lines {
		var doc map[string]any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("line %d is not JSON: %v", i, err)
		}
		if doc["type"] == "stopped" {
			if i != len(lines)-1 || doc["reason"] != "duration" {
				t.Errorf("unexpected stopped line: %s", line)
			}
		} else if doc["type"] != "sample" {
			t.Errorf("line %d type = %v", i, doc["type"])
		}
	}
	if !strings.Contains(lines[len(lines)-1], `"type":"stopped"`) {
		t.Errorf("last line is not stopped: %s", lines[len(lines)-1])
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Error("stream contains ANSI escapes")
	}
}

func TestRunCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	cfg := samplerConfig(&out, testItems(), testSystem())
	cfg.OnSample = func(WindowStats) { cancel() }
	if err := Run(ctx, cfg); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out.String(), `"type":"stopped"`) || !strings.Contains(out.String(), `"reason":"signal"`) {
		t.Errorf("missing signal stopped line:\n%s", out.String())
	}
}

func TestRunOnceRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.ndjson")
	var out bytes.Buffer
	cfg := samplerConfig(&out, testItems(), testSystem())
	cfg.Once = true
	cfg.RecordPath = path
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	samples := readRecorded(t, path)
	if len(samples) != 1 {
		t.Fatalf("recorded %d samples, want 1", len(samples))
	}
}

func TestSlowLane(t *testing.T) {
	var scans int32
	var out bytes.Buffer
	cfg := samplerConfig(&out, testItems(), testSystem())
	cfg.TmpInterval = 10 * time.Millisecond
	cfg.Duration = 80 * time.Millisecond
	cfg.ScanTmp = func() []model.SwapItem {
		atomic.AddInt32(&scans, 1)
		return testItems()
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got := atomic.LoadInt32(&scans); got < 2 {
		t.Errorf("slow lane scans = %d, want >= 2", got)
	}
}

func TestClockJumpDropped(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	base := time.Now()
	mono := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls > 4 {
			return base.Add(-time.Minute)
		}
		return base
	}
	var out bytes.Buffer
	cfg := samplerConfig(&out, testItems(), testSystem())
	cfg.Interval = 5 * time.Millisecond
	cfg.Duration = 50 * time.Millisecond
	cfg.Mono = mono
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if !strings.Contains(out.String(), `"type":"stopped"`) {
		t.Error("missing stopped line")
	}
}

func TestNoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()
	var out bytes.Buffer
	cfg := samplerConfig(&out, testItems(), testSystem())
	cfg.Duration = 50 * time.Millisecond
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && runtime.NumGoroutine() > before+2 {
		time.Sleep(10 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before+2 {
		t.Errorf("goroutines grew from %d to %d", before, after)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("broken pipe")
}

func TestBrokenPipe(t *testing.T) {
	cfg := samplerConfig(&bytes.Buffer{}, testItems(), testSystem())
	cfg.Out = failingWriter{}
	cfg.Duration = 20 * time.Millisecond
	if err := Run(context.Background(), cfg); err == nil {
		t.Fatal("Run must surface a write error")
	}
}

func TestCapacity(t *testing.T) {
	cases := []struct {
		window, interval time.Duration
		want             int
	}{
		{time.Hour, time.Second, 600},
		{time.Minute, 100 * time.Millisecond, 600},
		{time.Second, time.Minute, 1},
		{time.Minute, time.Second, 60},
	}
	for _, tc := range cases {
		if got := capacity(tc.window, tc.interval); got != tc.want {
			t.Errorf("capacity(%v,%v) = %d, want %d", tc.window, tc.interval, got, tc.want)
		}
	}
}

func readRecorded(t *testing.T, path string) []sample {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	samples, err := Replay(f)
	if err != nil {
		t.Fatal(err)
	}
	return samples
}
