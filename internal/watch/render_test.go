//go:build linux

package watch

import (
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/live"
)

func TestFormatTitle(t *testing.T) {
	eta := int64(312)
	if got := FormatTitle("swap", "warning", &eta, 81.3); got != "swapaudit: swap full in 5m 12s" {
		t.Errorf("title = %q", got)
	}
	if got := FormatTitle("tmpfs", "critical", nil, 95.5); got != "swapaudit: tmpfs 95.5% and rising" {
		t.Errorf("pressure title = %q", got)
	}
}

func TestFormatBodyProcess(t *testing.T) {
	now := time.Unix(0, 0)
	s := dangerSwap(now, 312)
	s.SustainedSeconds = 42
	body := FormatBody("swap", s, TopMover(s, "swap"), 0, 0)
	lines := strings.Split(body, "\n")
	if len(lines) != 3 {
		t.Fatalf("body lines = %d, want 3\n%s", len(lines), body)
	}
	for _, want := range []string{"800 B/1000 B (80.0%)", "120.0 MB/min", "sustained 42s", "gopls (PID 1420081)", "+96.0 MB/min", "now 1.9 GB", "run 'swapaudit'"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "\x1b") {
		t.Error("body contains ANSI escapes")
	}
	if len(body) > maxBodyBytes {
		t.Errorf("body = %d bytes, cap %d", len(body), maxBodyBytes)
	}
}

func TestFormatBodyCache(t *testing.T) {
	now := time.Unix(0, 0)
	s := baseStats(now)
	s.System.TmpfsUsed = 900
	s.Tmpfs = live.SeriesStats{Trend: live.TrendFilling, RateBytesPerSec: 1 << 20, Samples: 60, SpanSeconds: 59, FitR2: 1, NonNegativeRatio: 1}
	s.TmpfsMovers = []live.Mover{{Kind: "cache", Name: "/tmp/go-build998", DeltaBytes: 48 << 20, CurrentBytes: 2400 << 20}}
	body := FormatBody("tmpfs", s, TopMover(s, "tmpfs"), 605552640, 8*time.Second)
	for _, want := range []string{"/tmp/go-build998", "+48.0 MB/min", "577.5 MB reclaimable (sampled 8s ago)", "run 'swapaudit' to review"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "RAM") {
		t.Error("tmpfs body must not imply RAM is swap")
	}
}

func TestFormatBodyNullTop(t *testing.T) {
	now := time.Unix(0, 0)
	body := FormatBody("swap", baseStats(now), nil, 0, 0)
	if !strings.Contains(body, "top grower: unaccounted (shared/kernel)") {
		t.Errorf("body = %q", body)
	}
}

func TestFormatBodyTruncatesName(t *testing.T) {
	now := time.Unix(0, 0)
	long := strings.Repeat("x", 100)
	s := baseStats(now)
	s.Movers = []live.Mover{{Kind: "process", PID: intPtr(7), StartTicks: int64Ptr(1), Name: long, DeltaBytes: 1 << 20, CurrentBytes: 1 << 30}}
	body := FormatBody("swap", s, TopMover(s, "swap"), 0, 0)
	if strings.Contains(body, long) {
		t.Error("long name was not truncated")
	}
	if len(body) > maxBodyBytes {
		t.Errorf("body = %d bytes, cap %d", len(body), maxBodyBytes)
	}
}

func TestFormatStatus(t *testing.T) {
	now := time.Date(2026, 10, 21, 23, 41, 7, 0, time.UTC)
	s := dangerSwap(now, 312)
	s.Swap.RateBytesPerSec = 205.0 * 1024 * 1024 / 60
	s.Tmpfs = live.SeriesStats{Trend: live.TrendInsufficient, Samples: 60, SpanSeconds: 59, NonNegativeRatio: 1}
	s.System.TmpfsUsed = 500
	line := FormatStatus("both", s, TopMover(s, "swap"), 90)
	for _, want := range []string{"23:41:07", "swap 80.0%", "+205.0 MB/min", "eta 5m 12s", "top gopls"} {
		if !strings.Contains(line, want) {
			t.Errorf("status missing %q: %s", want, line)
		}
	}
	if strings.Contains(line, "tmpfs") {
		t.Errorf("insufficient tmpfs below pressure must be omitted: %s", line)
	}
	if strings.Contains(line, "\x1b") {
		t.Error("status contains ANSI escapes")
	}
}

func TestFormatStatusNullETA(t *testing.T) {
	now := time.Unix(0, 0)
	s := baseStats(now)
	s.System.SwapUsed = 950
	s.Swap = live.SeriesStats{Trend: live.TrendInsufficient, RateBytesPerSec: 1, Samples: 60, SpanSeconds: 59, NonNegativeRatio: 1}
	line := FormatStatus("swap", s, nil, 90)
	if !strings.Contains(line, "eta --") {
		t.Errorf("status = %q", line)
	}
}
