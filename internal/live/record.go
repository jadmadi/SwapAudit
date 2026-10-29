//go:build linux

package live

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/snapshot"
)

type rawProcess struct {
	PID        int    `json:"pid"`
	StartTicks int64  `json:"start_ticks"`
	Name       string `json:"name"`
	SwapBytes  int64  `json:"swap_bytes"`
}

type rawCache struct {
	Path       string `json:"path"`
	SizeBytes  int64  `json:"size_bytes"`
	AgeSeconds int64  `json:"age_seconds"`
}

type rawSample struct {
	SchemaVersion       int             `json:"schema_version"`
	Type                string          `json:"type"`
	SampledAt           string          `json:"sampled_at"`
	MonotonicSeconds    float64         `json:"monotonic_seconds"`
	System              snapshot.System `json:"system"`
	Processes           []rawProcess    `json:"processes"`
	Caches              []rawCache      `json:"caches"`
	TmpfsSampledAt      string          `json:"tmpfs_sampled_at,omitempty"`
	TmpfsCleanableBytes int64           `json:"tmpfs_cleanable_bytes,omitempty"`
	TmpfsCleanableItems int             `json:"tmpfs_cleanable_items,omitempty"`
}

// Recorder writes one raw NDJSON sample per fast tick with mode 0600.
type Recorder struct {
	file *os.File
	buf  *bufio.Writer
	enc  *json.Encoder
}

// NewRecorder opens path for truncation with owner-only permissions.
func NewRecorder(path string) (*Recorder, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, err
	}
	buf := bufio.NewWriter(f)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	return &Recorder{file: f, buf: buf, enc: enc}, nil
}

// Write appends one raw line and flushes it.
func (r *Recorder) Write(s sample) error {
	if err := r.enc.Encode(toRaw(s)); err != nil {
		return err
	}
	return r.buf.Flush()
}

// Close flushes and closes the recorder.
func (r *Recorder) Close() error {
	if err := r.buf.Flush(); err != nil {
		r.file.Close()
		return err
	}
	return r.file.Close()
}

func toRaw(s sample) rawSample {
	out := rawSample{
		SchemaVersion:       1,
		Type:                "raw_sample",
		SampledAt:           s.At.Format(time.RFC3339),
		MonotonicSeconds:    s.MonoSeconds,
		System:              rawSystem(s.System),
		Processes:           []rawProcess{},
		Caches:              []rawCache{},
		TmpfsCleanableBytes: s.TmpfsCleanableBytes,
		TmpfsCleanableItems: s.TmpfsCleanableItems,
	}
	if !s.TmpfsSampledAt.IsZero() {
		out.TmpfsSampledAt = s.TmpfsSampledAt.Format(time.RFC3339)
	}

	keys := make([]processKey, 0, len(s.Processes))
	for key := range s.Processes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].PID != keys[j].PID {
			return keys[i].PID < keys[j].PID
		}
		return keys[i].StartTicks < keys[j].StartTicks
	})
	for _, key := range keys {
		out.Processes = append(out.Processes, rawProcess{
			PID: key.PID, StartTicks: key.StartTicks, Name: s.Names[key], SwapBytes: s.Processes[key],
		})
	}

	paths := make([]string, 0, len(s.Caches))
	for path := range s.Caches {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		out.Caches = append(out.Caches, rawCache{Path: path, SizeBytes: s.Caches[path], AgeSeconds: s.CacheAges[path]})
	}
	return out
}

func rawSystem(sys model.SystemSwap) snapshot.System {
	nn := func(v int64) int64 {
		if v < 0 {
			return 0
		}
		return v
	}
	return snapshot.System{
		SwapTotalBytes: nn(sys.SwapTotal), SwapUsedBytes: nn(sys.SwapUsed), SwapFreeBytes: nn(sys.SwapFree),
		MemTotalBytes: nn(sys.MemTotal), MemUsedBytes: nn(sys.MemUsed), MemAvailableBytes: nn(sys.MemAvail),
		TmpfsTotalBytes: nn(sys.TmpfsSize), TmpfsUsedBytes: nn(sys.TmpfsUsed),
	}
}

// Replay parses raw NDJSON lines back into samples for tests and analysis.
func Replay(r io.Reader) ([]sample, error) {
	dec := json.NewDecoder(r)
	var out []sample
	for {
		var raw rawSample
		err := dec.Decode(&raw)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		s, err := raw.toSample()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
}

func (r rawSample) toSample() (sample, error) {
	at, err := time.Parse(time.RFC3339, r.SampledAt)
	if err != nil {
		return sample{}, fmt.Errorf("parse sampled_at: %w", err)
	}
	s := sample{
		At:                  at,
		MonoSeconds:         r.MonotonicSeconds,
		System:              toSystem(r.System),
		Processes:           make(map[processKey]int64, len(r.Processes)),
		Caches:              make(map[string]int64, len(r.Caches)),
		CacheAges:           make(map[string]int64, len(r.Caches)),
		Names:               make(map[processKey]string, len(r.Processes)),
		Idle:                make(map[processKey]int64),
		TmpfsCleanableBytes: r.TmpfsCleanableBytes,
		TmpfsCleanableItems: r.TmpfsCleanableItems,
	}
	for _, p := range r.Processes {
		key := processKey{PID: p.PID, StartTicks: p.StartTicks}
		s.Processes[key] = p.SwapBytes
		s.Names[key] = p.Name
	}
	for _, c := range r.Caches {
		s.Caches[c.Path] = c.SizeBytes
		s.CacheAges[c.Path] = c.AgeSeconds
	}
	if r.TmpfsSampledAt != "" {
		if sampledAt, err := time.Parse(time.RFC3339, r.TmpfsSampledAt); err == nil {
			s.TmpfsSampledAt = sampledAt
		}
	}
	return s, nil
}

func toSystem(s snapshot.System) model.SystemSwap {
	return model.SystemSwap{
		SwapTotal: s.SwapTotalBytes, SwapUsed: s.SwapUsedBytes, SwapFree: s.SwapFreeBytes,
		MemTotal: s.MemTotalBytes, MemUsed: s.MemUsedBytes, MemAvail: s.MemAvailableBytes,
		TmpfsSize: s.TmpfsTotalBytes, TmpfsUsed: s.TmpfsUsedBytes,
	}
}
