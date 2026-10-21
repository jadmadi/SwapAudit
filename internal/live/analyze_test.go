//go:build linux

package live

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

func modelSystem(swapTotal int64) model.SystemSwap {
	return model.SystemSwap{SwapTotal: swapTotal, TmpfsSize: swapTotal}
}

func fmtETA(eta *int64) any {
	if eta == nil {
		return "null"
	}
	return *eta
}

func loadFixture(t *testing.T, name string) []sample {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".ndjson"))
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer f.Close()
	samples, err := Replay(f)
	if err != nil {
		t.Fatalf("replay fixture %s: %v", name, err)
	}
	if len(samples) == 0 {
		t.Fatalf("fixture %s is empty", name)
	}
	return samples
}

func analyzeFixture(t *testing.T, name string) WindowStats {
	t.Helper()
	samples := loadFixture(t, name)
	last := samples[len(samples)-1]
	return Analyze(samples, 60*time.Second, last.TmpfsSampledAt, last.At.Add(time.Second))
}

func TestAnalyzeFixtures(t *testing.T) {
	cases := []struct {
		name  string
		trend Trend
		basis string
		eta   bool
	}{
		{"idle", TrendFlat, "flat", false},
		{"ramp", TrendFilling, "ols", true},
		{"spike", TrendInsufficient, "noisy_fit", false},
		{"recovering", TrendRecovering, "recovering", false},
		{"noise", TrendInsufficient, "non_monotone", false},
		{"short", TrendInsufficient, "insufficient_samples", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stats := analyzeFixture(t, tc.name)
			if stats.Swap.Trend != tc.trend {
				t.Errorf("trend = %q, want %q (basis %q)", stats.Swap.Trend, tc.trend, stats.Swap.ETABasis)
			}
			if stats.Swap.ETABasis != tc.basis {
				t.Errorf("basis = %q, want %q", stats.Swap.ETABasis, tc.basis)
			}
			if (stats.Swap.ETASeconds != nil) != tc.eta {
				t.Errorf("eta presence = %v, want %v", stats.Swap.ETASeconds != nil, tc.eta)
			}
			if stats.Swap.Samples != len(loadFixture(t, tc.name)) {
				t.Errorf("samples = %d", stats.Swap.Samples)
			}
			t.Logf("trend=%s basis=%s eta=%v r2=%.6f ratio=%.3f span=%.0fs",
				stats.Swap.Trend, stats.Swap.ETABasis, fmtETA(stats.Swap.ETASeconds), stats.Swap.FitR2, stats.Swap.NonNegativeRatio, stats.Swap.SpanSeconds)
		})
	}
}

func TestRampETAWithin10Percent(t *testing.T) {
	stats := analyzeFixture(t, "ramp")
	if stats.Swap.ETASeconds == nil {
		t.Fatal("ramp must produce an ETA")
	}
	last := stats.System
	free := float64(last.SwapTotal - last.SwapUsed)
	analytic := free / stats.Swap.RateBytesPerSec
	got := float64(*stats.Swap.ETASeconds)
	if math.Abs(got-analytic)/analytic > 0.10 {
		t.Errorf("ETA = %.0fs, analytic %.0fs, deviation > 10%%", got, analytic)
	}
}

func TestStaleTmpfs(t *testing.T) {
	stats := analyzeFixture(t, "stale-tmp")
	if stats.Tmpfs.ETASeconds != nil {
		t.Errorf("stale tmpfs ETA = %d, want null", *stats.Tmpfs.ETASeconds)
	}
	if stats.Tmpfs.ETABasis != "short_span" {
		t.Errorf("stale tmpfs basis = %q, want short_span", stats.Tmpfs.ETABasis)
	}
	if stats.TmpfsSampledAgeSec < 300 {
		t.Errorf("tmpfs sampled age = %d, want >= 300", stats.TmpfsSampledAgeSec)
	}
}

func TestPIDReuse(t *testing.T) {
	stats := analyzeFixture(t, "pid-reuse")
	if stats.ExitedBytes != 300*1024*1024 {
		t.Errorf("exited bytes = %d, want %d", stats.ExitedBytes, 300*1024*1024)
	}
	if stats.StartedBytes != 100*1024*1024 {
		t.Errorf("started bytes = %d, want %d", stats.StartedBytes, 100*1024*1024)
	}
	for _, m := range stats.Movers {
		if m.PID != nil && *m.PID == 42 {
			t.Errorf("PID reuse produced a phantom mover: %+v", m)
		}
	}
}

func TestExitNeverNegativeMover(t *testing.T) {
	stats := analyzeFixture(t, "exit")
	if stats.ExitedBytes != 200*1024*1024 {
		t.Errorf("exited bytes = %d, want %d", stats.ExitedBytes, 200*1024*1024)
	}
	for _, m := range stats.Movers {
		if m.DeltaBytes < 0 {
			t.Errorf("negative mover emitted: %+v", m)
		}
	}
}

func TestEmptySlices(t *testing.T) {
	stats := Analyze(nil, time.Minute, time.Time{}, time.Now())
	if stats.Movers == nil || stats.TmpfsMovers == nil {
		t.Fatal("movers must be empty slices, not nil")
	}
	if len(stats.Movers) != 0 || len(stats.TmpfsMovers) != 0 {
		t.Fatal("empty input produced movers")
	}
	if stats.Swap.ETABasis != "insufficient_samples" {
		t.Errorf("basis = %q", stats.Swap.ETABasis)
	}
}

func TestDeterministic(t *testing.T) {
	stats := analyzeFixture(t, "ramp")
	a, err := json.Marshal(stats)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(analyzeFixture(t, "ramp"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Error("Analyze is not deterministic")
	}
}

func TestMoverRankingAndLimit(t *testing.T) {
	oldest := emptySample(time.Unix(0, 0), modelSystem(8*1024*1024*1024))
	newest := emptySample(time.Unix(60, 0), modelSystem(8*1024*1024*1024))
	for i := 0; i < 15; i++ {
		key := processKey{PID: 100 + i, StartTicks: 1}
		oldest.Processes[key] = 10
		newest.Processes[key] = 10 + int64(i+1)*100
		newest.Names[key] = "p"
	}
	oldest.Caches["/tmp/a"] = 100
	newest.Caches["/tmp/a"] = 500
	oldest.Caches["/tmp/b"] = 100
	newest.Caches["/tmp/b"] = 300

	stats := Analyze([]sample{oldest, newest}, time.Minute, newest.At, newest.At)
	if len(stats.Movers) != 10 {
		t.Fatalf("movers = %d, want 10", len(stats.Movers))
	}
	if stats.Movers[0].DeltaBytes != 1500 {
		t.Errorf("first mover delta = %d, want 1500", stats.Movers[0].DeltaBytes)
	}
	if len(stats.TmpfsMovers) != 2 || stats.TmpfsMovers[0].Name != "/tmp/a" {
		t.Errorf("tmpfs movers = %+v", stats.TmpfsMovers)
	}
}

func TestSeriesGates(t *testing.T) {
	base := time.Unix(0, 0)
	free := int64(1024 * 1024 * 1024)
	mk := func(values ...float64) []point {
		pts := make([]point, len(values))
		for i, v := range values {
			pts[i] = point{t: float64(i), y: v}
		}
		return pts
	}
	cases := []struct {
		name  string
		pts   []point
		basis string
	}{
		{"few", mk(1, 2, 3), "insufficient_samples"},
		{"short span", mk(1, 2, 3, 4, 5), "short_span"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			samples := make([]sample, len(tc.pts))
			for i := range tc.pts {
				samples[i] = emptySample(base.Add(time.Duration(i)*time.Second), modelSystem(0))
				samples[i].MonoSeconds = tc.pts[i].t
				samples[i].System.SwapUsed = int64(tc.pts[i].y)
			}
			stats := seriesStats(samples, 60*time.Second, func(s sample) int64 { return s.System.SwapUsed }, free)
			if stats.ETABasis != tc.basis {
				t.Errorf("basis = %q, want %q", stats.ETABasis, tc.basis)
			}
			if stats.ETASeconds != nil {
				t.Error("ETA must be null")
			}
		})
	}
}
