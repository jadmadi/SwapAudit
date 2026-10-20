package model

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// Category represents the classification of a swap-consuming entity.
type Category string

const (
	CatUntouchedCache Category = "untouched-caches"
	CatAbandonedRun   Category = "abandoned-runs"
	CatRecentCache    Category = "recent-caches"
	CatInUseCache     Category = "in-use-caches"
	CatOrphanedCache  Category = "orphaned-caches"
	CatDormantAgent   Category = "dormant-agents"
	CatAbandonedAgent Category = "abandoned-agents"
	CatDesktopDaemon  Category = "desktop-daemons"
	CatActiveProcess  Category = "active-apps"

	// CatActiveCache is a deprecated alias. The rendered value is recent-caches.
	CatActiveCache = CatRecentCache
)

// Kind identifies which reclaim pool an item belongs to.
type Kind string

const (
	KindProcess Kind = "process"
	KindCache   Kind = "cache"
	KindHybrid  Kind = "hybrid"
)

// CacheKind classifies a /tmp cache by its expected lifecycle.
type CacheKind string

const (
	CacheBuild   CacheKind = "build"
	CacheRun     CacheKind = "run"
	CacheGeneric CacheKind = "generic"
)

// SwapItem represents an individual process or filesystem cache occupying swap or tmpfs.
type SwapItem struct {
	ID              string
	Kind            Kind
	PID             int
	StartTicks      int64
	UID             uint32
	Name            string
	Command         string
	Category        Category
	CacheKind       CacheKind
	SwapBytes       int64
	SizeBytes       int64
	AllocatedBytes  int64
	ApparentBytes   int64
	Age             time.Duration
	LastActive      time.Duration
	LastActiveAt    time.Time
	LastActiveKnown bool
	Path            string
	Cleanable       bool
	Count           int // for aggregated rows

	// Cache liveness evidence.
	OpenHandles  int
	CwdInside    bool
	MappedInside bool
	Evidence     string

	// Agent evidence.
	Agent        bool
	IdleKnown    bool
	OrphanReason string
	ParentPID    int
	ParentComm   string
	ParentAlive  bool
	TTYNr        int64
	Cgroup       string
	RSS          int64
	Signal       string

	// Display grouping.
	GroupKey   string
	GroupCount int
}

// SystemSwap represents overall system memory and swap status.
type SystemSwap struct {
	SwapTotal  int64
	SwapUsed   int64
	SwapFree   int64
	MemTotal   int64
	MemUsed    int64
	MemAvail   int64
	TmpfsUsed  int64
	TmpfsSize  int64
	TmpfsFree  int64
	TmpfsAvail int64
}

// Col defines a column in RenderTable.
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

// ShortID returns a deterministic eight-hex-character identity for an item.
func ShortID(it SwapItem) string {
	key := it.Path
	if key == "" {
		key = fmt.Sprintf("%d:%d", it.PID, it.StartTicks)
	}
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", sum[:4])
}
