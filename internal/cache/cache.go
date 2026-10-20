//go:build linux

package cache

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

// DefaultTmpPatterns lists common prefixes/wildcards for test and temporary run directories in /tmp.
var DefaultTmpPatterns = []string{
	"go-build*", "Test*", "test*", "test-bwrap*", "tsx-*",
	"tmp-*", "temp-*", "rustup-update*", "cargo-install*", "npm-*",
}

// SkipReason records excluded bytes and entities for one scan reason.
type SkipReason struct {
	Reason string
	Bytes  int64
	Count  int
}

// ScanStats records what the /tmp scan saw and excluded.
type ScanStats struct {
	Skipped            []SkipReason
	FallbackCount      int
	Scanned            int
	InUse              int
	Orphaned           int
	Recent             int
	EvidenceIncomplete int
	OpenIndexTruncated bool
	OpenIndexErrors    int
	OpenIndexPIDs      int
	OpenIndexFDs       int
}

// AddSkip increments a skip reason.
func (s *ScanStats) AddSkip(reason string, bytes int64) {
	for i := range s.Skipped {
		if s.Skipped[i].Reason == reason {
			s.Skipped[i].Bytes += bytes
			s.Skipped[i].Count++
			return
		}
	}
	s.Skipped = append(s.Skipped, SkipReason{Reason: reason, Bytes: bytes, Count: 1})
}

// SortedSkips returns skip reasons sorted by bytes descending then reason.
func (s ScanStats) SortedSkips() []SkipReason {
	out := append([]SkipReason(nil), s.Skipped...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

// ScanOptions controls a /tmp scan.
type ScanOptions struct {
	Now                 time.Time
	MaxAge              time.Duration
	OrphanCacheAge      time.Duration
	Patterns            []string
	OpenIndex           *OpenIndex
	AllowGenericCaches  bool
	CleanOrphanedCaches bool
}

var knownDaemonPrefixes = []string{
	"tmux-", "codex-daemon-", "MegaLimited", "plasma-",
	"wayland-", "dbus-", "ssh-", "gnupg-", "atuin-",
}

// MatchTmpPattern checks if a directory name matches a shell pattern or prefix.
func MatchTmpPattern(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	if err == nil && matched {
		return true
	}
	cleanPat := strings.TrimSuffix(pattern, "*")
	if cleanPat != "" && (strings.HasPrefix(name, cleanPat) || strings.EqualFold(name, cleanPat)) {
		return true
	}
	return false
}

// ScanTmpDirectories scans /tmp for stale caches, abandoned runs, and temporary build outputs.
func ScanTmpDirectories(now time.Time, maxAge time.Duration, patterns []string) []model.SwapItem {
	return ScanTmpDirectoriesIn("/tmp", now, maxAge, patterns)
}

// ScanTmpDirectoriesIn scans a specified base directory for stale caches.
// It is retained as a compatibility wrapper around ScanTmpDirectoriesDetailed.
func ScanTmpDirectoriesIn(baseDir string, now time.Time, maxAge time.Duration, patterns []string) []model.SwapItem {
	items, _ := ScanTmpDirectoriesDetailed(baseDir, ScanOptions{
		Now:            now,
		MaxAge:         maxAge,
		OrphanCacheAge: 0,
		Patterns:       patterns,
	})
	return items
}

// ScanTmpDirectoriesDetailed scans baseDir and returns items plus exclusion statistics.
func ScanTmpDirectoriesDetailed(baseDir string, opts ScanOptions) ([]model.SwapItem, ScanStats) {
	var items []model.SwapItem
	var stats ScanStats
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	patterns := opts.Patterns
	if len(patterns) == 0 {
		patterns = DefaultTmpPatterns
	}
	if opts.OpenIndex != nil {
		stats.OpenIndexTruncated = opts.OpenIndex.Truncated
		stats.OpenIndexErrors = opts.OpenIndex.Errors
		stats.OpenIndexPIDs = opts.OpenIndex.ScannedPIDs
		stats.OpenIndexFDs = opts.OpenIndex.ScannedFDs
	}

	currentUID := uint32(os.Getuid())
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		stats.AddSkip("inaccessible", 0)
		return items, stats
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(baseDir, name)

		fi, err := os.Lstat(fullPath)
		if err != nil {
			stats.AddSkip("inaccessible", 0)
			continue
		}
		allocated, apparent, fallback := measureTree(fullPath, fi)
		if fallback {
			stats.FallbackCount++
		}

		if strings.HasPrefix(name, ".") {
			stats.AddSkip("dotfile", allocated)
			continue
		}
		if strings.HasPrefix(name, "systemd-") || strings.HasPrefix(name, "pulse-") {
			stats.AddSkip("systemd-or-pulse", allocated)
			continue
		}
		isDaemon := false
		for _, p := range knownDaemonPrefixes {
			if strings.HasPrefix(name, p) {
				isDaemon = true
				break
			}
		}
		if isDaemon {
			stats.AddSkip("daemon-prefix", allocated)
			continue
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			stats.AddSkip("symlink", allocated)
			continue
		}
		if fi.Mode()&os.ModeSocket != 0 || fi.Mode()&os.ModeNamedPipe != 0 || fi.Mode()&os.ModeDevice != 0 {
			stats.AddSkip("special-file", allocated)
			continue
		}
		stat, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != currentUID {
			stats.AddSkip("other-uid", allocated)
			continue
		}

		lower := strings.ToLower(name)
		isBuildCache := strings.HasPrefix(name, "go-build") || strings.Contains(lower, "cache")
		patternMatch := false
		for _, pat := range patterns {
			if MatchTmpPattern(pat, name) {
				patternMatch = true
				break
			}
		}
		keywordMatch := false
		for _, k := range []string{"test", "build", "review", "candidate", "probe", "profile", "repro"} {
			if strings.Contains(lower, k) {
				keywordMatch = true
				break
			}
		}
		if !isBuildCache && !patternMatch && !keywordMatch {
			stats.AddSkip("non-matching", allocated)
			continue
		}
		kind := ClassifyCacheKind(name, patterns)

		meta := CacheMeta{
			Name:           name,
			Kind:           kind,
			RootMtime:      fi.ModTime(),
			NewestMtime:    newestMtime(fullPath, fi),
			AllocatedBytes: allocated,
		}
		if opts.OpenIndex != nil {
			handles, cwds, mapped := opts.OpenIndex.Lookup(fullPath)
			meta.OpenHandles = handles
			meta.CwdInside = cwds > 0
			meta.MappedInside = mapped > 0
			meta.EvidenceIncomplete = !opts.OpenIndex.Complete()
		}

		cat, cleanable, evidence := ClassifyCache(meta, opts.Now, Thresholds{
			MaxAge:         opts.MaxAge,
			OrphanCacheAge: opts.OrphanCacheAge,
		}, Flags{
			AllowGenericCaches:  opts.AllowGenericCaches,
			CleanOrphanedCaches: opts.CleanOrphanedCaches,
		})

		stats.Scanned++
		switch cat {
		case model.CatInUseCache:
			stats.InUse++
		case model.CatOrphanedCache:
			stats.Orphaned++
		case model.CatRecentCache:
			stats.Recent++
		}
		if meta.EvidenceIncomplete {
			stats.EvidenceIncomplete++
		}

		items = append(items, model.SwapItem{
			ID:              model.ShortID(model.SwapItem{Path: fullPath}),
			Kind:            model.KindCache,
			Name:            name,
			Command:         fullPath,
			Path:            fullPath,
			Category:        cat,
			CacheKind:       kind,
			SwapBytes:       0,
			SizeBytes:       allocated,
			AllocatedBytes:  allocated,
			ApparentBytes:   apparent,
			Age:             opts.Now.Sub(fi.ModTime()),
			LastActive:      opts.Now.Sub(meta.NewestMtime),
			LastActiveAt:    meta.NewestMtime,
			LastActiveKnown: true,
			Cleanable:       cleanable,
			Count:           1,
			OpenHandles:     meta.OpenHandles,
			CwdInside:       meta.CwdInside,
			MappedInside:    meta.MappedInside,
			Evidence:        evidence,
			UID:             currentUID,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Path < items[j].Path
	})
	return items, stats
}

// MeasureTree exposes allocated and apparent size measurement for tests.
func MeasureTree(path string, fi os.FileInfo) (allocated, apparent int64, fallback bool) {
	return measureTree(path, fi)
}

func measureTree(path string, fi os.FileInfo) (allocated, apparent int64, fallback bool) {
	if !fi.IsDir() {
		a, ok := statBlocks(fi)
		if !ok {
			return fi.Size(), fi.Size(), true
		}
		return a, fi.Size(), false
	}
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if a, ok := statBlocks(info); ok {
			allocated += a
		} else {
			fallback = true
			if !info.IsDir() {
				allocated += info.Size()
			}
		}
		if !info.IsDir() {
			apparent += info.Size()
		}
		return nil
	})
	if err != nil {
		fallback = true
	}
	return allocated, apparent, fallback
}

func statBlocks(fi os.FileInfo) (int64, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return st.Blocks * 512, true
}

func newestMtime(path string, fi os.FileInfo) time.Time {
	newest := fi.ModTime()
	if !fi.IsDir() {
		return newest
	}
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil {
			return nil
		}
		info, err := d.Info()
		if err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return newest
}
