//go:build linux

package live

import (
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

// Trend classifies a series over the requested window.
type Trend string

const (
	TrendFilling      Trend = "filling"
	TrendRecovering   Trend = "recovering"
	TrendFlat         Trend = "flat"
	TrendInsufficient Trend = "insufficient"
)

const (
	minSamples          = 5
	spanFraction        = 0.8
	minR2               = 0.50
	minNonNegativeRatio = 0.70
	flatRate            = 1024.0
	maxMovers           = 10
	staleTmpfsAfter     = 60 * time.Second
	maxSamples          = 600
)

// SeriesStats is the estimator output for one resource.
type SeriesStats struct {
	Trend            Trend   `json:"trend"`
	RateBytesPerSec  float64 `json:"rate_bytes_per_sec"`
	ETASeconds       *int64  `json:"eta_seconds"`
	ETABasis         string  `json:"eta_basis"`
	FitR2            float64 `json:"fit_r2"`
	NonNegativeRatio float64 `json:"non_negative_ratio"`
	Samples          int     `json:"samples"`
	SpanSeconds      float64 `json:"span_seconds"`
}

// Mover is one entity whose usage changed across the window.
type Mover struct {
	Kind         string  `json:"kind"`
	PID          *int    `json:"pid"`
	StartTicks   *int64  `json:"start_ticks"`
	Name         string  `json:"name"`
	Path         *string `json:"path"`
	DeltaBytes   int64   `json:"delta_bytes"`
	CurrentBytes int64   `json:"current_bytes"`
	Started      bool    `json:"started"`
	Exited       bool    `json:"exited"`
}

// WindowStats is the window block embedded in once/stream output.
type WindowStats struct {
	Seconds             int              `json:"seconds"`
	Samples             int              `json:"samples"`
	Complete            bool             `json:"complete"`
	Swap                SeriesStats      `json:"swap"`
	Tmpfs               SeriesStats      `json:"tmpfs"`
	Movers              []Mover          `json:"movers"`
	TmpfsMovers         []Mover          `json:"tmpfs_movers"`
	UnattributedBytes   int64            `json:"unattributed_bytes"`
	ExitedBytes         int64            `json:"exited_bytes"`
	StartedBytes        int64            `json:"started_bytes"`
	TmpfsSampledAgeSec  int64            `json:"tmpfs_sampled_age_seconds"`
	SkippedTicks        int              `json:"skipped_ticks"`
	SlowLaneSkips       int              `json:"slow_lane_skips"`
	TmpfsCleanableBytes int64            `json:"tmpfs_cleanable_bytes"`
	TmpfsCleanableItems int              `json:"tmpfs_cleanable_items"`
	System              model.SystemSwap `json:"-"`
	SustainedSeconds    int64            `json:"-"`
	SampledAt           time.Time        `json:"-"`
}

type point struct {
	t float64
	y float64
}

// Analyze is pure and deterministic for a given sample slice.
func Analyze(samples []sample, window time.Duration, tmpfsSampledAt time.Time, now time.Time) WindowStats {
	stats := WindowStats{
		Seconds:            int(window / time.Second),
		Samples:            len(samples),
		Movers:             []Mover{},
		TmpfsMovers:        []Mover{},
		TmpfsSampledAgeSec: tmpfsAge(tmpfsSampledAt, now),
	}
	if len(samples) == 0 {
		stats.Swap = SeriesStats{Trend: TrendInsufficient, ETABasis: "insufficient_samples", NonNegativeRatio: 1}
		stats.Tmpfs = SeriesStats{Trend: TrendInsufficient, ETABasis: "insufficient_samples", NonNegativeRatio: 1}
		return stats
	}

	oldest, newest := samples[0], samples[len(samples)-1]
	stats.System = newest.System
	stats.SampledAt = newest.At
	stats.TmpfsCleanableBytes = newest.TmpfsCleanableBytes
	stats.TmpfsCleanableItems = newest.TmpfsCleanableItems
	stats.Swap = seriesStats(samples, window, func(s sample) int64 { return s.System.SwapUsed }, freeBytes(newest.System.SwapTotal, newest.System.SwapUsed))
	stats.Tmpfs = seriesStats(samples, window, func(s sample) int64 { return s.System.TmpfsUsed }, freeBytes(newest.System.TmpfsSize, newest.System.TmpfsUsed))

	if tmpfsSampledAt.IsZero() || now.Sub(tmpfsSampledAt) > staleTmpfsAfter {
		stats.Tmpfs.ETASeconds = nil
		stats.Tmpfs.ETABasis = "short_span"
		if stats.Tmpfs.Trend == TrendFilling {
			stats.Tmpfs.Trend = TrendInsufficient
		}
	}

	stats.Complete = len(samples) >= minSamples && stats.Swap.SpanSeconds >= spanFraction*window.Seconds()
	stats.Movers = processMovers(oldest, newest)
	stats.TmpfsMovers = cacheMovers(oldest, newest)

	var attributed, started, exited int64
	for key, cur := range newest.Processes {
		if old, ok := oldest.Processes[key]; ok {
			attributed += cur - old
		} else {
			started += cur
		}
	}
	for key, old := range oldest.Processes {
		if _, ok := newest.Processes[key]; !ok {
			exited += old
		}
	}
	stats.StartedBytes = started
	stats.ExitedBytes = exited
	stats.UnattributedBytes = newest.System.SwapUsed - oldest.System.SwapUsed - attributed - started + exited
	return stats
}

func seriesStats(samples []sample, window time.Duration, value func(sample) int64, free int64) SeriesStats {
	st := SeriesStats{Samples: len(samples), Trend: TrendInsufficient, ETABasis: "insufficient_samples", NonNegativeRatio: 1}
	if len(samples) < 2 {
		return st
	}
	pts := make([]point, len(samples))
	for i, s := range samples {
		pts[i] = point{t: s.MonoSeconds, y: float64(value(s))}
	}
	st.SpanSeconds = pts[len(pts)-1].t - pts[0].t
	st.RateBytesPerSec, _, st.FitR2 = ols(pts)
	st.NonNegativeRatio = nonNegativeRatio(pts)

	if len(pts) < minSamples {
		return st
	}
	if st.SpanSeconds < spanFraction*window.Seconds() {
		st.ETABasis = "short_span"
		return st
	}
	if st.RateBytesPerSec < 0 {
		st.Trend, st.ETABasis = TrendRecovering, "recovering"
		return st
	}
	if st.RateBytesPerSec < flatRate {
		st.Trend, st.ETABasis = TrendFlat, "flat"
		return st
	}
	if st.FitR2 < minR2 {
		st.ETABasis = "noisy_fit"
		return st
	}
	if st.NonNegativeRatio < minNonNegativeRatio {
		st.ETABasis = "non_monotone"
		return st
	}

	st.Trend, st.ETABasis = TrendFilling, "ols"
	eta := int64(float64(free) / st.RateBytesPerSec)
	if eta < 0 {
		eta = 0
	}
	st.ETASeconds = &eta
	return st
}

func ols(pts []point) (slope, intercept, r2 float64) {
	n := float64(len(pts))
	var sumX, sumY, sumXX, sumXY float64
	for _, p := range pts {
		sumX += p.t
		sumY += p.y
		sumXX += p.t * p.t
		sumXY += p.t * p.y
	}
	denom := n*sumXX - sumX*sumX
	meanY := sumY / n
	if denom == 0 {
		return 0, meanY, 1
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n

	var ssRes, ssTot float64
	for _, p := range pts {
		pred := intercept + slope*p.t
		ssRes += (p.y - pred) * (p.y - pred)
		ssTot += (p.y - meanY) * (p.y - meanY)
	}
	if ssTot == 0 {
		return slope, intercept, 1
	}
	r2 = 1 - ssRes/ssTot
	return slope, intercept, math.Max(0, math.Min(1, r2))
}

func nonNegativeRatio(pts []point) float64 {
	if len(pts) < 2 {
		return 1
	}
	nonNegative := 0
	for i := 1; i < len(pts); i++ {
		if pts[i].y-pts[i-1].y >= 0 {
			nonNegative++
		}
	}
	return float64(nonNegative) / float64(len(pts)-1)
}

func freeBytes(total, used int64) int64 {
	free := total - used
	if free < 0 {
		return 0
	}
	return free
}

func tmpfsAge(sampledAt, now time.Time) int64 {
	if sampledAt.IsZero() {
		return 0
	}
	age := now.Sub(sampledAt).Seconds()
	if age < 0 {
		return 0
	}
	return int64(age)
}

func processMovers(oldest, newest sample) []Mover {
	movers := make([]Mover, 0)
	for key, cur := range newest.Processes {
		old, ok := oldest.Processes[key]
		if !ok || cur == old {
			continue
		}
		pid, ticks := key.PID, key.StartTicks
		name := newest.Names[key]
		if name == "" {
			name = "pid-" + strconv.Itoa(pid)
		}
		movers = append(movers, Mover{
			Kind: "process", PID: &pid, StartTicks: &ticks, Name: name,
			DeltaBytes: cur - old, CurrentBytes: cur,
		})
	}
	return sortMovers(movers)
}

func cacheMovers(oldest, newest sample) []Mover {
	movers := make([]Mover, 0)
	for path, cur := range newest.Caches {
		old, ok := oldest.Caches[path]
		if !ok || cur == old {
			continue
		}
		p := path
		movers = append(movers, Mover{
			Kind: "cache", Name: path, Path: &p,
			DeltaBytes: cur - old, CurrentBytes: cur,
		})
	}
	return sortMovers(movers)
}

func sortMovers(movers []Mover) []Mover {
	sort.Slice(movers, func(i, j int) bool {
		ai, aj := abs64(movers[i].DeltaBytes), abs64(movers[j].DeltaBytes)
		if ai != aj {
			return ai > aj
		}
		if movers[i].Name != movers[j].Name {
			return movers[i].Name < movers[j].Name
		}
		return pidValue(movers[i]) < pidValue(movers[j])
	})
	if len(movers) > maxMovers {
		movers = movers[:maxMovers]
	}
	return movers
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func pidValue(m Mover) int {
	if m.PID == nil {
		return math.MaxInt32
	}
	return *m.PID
}
