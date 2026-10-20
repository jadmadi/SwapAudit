package model

import "testing"

func TestItemSize(t *testing.T) {
	tmpItem := SwapItem{
		SizeBytes: 1024 * 1024,
		SwapBytes: 0,
	}
	if sz := ItemSize(tmpItem); sz != 1024*1024 {
		t.Errorf("ItemSize(tmpItem) = %d, want %d", sz, 1024*1024)
	}

	procItem := SwapItem{
		SizeBytes: 0,
		SwapBytes: 2048 * 1024,
	}
	if sz := ItemSize(procItem); sz != 2048*1024 {
		t.Errorf("ItemSize(procItem) = %d, want %d", sz, 2048*1024)
	}
}

func TestShortID(t *testing.T) {
	a := SwapItem{Path: "/tmp/go-build123"}
	b := SwapItem{Path: "/tmp/go-build123"}
	c := SwapItem{Path: "/tmp/go-build456"}
	d := SwapItem{PID: 42, StartTicks: 100}
	if ShortID(a) != ShortID(b) {
		t.Errorf("ShortID is not deterministic")
	}
	if ShortID(a) == ShortID(c) {
		t.Errorf("ShortID collision for different paths")
	}
	if len(ShortID(a)) != 8 {
		t.Errorf("ShortID length = %d, want 8", len(ShortID(a)))
	}
	if ShortID(d) == ShortID(a) {
		t.Errorf("PID-based and path-based IDs should differ")
	}
}
