//go:build linux

// Package snapshot defines the versioned machine-readable swapaudit document.
// It is deliberately neutral so internal/app and internal/live can share it
// without an import cycle.
package snapshot

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/version"
)

// SchemaVersion is the version of the JSON document contract.
const SchemaVersion = 1

// Snapshot is the top-level schema_version 1 document.
type Snapshot struct {
	SchemaVersion int            `json:"schema_version"`
	Tool          Tool           `json:"tool"`
	GeneratedAt   string         `json:"generated_at"`
	UptimeSeconds float64        `json:"uptime_seconds"`
	Thresholds    Thresholds     `json:"thresholds"`
	System        System         `json:"system"`
	Totals        Totals         `json:"totals"`
	Summary       []SummaryEntry `json:"summary"`
	Items         []Item         `json:"items"`
	Warnings      []string       `json:"warnings"`
}

// Tool carries build metadata without formatting prose.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Built   string `json:"built"`
}

// Thresholds records the scan cutoffs in whole seconds.
type Thresholds struct {
	MaxAgeSeconds        int64 `json:"max_age_seconds"`
	IdleThresholdSeconds int64 `json:"idle_threshold_seconds"`
}

// System carries the raw kernel counters in bytes.
type System struct {
	SwapTotalBytes    int64 `json:"swap_total_bytes"`
	SwapUsedBytes     int64 `json:"swap_used_bytes"`
	SwapFreeBytes     int64 `json:"swap_free_bytes"`
	MemTotalBytes     int64 `json:"mem_total_bytes"`
	MemUsedBytes      int64 `json:"mem_used_bytes"`
	MemAvailableBytes int64 `json:"mem_available_bytes"`
	TmpfsTotalBytes   int64 `json:"tmpfs_total_bytes"`
	TmpfsUsedBytes    int64 `json:"tmpfs_used_bytes"`
}

// Totals aggregates the raw scan without truncation.
type Totals struct {
	Items                     int   `json:"items"`
	ProcessSwapBytes          int64 `json:"process_swap_bytes"`
	TmpfsCacheBytes           int64 `json:"tmpfs_cache_bytes"`
	CleanableBytes            int64 `json:"cleanable_bytes"`
	CleanableProcessSwapBytes int64 `json:"cleanable_process_swap_bytes"`
	CleanableTmpfsBytes       int64 `json:"cleanable_tmpfs_bytes"`
}

// SummaryEntry aggregates one category.
type SummaryEntry struct {
	Category           string `json:"category"`
	Items              int    `json:"items"`
	SwapBytes          int64  `json:"swap_bytes"`
	SizeBytes          int64  `json:"size_bytes"`
	CleanableItems     int    `json:"cleanable_items"`
	CleanableSwapBytes int64  `json:"cleanable_swap_bytes"`
	CleanableSizeBytes int64  `json:"cleanable_size_bytes"`
}

// Item is one raw process or filesystem cache. Nullable fields use pointers so
// unknown is never overloaded onto 0, "", or -1.
type Item struct {
	PID                *int    `json:"pid"`
	StartTicks         *int64  `json:"start_ticks"`
	Name               string  `json:"name"`
	Command            string  `json:"command"`
	Category           string  `json:"category"`
	SwapBytes          int64   `json:"swap_bytes"`
	SizeBytes          int64   `json:"size_bytes"`
	EffectiveSizeBytes int64   `json:"effective_size_bytes"`
	AgeSeconds         *int64  `json:"age_seconds"`
	IdleSeconds        *int64  `json:"idle_seconds"`
	LastActiveAt       *string `json:"last_active_at"`
	Path               *string `json:"path"`
	Cleanable          bool    `json:"cleanable"`
}

// ErrorDocument is the only document emitted on a hard failure.
type ErrorDocument struct {
	SchemaVersion int         `json:"schema_version"`
	Error         ErrorDetail `json:"error"`
}

// ErrorDetail identifies the hard failure.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// BuildSnapshot converts a completed scan into the canonical document. It is
// pure: the caller supplies the clock and every input.
func BuildSnapshot(now time.Time, uptimeSeconds float64, sys model.SystemSwap, items []model.SwapItem, maxAge, idleThreshold time.Duration, noCommand bool) Snapshot {
	s := Snapshot{
		SchemaVersion: SchemaVersion,
		Tool: Tool{
			Name:    "swapaudit",
			Version: version.Version,
			Commit:  version.Commit,
			Built:   version.Date,
		},
		GeneratedAt:   now.Format(time.RFC3339),
		UptimeSeconds: uptimeSeconds,
		Thresholds: Thresholds{
			MaxAgeSeconds:        int64(maxAge / time.Second),
			IdleThresholdSeconds: int64(idleThreshold / time.Second),
		},
		System:   system(sys),
		Summary:  []SummaryEntry{},
		Items:    make([]Item, 0, len(items)),
		Warnings: []string{},
	}

	summaries := make(map[string]*SummaryEntry)
	for _, it := range items {
		entry := toItem(it, noCommand)
		s.Items = append(s.Items, entry)

		key := string(it.Category)
		sum, ok := summaries[key]
		if !ok {
			sum = &SummaryEntry{Category: key}
			summaries[key] = sum
		}
		count := it.Count
		if count == 0 {
			count = 1
		}
		sum.Items += count
		sum.SwapBytes += it.SwapBytes
		sum.SizeBytes += it.SizeBytes
		if it.Cleanable {
			sum.CleanableItems += count
			sum.CleanableSwapBytes += it.SwapBytes
			sum.CleanableSizeBytes += it.SizeBytes
			s.Totals.CleanableBytes += model.ItemSize(it)
			s.Totals.CleanableProcessSwapBytes += it.SwapBytes
			s.Totals.CleanableTmpfsBytes += it.SizeBytes
		}
		s.Totals.ProcessSwapBytes += it.SwapBytes
		s.Totals.TmpfsCacheBytes += it.SizeBytes
	}

	s.Totals.Items = len(s.Items)

	for _, sum := range summaries {
		s.Summary = append(s.Summary, *sum)
	}
	sort.Slice(s.Summary, func(i, j int) bool {
		return s.Summary[i].Category < s.Summary[j].Category
	})

	sort.SliceStable(s.Items, func(i, j int) bool {
		a, b := s.Items[i], s.Items[j]
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if a.EffectiveSizeBytes != b.EffectiveSizeBytes {
			return a.EffectiveSizeBytes > b.EffectiveSizeBytes
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.PID == nil {
			return false
		}
		if b.PID == nil {
			return true
		}
		return *a.PID < *b.PID
	})

	return s
}

func toItem(it model.SwapItem, noCommand bool) Item {
	out := Item{
		Name:               it.Name,
		Command:            it.Command,
		Category:           string(it.Category),
		SwapBytes:          it.SwapBytes,
		SizeBytes:          it.SizeBytes,
		EffectiveSizeBytes: model.ItemSize(it),
		Cleanable:          it.Cleanable,
	}
	if noCommand {
		out.Command = ""
	}

	if it.PID > 0 {
		pid := it.PID
		start := it.StartTicks
		out.PID = &pid
		out.StartTicks = &start
		if it.StartTicks > 0 {
			age := int64(it.Age / time.Second)
			out.AgeSeconds = &age
		}
	} else if it.Path != "" {
		path := it.Path
		out.Path = &path
		if !it.LastActiveAt.IsZero() {
			age := int64(it.Age / time.Second)
			out.AgeSeconds = &age
		}
	}

	if it.LastActive >= 0 && !it.LastActiveAt.IsZero() {
		idle := int64(it.LastActive / time.Second)
		lastActive := it.LastActiveAt.Format(time.RFC3339)
		out.IdleSeconds = &idle
		out.LastActiveAt = &lastActive
	}

	return out
}

func system(sys model.SystemSwap) System {
	return System{
		SwapTotalBytes:    nonNegative(sys.SwapTotal),
		SwapUsedBytes:     nonNegative(sys.SwapUsed),
		SwapFreeBytes:     nonNegative(sys.SwapFree),
		MemTotalBytes:     nonNegative(sys.MemTotal),
		MemUsedBytes:      nonNegative(sys.MemUsed),
		MemAvailableBytes: nonNegative(sys.MemAvail),
		TmpfsTotalBytes:   nonNegative(sys.TmpfsSize),
		TmpfsUsedBytes:    nonNegative(sys.TmpfsUsed),
	}
}

func nonNegative(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

// Write marshals the document to a buffer before writing so an encode failure
// cannot emit a partial document.
func Write(w io.Writer, s Snapshot) error {
	return writeJSON(w, s)
}

// WriteError marshals the hard-failure document.
func WriteError(w io.Writer, code, message string) error {
	return writeJSON(w, ErrorDocument{
		SchemaVersion: SchemaVersion,
		Error:         ErrorDetail{Code: code, Message: message},
	})
}

func writeJSON(w io.Writer, doc any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}
