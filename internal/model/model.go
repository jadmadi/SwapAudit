package model

import "time"

// Category represents the classification of a swap-consuming entity.
type Category string

const (
	CatUntouchedCache Category = "untouched-caches"
	CatAbandonedRun   Category = "abandoned-runs"
	CatDormantAgent   Category = "dormant-agents"
	CatAbandonedAgent Category = "abandoned-agents"
	CatActiveCache    Category = "active-caches"
	CatDesktopDaemon  Category = "desktop-daemons"
	CatActiveProcess  Category = "active-apps"
)

// SwapItem represents an individual process or filesystem cache occupying swap or tmpfs.
type SwapItem struct {
	PID          int
	StartTicks   int64
	Name         string
	Command      string
	Category     Category
	SwapBytes    int64
	SizeBytes    int64
	Age          time.Duration
	LastActive   time.Duration
	LastActiveAt time.Time
	Path         string
	Cleanable    bool
	Count        int // for aggregated rows
}

// SystemSwap represents overall system memory and swap status.
type SystemSwap struct {
	SwapTotal int64
	SwapUsed  int64
	SwapFree  int64
	MemTotal  int64
	MemUsed   int64
	MemAvail  int64
	TmpfsUsed int64
	TmpfsSize int64
}

// Col defines a column in renderTable.
type Col struct {
	Title string
	Width int
	Right bool // right-align
}

// ItemSize returns the effective reclaimable or display size for an item.
func ItemSize(it SwapItem) int64 {
	if it.SizeBytes > 0 {
		return it.SizeBytes
	}
	return it.SwapBytes
}
