//go:build linux

package live

import (
	"sync"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

type processKey struct {
	PID        int
	StartTicks int64
}

type sample struct {
	At                  time.Time
	MonoSeconds         float64
	System              model.SystemSwap
	Processes           map[processKey]int64
	Caches              map[string]int64
	CacheAges           map[string]int64
	Names               map[processKey]string
	Idle                map[processKey]int64
	TmpfsSampledAt      time.Time
	TmpfsCleanableBytes int64
	TmpfsCleanableItems int
}

// Ring is a bounded FIFO window safe for one writer and one reader.
type Ring struct {
	mu       sync.Mutex
	capacity int
	items    []sample
}

// NewRing creates a ring with a hard capacity floor of one.
func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{capacity: capacity, items: make([]sample, 0, capacity)}
}

// Push appends a sample, evicting the oldest at capacity.
func (r *Ring) Push(s sample) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) < r.capacity {
		r.items = append(r.items, s)
		return
	}
	copy(r.items, r.items[1:])
	r.items[len(r.items)-1] = s
}

// Snapshot returns a deep copy so the analyzer cannot race the writer.
func (r *Ring) Snapshot() []sample {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]sample, len(r.items))
	for i, s := range r.items {
		out[i] = s
		out[i].Processes = cloneInt64Map(s.Processes)
		out[i].Caches = cloneInt64Map(s.Caches)
		out[i].CacheAges = cloneInt64Map(s.CacheAges)
		out[i].Names = cloneStringMap(s.Names)
		out[i].Idle = cloneInt64Map(s.Idle)
	}
	return out
}

// Len reports the number of buffered samples.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items)
}

func cloneInt64Map[K comparable](in map[K]int64) map[K]int64 {
	out := make(map[K]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringMap[K comparable](in map[K]string) map[K]string {
	out := make(map[K]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
