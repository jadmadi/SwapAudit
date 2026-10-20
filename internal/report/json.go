package report

import (
	"encoding/json"
	"io"
	"sort"
	"time"

	"github.com/jadmadi/swapaudit/internal/cleaner"
	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/triage"
)

// SchemaVersion is the machine-readable contract version.
const SchemaVersion = 1

// JSONReport is the versioned top-level object written by --json.
type JSONReport struct {
	SchemaVersion int                `json:"schema_version"`
	GeneratedAt   string             `json:"generated_at"`
	Host          JSONHost           `json:"host"`
	Thresholds    JSONThresholds     `json:"thresholds"`
	System        JSONSystem         `json:"system"`
	Attribution   JSONAttributionSet `json:"attribution"`
	Reclaim       JSONReclaim        `json:"reclaim"`
	Population    JSONPopulation     `json:"population"`
	Scan          JSONScan           `json:"scan"`
	Categories    []JSONCategory     `json:"categories"`
	Groups        []JSONGroup        `json:"groups"`
	Items         []JSONItem         `json:"items"`
	Cleanup       *JSONCleanup       `json:"cleanup"`
	Errors        []JSONError        `json:"errors,omitempty"`
}

// JSONHost identifies the host.
type JSONHost struct {
	KernelRelease string `json:"kernel_release"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// JSONThresholds records thresholds in integer seconds.
type JSONThresholds struct {
	MaxAgeSeconds         int64 `json:"max_age_seconds"`
	IdleThresholdSeconds  int64 `json:"idle_threshold_seconds"`
	OrphanCacheAgeSeconds int64 `json:"orphan_cache_age_seconds"`
}

// JSONSystem is the kernel header data.
type JSONSystem struct {
	Swap  JSONSwap  `json:"swap"`
	RAM   JSONRAM   `json:"ram"`
	Tmpfs JSONTmpfs `json:"tmpfs"`
}

type JSONSwap struct {
	TotalBytes int64 `json:"total_bytes"`
	UsedBytes  int64 `json:"used_bytes"`
	FreeBytes  int64 `json:"free_bytes"`
}

type JSONRAM struct {
	TotalBytes     int64  `json:"total_bytes"`
	UsedBytes      int64  `json:"used_bytes"`
	AvailableBytes int64  `json:"available_bytes"`
	Definition     string `json:"definition"`
}

type JSONTmpfs struct {
	Path       string `json:"path"`
	TotalBytes int64  `json:"total_bytes"`
	UsedBytes  int64  `json:"used_bytes"`
	FreeBytes  int64  `json:"free_bytes"`
	AvailBytes int64  `json:"avail_bytes"`
}

type JSONAttributionSet struct {
	Swap  JSONAttribution `json:"swap"`
	Tmpfs JSONAttribution `json:"tmpfs"`
}

type JSONAttribution struct {
	TotalBytes        int64        `json:"total_bytes"`
	AttributedBytes   int64        `json:"attributed_bytes"`
	UnattributedBytes int64        `json:"unattributed_bytes"`
	OverlapBytes      int64        `json:"overlap_bytes"`
	Reasons           []JSONReason `json:"reasons"`
}

type JSONReason struct {
	Reason string `json:"reason"`
	Bytes  int64  `json:"bytes"`
	Count  int    `json:"count"`
}

type JSONReclaim struct {
	TmpfsBytes   int64  `json:"tmpfs_bytes"`
	SwapBytes    int64  `json:"swap_bytes"`
	UpperBound   bool   `json:"upper_bound"`
	Note         string `json:"note"`
	CacheCount   int    `json:"cache_count"`
	ProcessCount int    `json:"process_count"`
}

// JSONPopulation is the full population accounting.
type JSONPopulation struct {
	RawItems           int `json:"raw_items"`
	AggregatedShells   int `json:"aggregated_shells"`
	AggregatedRows     int `json:"aggregated_rows"`
	Displayed          int `json:"displayed"`
	HiddenByLimit      int `json:"hidden_by_limit"`
	HiddenBySizeFilter int `json:"hidden_by_size_filter"`
	Cleanable          int `json:"cleanable"`
	Protected          int `json:"protected"`
	Processes          int `json:"processes"`
	ProcessesWithSwap  int `json:"processes_with_swap"`
	Agents             int `json:"agents"`
	AgentsWithSwap     int `json:"agents_with_swap"`
	AgentsZeroSwap     int `json:"agents_zero_swap"`
	AgentsDormant      int `json:"agents_dormant"`
	AgentsOrphaned     int `json:"agents_orphaned"`
	AgentsParentAlive  int `json:"agents_parent_alive"`
	AgentsIdleUnknown  int `json:"agents_idle_unknown"`
	AgentsActive       int `json:"agents_active"`
	Vanished           int `json:"vanished"`
	PermissionDenied   int `json:"permission_denied"`
}

// JSONScan records scanner metadata and liveness evidence.
type JSONScan struct {
	ScannedCaches      int  `json:"scanned_caches"`
	InUseCaches        int  `json:"in_use_caches"`
	OrphanedCaches     int  `json:"orphaned_caches"`
	RecentCaches       int  `json:"recent_caches"`
	EvidenceIncomplete int  `json:"evidence_incomplete"`
	OpenIndexTruncated bool `json:"open_index_truncated"`
	OpenIndexErrors    int  `json:"open_index_errors"`
	OpenIndexPIDs      int  `json:"open_index_pids"`
	OpenIndexFDs       int  `json:"open_index_fds"`
	FallbackCount      int  `json:"fallback_count"`
}

// JSONCategory is one category row.
type JSONCategory struct {
	Category          string  `json:"category"`
	Pool              string  `json:"pool"`
	Entities          int     `json:"entities"`
	AttributedBytes   int64   `json:"attributed_bytes"`
	ReclaimableBytes  int64   `json:"reclaimable_bytes"`
	ShareOfAttributed float64 `json:"share_of_attributed"`
	ShareOfSystem     float64 `json:"share_of_system"`
}

// JSONGroup is one collapsed pattern group.
type JSONGroup struct {
	Key            string          `json:"key"`
	Category       string          `json:"category"`
	CacheKind      string          `json:"cache_kind,omitempty"`
	Count          int             `json:"count"`
	TotalBytes     int64           `json:"total_bytes"`
	MinIdleSeconds int64           `json:"min_idle_seconds"`
	MaxIdleSeconds int64           `json:"max_idle_seconds"`
	Cleanable      bool            `json:"cleanable"`
	Representative JSONGroupMember `json:"representative"`
}

type JSONGroupMember struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// JSONItem is one full-identity item.
type JSONItem struct {
	Kind           string `json:"kind"`
	ID             string `json:"id"`
	PID            int    `json:"pid"`
	StartTicks     int64  `json:"start_ticks"`
	UID            uint32 `json:"uid"`
	Name           string `json:"name"`
	Path           string `json:"path"`
	Category       string `json:"category"`
	CacheKind      string `json:"cache_kind,omitempty"`
	Cleanable      bool   `json:"cleanable"`
	Agent          bool   `json:"agent"`
	SwapBytes      int64  `json:"swap_bytes"`
	AllocatedBytes int64  `json:"allocated_bytes"`
	ApparentBytes  int64  `json:"apparent_bytes"`
	RSSBytes       int64  `json:"rss_bytes"`
	AgeSeconds     int64  `json:"age_seconds"`
	IdleSeconds    *int64 `json:"idle_seconds"`
	IdleKnown      bool   `json:"idle_known"`
	OpenHandles    int    `json:"open_handles"`
	CwdInside      bool   `json:"cwd_inside"`
	MappedInside   bool   `json:"mapped_inside"`
	ParentPID      int    `json:"parent_pid"`
	ParentComm     string `json:"parent_comm"`
	ParentAlive    bool   `json:"parent_alive"`
	OrphanReason   string `json:"orphan_reason,omitempty"`
	TTYNr          int64  `json:"tty_nr"`
	Cgroup         string `json:"cgroup,omitempty"`
	Evidence       string `json:"evidence,omitempty"`
	Signal         string `json:"signal,omitempty"`
	GroupKey       string `json:"group_key,omitempty"`
	GroupCount     int    `json:"group_count,omitempty"`
}

// JSONCleanup mirrors cleaner.CleanupResult.
type JSONCleanup struct {
	DryRun  bool               `json:"dry_run"`
	Groups  []JSONCleanupGroup `json:"groups"`
	Summary JSONCleanupSummary `json:"summary"`
}

type JSONCleanupGroup struct {
	Group      string              `json:"group"`
	Targets    int                 `json:"targets"`
	Bytes      int64               `json:"bytes"`
	WouldPrune int                 `json:"would_prune"`
	WouldEnd   int                 `json:"would_end"`
	Pruned     int                 `json:"pruned"`
	Ended      int                 `json:"ended"`
	Skipped    int                 `json:"skipped"`
	Reasons    []JSONCleanupReason `json:"reasons"`
}

type JSONCleanupReason struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type JSONCleanupSummary struct {
	Pruned  int                 `json:"pruned"`
	Ended   int                 `json:"ended"`
	Skipped int                 `json:"skipped"`
	Reasons []JSONCleanupReason `json:"reasons"`
}

// JSONError is a documented runtime error entry.
type JSONError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Target  string `json:"target,omitempty"`
}

// ToJSON projects an AuditReport into the stable machine schema.
func ToJSON(r AuditReport) JSONReport {
	jr := JSONReport{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   r.GeneratedAt.UTC().Format(time.RFC3339),
		Host:          JSONHost{KernelRelease: r.Host.KernelRelease, UptimeSeconds: r.Host.UptimeSeconds},
		Thresholds: JSONThresholds{
			MaxAgeSeconds:         int64(r.Thresholds.MaxAge / time.Second),
			IdleThresholdSeconds:  int64(r.Thresholds.IdleThreshold / time.Second),
			OrphanCacheAgeSeconds: int64(r.Thresholds.OrphanCacheAge / time.Second),
		},
		System: JSONSystem{
			Swap: JSONSwap{TotalBytes: r.System.SwapTotal, UsedBytes: r.System.SwapUsed, FreeBytes: r.System.SwapFree},
			RAM: JSONRAM{
				TotalBytes: r.System.MemTotal, UsedBytes: r.System.MemUsed,
				AvailableBytes: r.System.MemAvail, Definition: "used = total - available",
			},
			Tmpfs: JSONTmpfs{
				Path: "/tmp", TotalBytes: r.System.TmpfsSize, UsedBytes: r.System.TmpfsUsed,
				FreeBytes: r.System.TmpfsFree, AvailBytes: r.System.TmpfsAvail,
			},
		},
		Attribution: JSONAttributionSet{
			Swap:  jsonAttribution(r.Swap),
			Tmpfs: jsonAttribution(r.Tmpfs),
		},
		Reclaim: JSONReclaim{
			TmpfsBytes: r.Reclaim.TmpfsBytes, SwapBytes: r.Reclaim.SwapBytes,
			UpperBound: true, Note: "tmpfs and swap pools may overlap; tmpfs pages may be swapped",
			CacheCount: r.Reclaim.CacheCount, ProcessCount: r.Reclaim.ProcessCount,
		},
		Population: JSONPopulation{
			RawItems: r.Population.RawItems, AggregatedShells: r.Population.AggregatedShells,
			AggregatedRows: r.Population.AggregatedRows, Displayed: r.Population.Displayed,
			HiddenByLimit: r.Population.HiddenByLimit, HiddenBySizeFilter: r.Population.HiddenBySizeFilter,
			Cleanable: r.Population.Cleanable, Protected: r.Population.Protected,
			Processes: r.Population.Processes, ProcessesWithSwap: r.Population.ProcessesWithSwap,
			Agents: r.Population.Agents, AgentsWithSwap: r.Population.AgentsWithSwap,
			AgentsZeroSwap: r.Population.AgentsZeroSwap,
			AgentsDormant:  r.Population.AgentsDormant, AgentsOrphaned: r.Population.AgentsOrphaned,
			AgentsParentAlive: r.Population.AgentsParentAlive, AgentsIdleUnknown: r.Population.AgentsIdleUnknown,
			AgentsActive: r.Population.AgentsActive, Vanished: r.Population.Vanished,
			PermissionDenied: r.Population.PermissionDenied,
		},
		Categories: jsonCategories(r.Categories),
		Groups:     jsonGroups(r.Items),
		Items:      jsonItems(r.Items, r.Sort),
		Scan: JSONScan{
			ScannedCaches: r.Scan.ScannedCaches, InUseCaches: r.Scan.InUseCaches,
			OrphanedCaches: r.Scan.OrphanedCaches, RecentCaches: r.Scan.RecentCaches,
			EvidenceIncomplete: r.Scan.EvidenceIncomplete,
			OpenIndexTruncated: r.Scan.OpenIndexTruncated, OpenIndexErrors: r.Scan.OpenIndexErrors,
			OpenIndexPIDs: r.Scan.OpenIndexPIDs, OpenIndexFDs: r.Scan.OpenIndexFDs,
			FallbackCount: r.Scan.FallbackCount,
		},
	}
	return jr
}

// WriteJSON writes exactly one JSON object plus a newline.
func WriteJSON(w io.Writer, r AuditReport, cleanup *cleaner.CleanupResult) error {
	jr := ToJSON(r)
	jr.Cleanup = jsonCleanup(cleanup)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(jr)
}

func jsonAttribution(a Attribution) JSONAttribution {
	out := JSONAttribution{
		TotalBytes: a.TotalBytes, AttributedBytes: a.AttributedBytes,
		UnattributedBytes: a.UnattributedBytes, OverlapBytes: a.OverlapBytes,
		Reasons: make([]JSONReason, 0, len(a.Reasons)),
	}
	for _, rr := range a.Reasons {
		out.Reasons = append(out.Reasons, JSONReason{Reason: rr.Reason, Bytes: rr.Bytes, Count: rr.Count})
	}
	sort.SliceStable(out.Reasons, func(i, j int) bool {
		if out.Reasons[i].Bytes != out.Reasons[j].Bytes {
			return out.Reasons[i].Bytes > out.Reasons[j].Bytes
		}
		return out.Reasons[i].Reason < out.Reasons[j].Reason
	})
	return out
}

func jsonCategories(cats []CategoryTotal) []JSONCategory {
	out := make([]JSONCategory, 0, len(cats))
	for _, ct := range cats {
		out = append(out, JSONCategory{
			Category: string(ct.Category), Pool: string(ct.Pool), Entities: ct.Entities,
			AttributedBytes: ct.AttributedBytes, ReclaimableBytes: ct.ReclaimableBytes,
			ShareOfAttributed: ct.ShareOfAttributed, ShareOfSystem: ct.ShareOfSystem,
		})
	}
	return out
}

func jsonGroups(items []model.SwapItem) []JSONGroup {
	groups := triage.GroupItems(items, true)
	out := make([]JSONGroup, 0, len(groups))
	for _, g := range groups {
		if g.Count < 2 {
			continue
		}
		out = append(out, JSONGroup{
			Key: g.Key, Category: string(g.Category), CacheKind: string(g.CacheKind),
			Count: g.Count, TotalBytes: g.TotalBytes,
			MinIdleSeconds: int64(maxDuration(g.MinIdle) / time.Second),
			MaxIdleSeconds: int64(maxDuration(g.MaxIdle) / time.Second),
			Cleanable:      g.Cleanable,
			Representative: JSONGroupMember{Name: g.Representative.Name, Path: g.Representative.Path},
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TotalBytes != out[j].TotalBytes {
			return out[i].TotalBytes > out[j].TotalBytes
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func maxDuration(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}

func jsonItems(items []model.SwapItem, mode SortMode) []JSONItem {
	sorted := append([]model.SwapItem(nil), items...)
	if mode == SortAge {
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Age != sorted[j].Age {
				return sorted[i].Age > sorted[j].Age
			}
			return itemIdentity(sorted[i]) < itemIdentity(sorted[j])
		})
	} else {
		sort.SliceStable(sorted, func(i, j int) bool {
			if model.ItemSize(sorted[i]) != model.ItemSize(sorted[j]) {
				return model.ItemSize(sorted[i]) > model.ItemSize(sorted[j])
			}
			return itemIdentity(sorted[i]) < itemIdentity(sorted[j])
		})
	}
	out := make([]JSONItem, 0, len(sorted))
	for _, it := range sorted {
		var idle *int64
		if it.LastActiveKnown {
			v := int64(it.LastActive / time.Second)
			idle = &v
		}
		out = append(out, JSONItem{
			Kind: string(it.Kind), ID: it.ID, PID: it.PID, StartTicks: it.StartTicks, UID: it.UID,
			Name: it.Name, Path: it.Path, Category: string(it.Category), CacheKind: string(it.CacheKind),
			Cleanable: it.Cleanable, Agent: it.Agent, SwapBytes: it.SwapBytes,
			AllocatedBytes: it.AllocatedBytes, ApparentBytes: it.ApparentBytes, RSSBytes: it.RSS,
			AgeSeconds: int64(maxDuration(it.Age) / time.Second), IdleSeconds: idle, IdleKnown: it.LastActiveKnown,
			OpenHandles: it.OpenHandles, CwdInside: it.CwdInside, MappedInside: it.MappedInside,
			ParentPID: it.ParentPID, ParentComm: it.ParentComm, ParentAlive: it.ParentAlive,
			OrphanReason: it.OrphanReason, TTYNr: it.TTYNr, Cgroup: it.Cgroup,
			Evidence: it.Evidence, Signal: it.Signal, GroupKey: it.GroupKey, GroupCount: it.GroupCount,
		})
	}
	return out
}

func itemIdentity(it model.SwapItem) string {
	if it.Path != "" {
		return it.Path
	}
	return it.ID
}

func jsonCleanup(c *cleaner.CleanupResult) *JSONCleanup {
	if c == nil {
		return nil
	}
	out := &JSONCleanup{DryRun: c.DryRun, Groups: make([]JSONCleanupGroup, 0, len(c.Groups))}
	for _, g := range c.Groups {
		jg := JSONCleanupGroup{
			Group: g.Group, Targets: g.Targets, Bytes: g.Bytes,
			WouldPrune: g.WouldPrune, WouldEnd: g.WouldEnd,
			Pruned: g.Pruned, Ended: g.Ended, Skipped: g.Skipped,
			Reasons: make([]JSONCleanupReason, 0, len(g.Reasons)),
		}
		for _, rc := range g.Reasons {
			jg.Reasons = append(jg.Reasons, JSONCleanupReason{Reason: rc.Reason, Count: rc.Count})
		}
		out.Groups = append(out.Groups, jg)
	}
	out.Summary = JSONCleanupSummary{
		Pruned: c.Summary.Pruned, Ended: c.Summary.Ended, Skipped: c.Summary.Skipped,
		Reasons: make([]JSONCleanupReason, 0, len(c.Summary.Reasons)),
	}
	for _, rc := range c.Summary.SortedReasons() {
		out.Summary.Reasons = append(out.Summary.Reasons, JSONCleanupReason{Reason: rc.Reason, Count: rc.Count})
	}
	return out
}
