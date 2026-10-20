package report

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

// Pool identifies a reclaim pool. Pools are not additive.
type Pool string

const (
	PoolProcessSwap Pool = "process-swap"
	PoolTmpfs       Pool = "tmpfs"
)

// ReasonBytes is one explicit scan exclusion or residual reason.
type ReasonBytes struct {
	Reason string `json:"reason"`
	Bytes  int64  `json:"bytes"`
	Count  int    `json:"count"`
}

// Attribution reconciles scanned bytes with a kernel total.
type Attribution struct {
	TotalBytes        int64
	AttributedBytes   int64
	UnattributedBytes int64
	OverlapBytes      int64
	Reasons           []ReasonBytes
}

// Reclaim separates the two reclaim pools.
type Reclaim struct {
	TmpfsBytes   int64
	SwapBytes    int64
	CacheCount   int
	ProcessCount int
}

// Population records every count that explains a displayed population.
type Population struct {
	RawItems           int
	AggregatedShells   int
	AggregatedRows     int
	Displayed          int
	HiddenByLimit      int
	HiddenBySizeFilter int
	Cleanable          int
	Protected          int

	Processes         int
	ProcessesWithSwap int
	Agents            int
	AgentsWithSwap    int
	AgentsZeroSwap    int
	AgentsDormant     int
	AgentsOrphaned    int
	AgentsParentAlive int
	AgentsIdleUnknown int
	AgentsActive      int
	Vanished          int
	PermissionDenied  int
}

// CategoryTotal is one category row. Shares are computed against the pool's
// attributed bytes and the pool's system total.
type CategoryTotal struct {
	Category          model.Category
	Pool              Pool
	Entities          int
	AttributedBytes   int64
	ReclaimableBytes  int64
	ShareOfAttributed float64
	ShareOfSystem     float64
}

// Thresholds records the thresholds that produced the classification.
type Thresholds struct {
	MaxAge         time.Duration
	IdleThreshold  time.Duration
	OrphanCacheAge time.Duration
}

// Scan carries scanner metadata that is not derivable from items.
type Scan struct {
	TmpfsReasons       []ReasonBytes
	SwapReasons        []ReasonBytes
	FallbackCount      int
	ScannedCaches      int
	InUseCaches        int
	OrphanedCaches     int
	RecentCaches       int
	EvidenceIncomplete int
	OpenIndexTruncated bool
	OpenIndexErrors    int
	OpenIndexPIDs      int
	OpenIndexFDs       int
}

// Host identifies the host that produced the report.
type Host struct {
	KernelRelease string
	UptimeSeconds int64
}

// SortMode selects the item ordering used by renderers.
type SortMode string

const (
	SortSize SortMode = "size"
	SortAge  SortMode = "age"
)

// AuditReport is the single source of truth for every rendered total.
type AuditReport struct {
	GeneratedAt time.Time
	Host        Host
	Sort        SortMode
	System      model.SystemSwap
	Swap        Attribution
	Tmpfs       Attribution
	Reclaim     Reclaim
	Categories  []CategoryTotal
	Population  Population
	Thresholds  Thresholds
	Items       []model.SwapItem
	Scan        Scan
}

var processCategoryOrder = []model.Category{
	model.CatDormantAgent,
	model.CatAbandonedAgent,
	model.CatDesktopDaemon,
	model.CatActiveProcess,
}

var cacheCategoryOrder = []model.Category{
	model.CatUntouchedCache,
	model.CatAbandonedRun,
	model.CatOrphanedCache,
	model.CatInUseCache,
	model.CatRecentCache,
}

// Build constructs a report from one scan snapshot. It does not mutate items.
func Build(now time.Time, system model.SystemSwap, items []model.SwapItem, thresholds Thresholds) (AuditReport, error) {
	return BuildWithScan(now, system, items, thresholds, Scan{})
}

// BuildWithScan constructs a report including scanner metadata.
func BuildWithScan(now time.Time, system model.SystemSwap, items []model.SwapItem, thresholds Thresholds, scan Scan) (AuditReport, error) {
	if now.IsZero() {
		now = time.Now()
	}
	r := AuditReport{
		GeneratedAt: now,
		Sort:        SortSize,
		System:      system,
		Thresholds:  thresholds,
		Items:       append([]model.SwapItem(nil), items...),
		Scan:        scan,
	}

	for _, it := range r.Items {
		if it.SwapBytes > 0 && it.AllocatedBytes > 0 && it.Kind != model.KindHybrid {
			return AuditReport{}, fmt.Errorf("item %q has both swap and tmpfs bytes without Kind=hybrid", it.Name)
		}
	}

	var processSwap, tmpfsAttributed int64
	var cleanableProcesses, cleanableCaches int
	processReclaim := make(map[model.Category]int64)
	cacheReclaim := make(map[model.Category]int64)
	processAttributed := make(map[model.Category]int64)
	cacheAttributed := make(map[model.Category]int64)
	processEntities := make(map[model.Category]int)
	cacheEntities := make(map[model.Category]int)

	for _, it := range r.Items {
		cat := it.Category
		switch {
		case it.SwapBytes > 0 || it.Kind == model.KindProcess:
			processSwap += it.SwapBytes
			processAttributed[cat] += it.SwapBytes
			processEntities[cat] += itemCount(it)
			if it.Cleanable {
				processReclaim[cat] += it.SwapBytes
				r.Reclaim.SwapBytes += it.SwapBytes
				cleanableProcesses++
			}
		case it.AllocatedBytes > 0 || it.Kind == model.KindCache:
			tmpfsAttributed += it.AllocatedBytes
			cacheAttributed[cat] += it.AllocatedBytes
			cacheEntities[cat] += itemCount(it)
			if it.Cleanable {
				cacheReclaim[cat] += it.AllocatedBytes
				r.Reclaim.TmpfsBytes += it.AllocatedBytes
				cleanableCaches++
			}
		}
		if it.Cleanable {
			r.Population.Cleanable++
		}
	}

	r.Swap = attribution(system.SwapUsed, processSwap, scan.SwapReasons, "kernel-or-shmem")
	r.Tmpfs = attribution(system.TmpfsUsed, tmpfsAttributed, scan.TmpfsReasons, "metadata-or-other")
	r.Reclaim.CacheCount = cleanableCaches
	r.Reclaim.ProcessCount = cleanableProcesses

	r.Categories = make([]CategoryTotal, 0, len(processCategoryOrder)+len(cacheCategoryOrder))
	for _, c := range processCategoryOrder {
		r.Categories = append(r.Categories, CategoryTotal{
			Category:         c,
			Pool:             PoolProcessSwap,
			Entities:         processEntities[c],
			AttributedBytes:  processAttributed[c],
			ReclaimableBytes: processReclaim[c],
		})
	}
	for _, c := range cacheCategoryOrder {
		r.Categories = append(r.Categories, CategoryTotal{
			Category:         c,
			Pool:             PoolTmpfs,
			Entities:         cacheEntities[c],
			AttributedBytes:  cacheAttributed[c],
			ReclaimableBytes: cacheReclaim[c],
		})
	}
	for i := range r.Categories {
		ct := &r.Categories[i]
		if ct.Pool == PoolProcessSwap {
			if r.Swap.AttributedBytes > 0 {
				ct.ShareOfAttributed = float64(ct.AttributedBytes) / float64(r.Swap.AttributedBytes)
			}
			if r.Swap.TotalBytes > 0 {
				ct.ShareOfSystem = float64(ct.AttributedBytes) / float64(r.Swap.TotalBytes)
			}
		} else {
			if r.Tmpfs.AttributedBytes > 0 {
				ct.ShareOfAttributed = float64(ct.AttributedBytes) / float64(r.Tmpfs.AttributedBytes)
			}
			if r.Tmpfs.TotalBytes > 0 {
				ct.ShareOfSystem = float64(ct.AttributedBytes) / float64(r.Tmpfs.TotalBytes)
			}
		}
	}

	r.Population.RawItems = len(r.Items)
	r.Population.Protected = r.Population.RawItems - r.Population.Cleanable
	return r, nil
}

// itemCount returns the number of entities represented by one row.
func itemCount(it model.SwapItem) int {
	if it.Count > 0 {
		return it.Count
	}
	return 1
}

func attribution(total, attributed int64, reasons []ReasonBytes, residualReason string) Attribution {
	a := Attribution{
		TotalBytes:      total,
		AttributedBytes: attributed,
	}
	if attributed > total {
		a.OverlapBytes = attributed - total
	} else {
		a.UnattributedBytes = total - attributed
	}

	out := append([]ReasonBytes(nil), reasons...)
	var reasonSum int64
	for _, rr := range out {
		reasonSum += rr.Bytes
	}
	if a.UnattributedBytes > reasonSum {
		out = append(out, ReasonBytes{
			Reason: residualReason,
			Bytes:  a.UnattributedBytes - reasonSum,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return out[i].Reason < out[j].Reason
	})
	a.Reasons = out
	return a
}

// Reconciles returns an error if any accounting invariant fails.
func (r AuditReport) Reconciles() error {
	if err := checkAttribution("swap", r.Swap); err != nil {
		return err
	}
	if err := checkAttribution("/tmp", r.Tmpfs); err != nil {
		return err
	}
	if r.Reclaim.TmpfsBytes < 0 || r.Reclaim.SwapBytes < 0 {
		return fmt.Errorf("reclaim bytes must not be negative")
	}
	for _, c := range r.Categories {
		if c.AttributedBytes < 0 || c.ReclaimableBytes < 0 {
			return fmt.Errorf("category %s has negative bytes", c.Category)
		}
		if c.ShareOfAttributed < 0 || c.ShareOfAttributed > 1.0000001 {
			return fmt.Errorf("category %s share_of_attributed out of range: %f", c.Category, c.ShareOfAttributed)
		}
		if c.ShareOfSystem < 0 {
			return fmt.Errorf("category %s share_of_system out of range: %f", c.Category, c.ShareOfSystem)
		}
	}
	return nil
}

func checkAttribution(name string, a Attribution) error {
	if a.TotalBytes < 0 || a.AttributedBytes < 0 || a.UnattributedBytes < 0 || a.OverlapBytes < 0 {
		return fmt.Errorf("%s attribution has a negative value: %+v", name, a)
	}
	if a.OverlapBytes == 0 {
		if a.AttributedBytes+a.UnattributedBytes != a.TotalBytes {
			return fmt.Errorf("%s does not reconcile: attributed %d + unattributed %d != total %d",
				name, a.AttributedBytes, a.UnattributedBytes, a.TotalBytes)
		}
	} else {
		if a.UnattributedBytes != 0 {
			return fmt.Errorf("%s overlap and unattributed are mutually exclusive", name)
		}
		if a.AttributedBytes-a.OverlapBytes != a.TotalBytes {
			return fmt.Errorf("%s overlap does not reconcile: %d - %d != %d",
				name, a.AttributedBytes, a.OverlapBytes, a.TotalBytes)
		}
	}
	return nil
}

// BiggestWin returns the largest cleanable item with a deterministic tie-break.
func (r AuditReport) BiggestWin() (model.SwapItem, bool) {
	var best model.SwapItem
	found := false
	for _, it := range r.Items {
		if !it.Cleanable {
			continue
		}
		if !found || model.ItemSize(it) > model.ItemSize(best) ||
			(model.ItemSize(it) == model.ItemSize(best) && tieBreak(it, best)) {
			best = it
			found = true
		}
	}
	return best, found
}

func tieBreak(a, b model.SwapItem) bool {
	if a.PID != b.PID {
		return a.PID < b.PID
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.Path < b.Path
}

// CleanableCount returns the number of cleanable rows.
func (r AuditReport) CleanableCount() int {
	n := 0
	for _, it := range r.Items {
		if it.Cleanable {
			n++
		}
	}
	return n
}

// SwapPercent returns the system swap usage percentage.
func (r AuditReport) SwapPercent() float64 {
	if r.System.SwapTotal <= 0 {
		return 0
	}
	return float64(r.System.SwapUsed) / float64(r.System.SwapTotal) * 100.0
}

// Category returns the category total for a category.
func (r AuditReport) Category(c model.Category) CategoryTotal {
	for _, ct := range r.Categories {
		if ct.Category == c {
			return ct
		}
	}
	return CategoryTotal{Category: c, Pool: poolFor(c)}
}

// PoolForCategory maps a category to its pool.
func PoolForCategory(c model.Category) Pool {
	return poolFor(c)
}

func poolFor(c model.Category) Pool {
	switch c {
	case model.CatDormantAgent, model.CatAbandonedAgent, model.CatDesktopDaemon, model.CatActiveProcess:
		return PoolProcessSwap
	default:
		return PoolTmpfs
	}
}

// FormatReasons renders a deterministic reason breakdown, omitting zero-byte reasons.
func FormatReasons(reasons []ReasonBytes) string {
	parts := make([]string, 0, len(reasons))
	for _, rr := range reasons {
		if rr.Bytes == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s", rr.Reason, formatBytes(rr.Bytes)))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " · ")
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
