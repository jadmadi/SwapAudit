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
