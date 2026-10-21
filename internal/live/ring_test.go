//go:build linux

package live

import (
	"sync"
	"testing"
)

func TestRingEviction(t *testing.T) {
	r := NewRing(2)
	for i := 0; i < 3; i++ {
		r.Push(sample{MonoSeconds: float64(i)})
	}
	if r.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", r.Len())
	}
	got := r.Snapshot()
	if got[0].MonoSeconds != 1 || got[1].MonoSeconds != 2 {
		t.Errorf("FIFO order = %v, %v", got[0].MonoSeconds, got[1].MonoSeconds)
	}
}

func TestRingCapacityOneAndEmpty(t *testing.T) {
	if NewRing(0).Len() != 0 {
		t.Error("new ring must be empty")
	}
	r := NewRing(1)
	r.Push(sample{MonoSeconds: 1})
	r.Push(sample{MonoSeconds: 2})
	if got := r.Snapshot(); len(got) != 1 || got[0].MonoSeconds != 2 {
		t.Errorf("capacity one = %+v", got)
	}
}

func TestRingSnapshotDeepCopy(t *testing.T) {
	r := NewRing(1)
	s := sample{Processes: map[processKey]int64{{PID: 1}: 10}, Caches: map[string]int64{"/tmp/a": 5}, CacheAges: map[string]int64{"/tmp/a": 1}, Names: map[processKey]string{{PID: 1}: "p"}, Idle: map[processKey]int64{{PID: 1}: 2}}
	r.Push(s)
	snap := r.Snapshot()
	snap[0].Processes[processKey{PID: 1}] = 999
	snap[0].Caches["/tmp/a"] = 999
	if again := r.Snapshot(); again[0].Processes[processKey{PID: 1}] != 10 || again[0].Caches["/tmp/a"] != 5 {
		t.Error("Snapshot must return independent maps")
	}
}

func TestRingConcurrent(t *testing.T) {
	r := NewRing(8)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			r.Push(sample{MonoSeconds: float64(i)})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = r.Snapshot()
		}
	}()
	wg.Wait()
	if r.Len() != 8 {
		t.Errorf("Len() = %d, want 8", r.Len())
	}
}
