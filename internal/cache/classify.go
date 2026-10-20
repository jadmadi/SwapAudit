//go:build linux

package cache

import (
	"fmt"
	"strings"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

// CacheMeta is the evidence available for one top-level /tmp entry.
type CacheMeta struct {
	Name               string
	Kind               model.CacheKind
	RootMtime          time.Time
	NewestMtime        time.Time
	AllocatedBytes     int64
	OpenHandles        int
	CwdInside          bool
	MappedInside       bool
	EvidenceIncomplete bool
}

// Thresholds controls cache staleness and orphan classification.
type Thresholds struct {
	MaxAge         time.Duration
	OrphanCacheAge time.Duration
}

// Flags controls which classifications are eligible for cleanup.
type Flags struct {
	AllowGenericCaches  bool
	CleanOrphanedCaches bool
}

// ClassifyCache applies the liveness-first classification precedence.
// The first matching rule wins.
func ClassifyCache(meta CacheMeta, now time.Time, th Thresholds, flags Flags) (model.Category, bool, string) {
	idle := now.Sub(meta.NewestMtime)
	evidence := fmt.Sprintf("idle=%s handles=%d cwd=%t mapped=%t kind=%s",
		formatIdle(idle), meta.OpenHandles, meta.CwdInside, meta.MappedInside, kindString(meta.Kind))

	if meta.OpenHandles > 0 || meta.CwdInside || meta.MappedInside {
		return model.CatInUseCache, false, evidence + " in-use=true"
	}
	if meta.EvidenceIncomplete {
		return model.CatInUseCache, false, evidence + " evidence=incomplete"
	}

	if idle > th.MaxAge {
		switch meta.Kind {
		case model.CacheBuild:
			return model.CatUntouchedCache, true, evidence + " stale=true"
		case model.CacheRun:
			return model.CatAbandonedRun, true, evidence + " stale=true"
		default:
			if flags.AllowGenericCaches {
				return model.CatAbandonedRun, true, evidence + " stale=true generic=allowed"
			}
			return model.CatRecentCache, false, evidence + " stale=true generic=protected"
		}
	}

	if idle > th.OrphanCacheAge && th.OrphanCacheAge > 0 {
		if meta.Kind != model.CacheGeneric || flags.AllowGenericCaches {
			if flags.CleanOrphanedCaches {
				return model.CatOrphanedCache, true, evidence + " orphaned=true opt-in=true"
			}
			return model.CatOrphanedCache, false, evidence + " orphaned=true use --clean-orphaned-caches"
		}
	}

	return model.CatRecentCache, false, evidence
}

// ClassifyCacheKind derives the lifecycle kind from the directory name and
// matched patterns.
func ClassifyCacheKind(name string, patterns []string) model.CacheKind {
	lower := strings.ToLower(name)

	buildPrefixes := []string{
		"go-build", "rustup-update", "cargo-install", "cargo-", "npm-",
		"sila-cache-", "pip-", "yarn-", "pnpm-", "gradle-", "m2-",
		"cmake-", "ninja-", "bazel-",
	}
	for _, p := range buildPrefixes {
		if strings.HasPrefix(lower, p) {
			return model.CacheBuild
		}
	}

	runPrefixes := []string{"test", "test-bwrap", "tsx-", "tmp-", "temp-"}
	for _, p := range runPrefixes {
		if strings.HasPrefix(lower, p) {
			return model.CacheRun
		}
	}
	for _, pat := range patterns {
		if MatchTmpPattern(pat, name) {
			return model.CacheRun
		}
	}
	for _, k := range []string{"test", "build", "review", "candidate", "probe", "profile", "repro"} {
		if strings.Contains(lower, k) {
			return model.CacheRun
		}
	}
	if strings.Contains(lower, "cache") {
		return model.CacheGeneric
	}
	return model.CacheGeneric
}

func kindString(k model.CacheKind) string {
	if k == "" {
		return string(model.CacheGeneric)
	}
	return string(k)
}

func formatIdle(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours()/24), int(d.Hours())%24)
}
