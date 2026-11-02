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
	RSSBytes     int64
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
	SwapTotal    int64
	SwapUsed     int64
	SwapFree     int64
	MemTotal     int64
	MemUsed      int64
	MemAvail     int64
	MemFree      int64
	Buffers      int64
	Cached       int64
	SReclaimable int64
	Shmem        int64
	Dirty        int64
	Writeback    int64
	TmpfsUsed    int64
	TmpfsSize    int64
}

// PageCacheBytes returns the page cache and reclaimable slab that
// /proc/sys/vm/drop_caches can release. Shmem is excluded because it is tmpfs
// content and is only freed by deleting files or by swapping.
func (s SystemSwap) PageCacheBytes() int64 {
	v := s.Buffers + s.Cached + s.SReclaimable - s.Shmem
	if v < 0 {
		return 0
	}
	return v
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
	if it.SwapBytes > 0 && it.RSSBytes > 0 {
		return it.SwapBytes + it.RSSBytes
	}
	if it.SwapBytes > 0 {
		return it.SwapBytes
	}
	return it.RSSBytes
}
