//go:build linux

package watch

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/live"
	"github.com/jadmadi/swapaudit/internal/model"
)

func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }

func baseStats(now time.Time) live.WindowStats {
	return live.WindowStats{
		Seconds: 60, Samples: 60, Complete: true, SampledAt: now,
		System:      model.SystemSwap{SwapTotal: 1000, SwapUsed: 500, SwapFree: 500, TmpfsSize: 1000, TmpfsUsed: 500},
		Swap:        live.SeriesStats{Trend: live.TrendFlat, ETABasis: "flat", NonNegativeRatio: 1, Samples: 60, SpanSeconds: 59, FitR2: 1},
		Tmpfs:       live.SeriesStats{Trend: live.TrendFlat, ETABasis: "flat", NonNegativeRatio: 1, Samples: 60, SpanSeconds: 59, FitR2: 1},
		Movers:      []live.Mover{},
		TmpfsMovers: []live.Mover{},
	}
}

func dangerSwap(now time.Time, etaSeconds int64) live.WindowStats {
	s := baseStats(now)
	s.System.SwapUsed = 800
	s.Swap = live.SeriesStats{Trend: live.TrendFilling, RateBytesPerSec: 2 << 20, ETASeconds: int64Ptr(etaSeconds), ETABasis: "ols", Samples: 60, SpanSeconds: 59, FitR2: 0.99, NonNegativeRatio: 1}
	s.Movers = []live.Mover{{Kind: "process", PID: intPtr(1420081), StartTicks: int64Ptr(123456), Name: "gopls", DeltaBytes: 96 << 20, CurrentBytes: 1900 << 20}}
	return s
}

func clearSwap(now time.Time) live.WindowStats {
	s := baseStats(now)
	s.System.SwapUsed = 400
	return s
}

func testConfig(now *time.Time) Config {
	return Config{
		ETAThreshold: 15 * time.Minute, PressureThreshold: 90, Sustain: 10 * time.Second,
		Cooldown: 10 * time.Minute, MaxPerHour: 4, RearmETA: 20 * time.Minute, RearmPressure: 85,
		Resource: "swap", Interval: time.Second, TmpInterval: 30 * time.Second,
		Now: func() time.Time { return *now },
	}
}

func TestIdleProducesNoAlerts(t *testing.T) {
	now := time.Unix(0, 0)
	cfg := testConfig(&now)
	states := map[string]*State{}
	for i := 0; i < 200; i++ {
		now = now.Add(3 * time.Second)
		if ds := Evaluate(cfg, states, clearSwap(now)); len(ds) != 0 {
			t.Fatalf("idle produced a decision: %+v", ds)
		}
	}
}

func TestRampProducesOneAlert(t *testing.T) {
	now := time.Unix(0, 0)
	cfg := testConfig(&now)
	states := map[string]*State{}
	if ds := Evaluate(cfg, states, dangerSwap(now, 312)); len(ds) != 0 {
		t.Fatalf("first danger tick must not alert: %+v", ds)
	}
	now = now.Add(10 * time.Second)
	ds := Evaluate(cfg, states, dangerSwap(now, 312))
	if len(ds) != 1 || ds[0].Kind != "alert" || ds[0].Severity != "warning" {
		t.Fatalf("decisions = %+v", ds)
	}
	if ds[0].Title != "swapaudit: swap full in 5m 12s" {
		t.Errorf("title = %q", ds[0].Title)
	}
	if !strings.Contains(ds[0].Body, "gopls (PID 1420081)") || !strings.Contains(ds[0].Body, "sustained 10s") {
		t.Errorf("body = %q", ds[0].Body)
	}
	now = now.Add(5 * time.Second)
	if ds := Evaluate(cfg, states, dangerSwap(now, 312)); len(ds) != 0 {
		t.Errorf("alerted state must suppress repeats: %+v", ds)
	}
}

func TestSustainBlipDoesNotAlert(t *testing.T) {
	now := time.Unix(0, 0)
	cfg := testConfig(&now)
	states := map[string]*State{}
	Evaluate(cfg, states, dangerSwap(now, 312))
	now = now.Add(5 * time.Second)
	if ds := Evaluate(cfg, states, clearSwap(now)); len(ds) != 0 {
		t.Fatalf("blip produced a decision: %+v", ds)
	}
	if states["swap"].Status != "armed" {
		t.Errorf("status = %q, want armed", states["swap"].Status)
	}
}

func TestPressureFallback(t *testing.T) {
	now := time.Unix(0, 0)
	cfg := testConfig(&now)
	states := map[string]*State{}
	s := baseStats(now)
	s.System.SwapUsed = 930
	s.Swap = live.SeriesStats{Trend: live.TrendInsufficient, RateBytesPerSec: 1 << 20, ETABasis: "noisy_fit", NonNegativeRatio: 1, Samples: 60, SpanSeconds: 59}
	Evaluate(cfg, states, s)
	now = now.Add(10 * time.Second)
	ds := Evaluate(cfg, states, s)
	if len(ds) != 1 || ds[0].Title != "swapaudit: swap 93.0% and rising" {
		t.Fatalf("decisions = %+v", ds)
	}
	if ds[0].Event.ETASeconds != nil {
		t.Error("pressure fallback must emit a null ETA")
	}
	if ds[0].Event.Top != nil {
		t.Error("top must be null when no mover exists")
	}
}

func TestCooldown(t *testing.T) {
	now := time.Unix(0, 0)
	cfg := testConfig(&now)
	states := map[string]*State{}
	Evaluate(cfg, states, dangerSwap(now, 312))
	now = now.Add(10 * time.Second)
	if ds := Evaluate(cfg, states, dangerSwap(now, 312)); len(ds) != 1 || ds[0].Kind != "alert" {
		t.Fatalf("first alert failed: %+v", ds)
	}
	now = now.Add(10 * time.Second)
	Evaluate(cfg, states, clearSwap(now))
	now = now.Add(10 * time.Second)
	Evaluate(cfg, states, clearSwap(now))
	now = now.Add(10 * time.Second)
	Evaluate(cfg, states, clearSwap(now))
	now = now.Add(time.Second)
	Evaluate(cfg, states, dangerSwap(now, 312))
	now = now.Add(10 * time.Second)
	ds := Evaluate(cfg, states, dangerSwap(now, 312))
	if len(ds) != 1 || ds[0].Kind != "suppressed" || ds[0].Reason != "cooldown" {
		t.Fatalf("expected cooldown suppression, got %+v", ds)
	}
	now = now.Add(10 * time.Minute)
	Evaluate(cfg, states, clearSwap(now))
	now = now.Add(10 * time.Second)
	Evaluate(cfg, states, clearSwap(now))
	now = now.Add(10 * time.Second)
	Evaluate(cfg, states, clearSwap(now))
	now = now.Add(time.Second)
	Evaluate(cfg, states, dangerSwap(now, 312))
	now = now.Add(10 * time.Second)
	ds = Evaluate(cfg, states, dangerSwap(now, 312))
	if len(ds) != 1 || ds[0].Kind != "alert" {
		t.Fatalf("post-cooldown alert failed: %+v", ds)
	}
}

func TestHourlyCap(t *testing.T) {
	now := time.Unix(0, 0)
	cfg := testConfig(&now)
	cfg.Cooldown = 0
	states := map[string]*State{}
	incident := func() {
		Evaluate(cfg, states, dangerSwap(now, 312))
		now = now.Add(10 * time.Second)
		Evaluate(cfg, states, dangerSwap(now, 312))
		now = now.Add(10 * time.Second)
		Evaluate(cfg, states, clearSwap(now))
		now = now.Add(10 * time.Second)
		Evaluate(cfg, states, clearSwap(now))
		now = now.Add(10 * time.Second)
		Evaluate(cfg, states, clearSwap(now))
	}
	for i := 0; i < 4; i++ {
		incident()
	}
	Evaluate(cfg, states, dangerSwap(now, 312))
	now = now.Add(10 * time.Second)
	ds := Evaluate(cfg, states, dangerSwap(now, 312))
	if len(ds) != 1 || ds[0].Kind != "suppressed" || ds[0].Reason != "hourly_cap" {
		t.Fatalf("expected hourly cap suppression, got %+v", ds)
	}
}

func TestEscalation(t *testing.T) {
	now := time.Unix(0, 0)
	cfg := testConfig(&now)
	states := map[string]*State{}
	Evaluate(cfg, states, dangerSwap(now, 312))
	now = now.Add(10 * time.Second)
	Evaluate(cfg, states, dangerSwap(now, 312))
	now = now.Add(10 * time.Second)
	ds := Evaluate(cfg, states, dangerSwap(now, 200))
	if len(ds) != 1 || ds[0].Severity != "critical" {
		t.Fatalf("escalation failed: %+v", ds)
	}
	now = now.Add(10 * time.Second)
	if ds := Evaluate(cfg, states, dangerSwap(now, 200)); len(ds) != 0 {
		t.Errorf("escalation must happen once per incident: %+v", ds)
	}
}

func TestStaleTmpfsSuppressedOnce(t *testing.T) {
	now := time.Unix(0, 0)
	cfg := testConfig(&now)
	cfg.Resource = "tmpfs"
	states := map[string]*State{}
	s := baseStats(now)
	s.TmpfsSampledAgeSec = 100
	ds := Evaluate(cfg, states, s)
	if len(ds) != 1 || ds[0].Reason != "stale_tmpfs" {
		t.Fatalf("expected stale tmpfs suppression, got %+v", ds)
	}
	if ds := Evaluate(cfg, states, s); len(ds) != 0 {
		t.Errorf("stale suppression must be emitted once: %+v", ds)
	}
	s.TmpfsSampledAgeSec = 0
	Evaluate(cfg, states, s)
	if states["tmpfs"].StaleNotified {
		t.Error("stale flag must reset when the sample is fresh")
	}
}

func TestResourceFilter(t *testing.T) {
	now := time.Unix(0, 0)
	cfg := testConfig(&now)
	cfg.Resource = "swap"
	states := map[string]*State{}
	s := baseStats(now)
	s.System.TmpfsUsed = 990
	s.Tmpfs = live.SeriesStats{Trend: live.TrendFilling, RateBytesPerSec: 1 << 20, ETASeconds: int64Ptr(60), Samples: 60, SpanSeconds: 59, FitR2: 1, NonNegativeRatio: 1}
	if ds := Evaluate(cfg, states, s); len(ds) != 0 {
		t.Fatalf("resource=swap produced a tmpfs decision: %+v", ds)
	}
	if _, ok := states["tmpfs"]; ok {
		t.Error("resource=swap must not create tmpfs state")
	}
}

func TestFixtureSequence(t *testing.T) {
	cases := []struct {
		name   string
		alerts int
	}{
		{"idle", 0}, {"ramp", 1}, {"spike", 0}, {"recovering", 0}, {"noise", 0},
		{"short", 0}, {"pid-reuse", 0}, {"exit", 0}, {"stale-tmp", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.Open(filepath.Join("..", "live", "testdata", tc.name+".ndjson"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			samples, err := live.Replay(f)
			if err != nil {
				t.Fatal(err)
			}
			now := samples[0].At
			cfg := testConfig(&now)
			cfg.ETAThreshold, cfg.RearmETA, cfg.Resource = time.Hour, 2*time.Hour, "swap"
			states := map[string]*State{}
			alerts := 0
			for i := range samples {
				now = samples[i].At
				stats := live.Analyze(samples[:i+1], time.Minute, samples[i].TmpfsSampledAt, samples[i].At)
				for _, d := range Evaluate(cfg, states, stats) {
					if d.Kind != "alert" {
						continue
					}
					alerts++
					eta, top := "null", "null"
					if d.Event.ETASeconds != nil {
						eta = fmt.Sprint(*d.Event.ETASeconds)
					}
					if d.Event.Top != nil {
						top = d.Event.Top.Name
					}
					t.Logf("alert at=%s eta=%s top=%s", samples[i].At.Format(time.RFC3339), eta, top)
					if d.Event.Top == nil || d.Event.Top.Name != "gopls" {
						t.Errorf("ramp alert top = %+v, want gopls", d.Event.Top)
					}
					if tc.name == "ramp" {
						if d.Event.ETASeconds == nil {
							t.Fatal("ramp alert has a null ETA")
						}
						analytic := float64(stats.System.SwapTotal-stats.System.SwapUsed) / stats.Swap.RateBytesPerSec
						if math.Abs(float64(*d.Event.ETASeconds)-analytic)/analytic > 0.10 {
							t.Errorf("alert ETA %d vs analytic %.0f", *d.Event.ETASeconds, analytic)
						}
					}
				}
			}
			if alerts != tc.alerts {
				t.Errorf("alerts = %d, want %d", alerts, tc.alerts)
			}
		})
	}
}

func TestEventJSON(t *testing.T) {
	now := time.Unix(0, 0)
	s := dangerSwap(now, 312)
	s.SustainedSeconds = 42
	alert := buildEvent("alert", "swap", "warning", s, TopMover(s, "swap"), now, "swapaudit: swap full in 5m 12s", "")
	raw, err := json.Marshal(alert)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["type"] != "alert" || doc["resource"] != "swap" || doc["severity"] != "warning" {
		t.Errorf("event = %s", raw)
	}
	if doc["eta_seconds"].(float64) != 312 {
		t.Errorf("eta = %v", doc["eta_seconds"])
	}
	top := doc["top"].(map[string]any)
	if top["name"] != "gopls" || top["pid"].(float64) != 1420081 {
		t.Errorf("top = %v", top)
	}
	if doc["transport"] != nil {
		t.Error("transport must be null in JSON mode")
	}
	suppressed := buildEvent("suppressed", "swap", "", s, nil, now, "", "cooldown")
	if raw, _ := json.Marshal(suppressed); !strings.Contains(string(raw), `"reason":"cooldown"`) {
		t.Errorf("suppressed event = %s", raw)
	}
	if raw, _ := json.Marshal(buildEvent("recovered", "swap", "", s, nil, now, "recovered", "")); !strings.Contains(string(raw), `"type":"recovered"`) {
		t.Errorf("recovered event = %s", raw)
	}
}

func TestDeterminism(t *testing.T) {
	run := func() []string {
		now := time.Unix(0, 0)
		cfg := testConfig(&now)
		states := map[string]*State{}
		var out []string
		for i := 0; i < 6; i++ {
			for _, d := range Evaluate(cfg, states, dangerSwap(now, 312)) {
				out = append(out, d.Kind+":"+d.Resource+":"+d.Reason)
			}
			now = now.Add(5 * time.Second)
		}
		return out
	}
	a, b := run(), run()
	if fmt.Sprint(a) != fmt.Sprint(b) {
		t.Errorf("non-deterministic decisions: %v vs %v", a, b)
	}
}
