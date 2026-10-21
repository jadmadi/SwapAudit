//go:build linux

package live

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
	"github.com/jadmadi/swapaudit/internal/snapshot"
)

// Config controls the headless sampler.
type Config struct {
	Interval    time.Duration
	Window      time.Duration
	TmpInterval time.Duration
	Duration    time.Duration
	RecordPath  string
	NoCommand   bool
	Once        bool
	Now         func() time.Time
	Clock       func() time.Time
	Mono        func() time.Time
	ScanFast    func(now time.Time) sample
	ScanTmp     func() []model.SwapItem
	Snapshot    func(now time.Time) snapshot.Snapshot
	OnSample    func(WindowStats)
	Out         io.Writer
	Err         io.Writer
}

// Scanner reads only the fast-lane proc files.
type Scanner struct {
	prev map[processKey]int64
	idle map[processKey]int64
}

// NewScanner creates an empty fast-lane scanner.
func NewScanner() *Scanner {
	return &Scanner{prev: map[processKey]int64{}, idle: map[processKey]int64{}}
}

// Scan captures system counters and processes with VmSwap > 0.
func (s *Scanner) Scan(now time.Time) sample {
	out := emptySample(now, proc.ReadSystemSwap())
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return out
	}
	uptimeMs := proc.ReadUptime() * 1000
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		swap, _ := proc.ReadProcStatus(pid)
		if swap <= 0 {
			continue
		}
		key := processKey{PID: pid, StartTicks: proc.ReadStartTicks(pid)}
		out.Processes[key] = swap
		out.Names[key] = proc.ReadComm(pid)
		if prev, ok := s.prev[key]; !ok || prev != swap {
			if last := proc.ReadProcessLastActive(pid, uptimeMs); last >= 0 {
				out.Idle[key] = int64(last / time.Second)
			} else {
				out.Idle[key] = -1
			}
		} else if idle, ok := s.idle[key]; ok {
			out.Idle[key] = idle
		}
	}
	s.prev, s.idle = out.Processes, out.Idle
	return out
}

// FixedScanner replays already-collected items, avoiding a second scan for --once.
type FixedScanner struct {
	items  []model.SwapItem
	system model.SystemSwap
}

// NewFixedScanner builds a scanner over a completed item slice.
func NewFixedScanner(items []model.SwapItem, sys model.SystemSwap) *FixedScanner {
	return &FixedScanner{items: items, system: sys}
}

// Scan converts the fixed items into a sample.
func (f *FixedScanner) Scan(now time.Time) sample {
	s := emptySample(now, f.system)
	for _, it := range f.items {
		if it.PID > 0 && it.SwapBytes > 0 {
			key := processKey{PID: it.PID, StartTicks: it.StartTicks}
			s.Processes[key] = it.SwapBytes
			s.Names[key] = it.Name
			if it.LastActive >= 0 {
				s.Idle[key] = int64(it.LastActive / time.Second)
			}
		}
		if it.Path != "" {
			s.Caches[it.Path] = it.SizeBytes
			s.CacheAges[it.Path] = int64(it.Age / time.Second)
		}
	}
	return s
}

func emptySample(now time.Time, sys model.SystemSwap) sample {
	return sample{
		At: now, System: sys,
		Processes: map[processKey]int64{}, Caches: map[string]int64{}, CacheAges: map[string]int64{},
		Names: map[processKey]string{}, Idle: map[processKey]int64{},
	}
}

type tmpCache struct {
	mu             sync.Mutex
	caches         map[string]int64
	ages           map[string]int64
	sampledAt      time.Time
	cleanableBytes int64
	cleanableItems int
	skips          int
}

func (c *tmpCache) set(items []model.SwapItem, now time.Time) {
	caches := map[string]int64{}
	ages := map[string]int64{}
	var cleanable int64
	var count int
	for _, it := range items {
		if it.Path == "" {
			continue
		}
		caches[it.Path] = it.SizeBytes
		ages[it.Path] = int64(it.Age / time.Second)
		if it.Cleanable {
			cleanable += model.ItemSize(it)
			count++
		}
	}
	c.mu.Lock()
	c.caches, c.ages, c.sampledAt, c.cleanableBytes, c.cleanableItems = caches, ages, now, cleanable, count
	c.mu.Unlock()
}

func (c *tmpCache) attach(s *sample) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s.Caches, s.CacheAges, s.TmpfsSampledAt = c.caches, c.ages, c.sampledAt
	s.TmpfsCleanableBytes, s.TmpfsCleanableItems = c.cleanableBytes, c.cleanableItems
}

func (c *tmpCache) skipCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.skips
}

func (c *tmpCache) addSkip() {
	c.mu.Lock()
	c.skips++
	c.mu.Unlock()
}

type onceDocument struct {
	Type      string            `json:"type"`
	SampledAt string            `json:"sampled_at"`
	Snapshot  snapshot.Snapshot `json:"snapshot"`
	Window    WindowStats       `json:"window"`
}

type streamDocument struct {
	Type             string          `json:"type"`
	SampledAt        string          `json:"sampled_at"`
	MonotonicSeconds float64         `json:"monotonic_seconds"`
	System           snapshot.System `json:"system"`
	Window           WindowStats     `json:"window"`
}

type stoppedDocument struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	SampledAt string `json:"sampled_at"`
}

// Run samples until --once completes, the duration elapses, or a signal arrives.
func Run(ctx context.Context, cfg Config) error {
	if cfg.ScanFast == nil {
		return errors.New("live: ScanFast is required")
	}
	if cfg.Interval <= 0 || cfg.Window <= 0 || cfg.TmpInterval <= 0 {
		return errors.New("live: interval, window, and tmp interval must be positive")
	}
	out := cfg.Out
	if out == nil {
		out = io.Discard
	}
	clock := cfg.Clock
	if clock == nil {
		clock = cfg.Now
	}
	if clock == nil {
		clock = time.Now
	}
	mono := cfg.Mono
	if mono == nil {
		mono = time.Now
	}
	if cfg.Once {
		return runOnce(cfg, clock(), out)
	}
	return runStream(ctx, cfg, out, clock, mono)
}

func runOnce(cfg Config, now time.Time, out io.Writer) error {
	if cfg.Snapshot == nil {
		return errors.New("live: Snapshot is required for --once")
	}
	s := cfg.ScanFast(now)
	if cfg.ScanTmp != nil {
		cache := &tmpCache{}
		cache.set(cfg.ScanTmp(), now)
		cache.attach(&s)
	}
	if cfg.RecordPath != "" {
		rec, err := NewRecorder(cfg.RecordPath)
		if err != nil {
			return err
		}
		if err := rec.Write(s); err != nil {
			rec.Close()
			return err
		}
		if err := rec.Close(); err != nil {
			return err
		}
	}
	ring := NewRing(1)
	ring.Push(s)
	stats := Analyze(ring.Snapshot(), cfg.Window, s.TmpfsSampledAt, now)
	doc := onceDocument{Type: "once", SampledAt: now.Format(time.RFC3339), Snapshot: cfg.Snapshot(now), Window: stats}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

func runStream(ctx context.Context, cfg Config, out io.Writer, clock, mono func() time.Time) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	var wg sync.WaitGroup
	defer wg.Wait()
	defer stop()

	var rec *Recorder
	if cfg.RecordPath != "" {
		var err error
		if rec, err = NewRecorder(cfg.RecordPath); err != nil {
			return err
		}
	}
	cache := &tmpCache{}
	wg.Add(1)
	go func() { defer wg.Done(); runSlowLane(ctx, cfg, cache, clock) }()

	buf := bufio.NewWriter(out)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	ring := NewRing(capacity(cfg.Window, cfg.Interval))
	startMono := mono()
	startWall := clock()
	lastMono := -1.0
	skipped := 0
	tick := func() error {
		now := clock()
		m := mono().Sub(startMono).Seconds()
		if m <= lastMono {
			skipped++
			return nil
		}
		tickStart := mono()
		s := cfg.ScanFast(now)
		s.MonoSeconds = m
		cache.attach(&s)
		ring.Push(s)
		if rec != nil {
			if err := rec.Write(s); err != nil {
				return err
			}
		}
		stats := Analyze(ring.Snapshot(), cfg.Window, s.TmpfsSampledAt, now)
		stats.SkippedTicks = skipped
		stats.SlowLaneSkips = cache.skipCount()
		stats.System = s.System
		if cfg.OnSample != nil {
			cfg.OnSample(stats)
		}
		line := streamDocument{
			Type: "sample", SampledAt: now.Format(time.RFC3339), MonotonicSeconds: m,
			System: rawSystem(s.System), Window: stats,
		}
		if err := enc.Encode(line); err != nil {
			return err
		}
		if err := buf.Flush(); err != nil {
			return err
		}
		if mono().Sub(tickStart) > cfg.Interval {
			skipped++
		}
		lastMono = m
		return nil
	}

	finish := func(reason string, runErr error) error {
		if runErr == nil {
			line := stoppedDocument{Type: "stopped", Reason: reason, SampledAt: clock().Format(time.RFC3339)}
			if err := enc.Encode(line); err != nil {
				runErr = err
			} else if err := buf.Flush(); err != nil {
				runErr = err
			}
		}
		if rec != nil {
			if err := rec.Close(); err != nil && runErr == nil {
				runErr = err
			}
		}
		return runErr
	}

	if ctx.Err() == nil {
		if err := tick(); err != nil {
			return finish("", err)
		}
	}
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return finish("signal", nil)
		case <-ticker.C:
			if cfg.Duration > 0 && clock().Sub(startWall) >= cfg.Duration {
				return finish("duration", nil)
			}
			if err := tick(); err != nil {
				return finish("", err)
			}
		}
	}
}

func runSlowLane(ctx context.Context, cfg Config, cache *tmpCache, clock func() time.Time) {
	if cfg.ScanTmp == nil {
		return
	}
	scan := func() {
		started := clock()
		cache.set(cfg.ScanTmp(), clock())
		if clock().Sub(started) > cfg.TmpInterval {
			cache.addSkip()
		}
	}
	scan()
	ticker := time.NewTicker(cfg.TmpInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scan()
		}
	}
}

func capacity(window, interval time.Duration) int {
	c := int(math.Ceil(window.Seconds() / interval.Seconds()))
	if c < 1 {
		c = 1
	}
	if c > maxSamples {
		c = maxSamples
	}
	return c
}
