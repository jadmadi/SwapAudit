//go:build linux

package cache

import (
	"io/fs"
	"os"
	"path/filepath"
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
// It verifies UID ownership, excludes symlinks and sockets, and walks directories to determine true age.
func ScanTmpDirectories(now time.Time, maxAge time.Duration, patterns []string) []model.SwapItem {
	return ScanTmpDirectoriesIn("/tmp", now, maxAge, patterns)
}

// ScanTmpDirectoriesIn scans a specified base directory for stale caches.
func ScanTmpDirectoriesIn(baseDir string, now time.Time, maxAge time.Duration, patterns []string) []model.SwapItem {
	var items []model.SwapItem
	currentUid := uint32(os.Getuid())

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return items
	}

	knownDaemonPrefixes := []string{
		"tmux-", "codex-daemon-", "MegaLimited", "plasma-",
		"wayland-", "dbus-", "ssh-", "gnupg-", "atuin-",
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "systemd-") || strings.HasPrefix(name, "pulse-") {
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
			continue
		}

		fullPath := filepath.Join(baseDir, name)
		fi, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			continue
		}

		stat_t, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || stat_t.Uid != currentUid {
			continue
		}

		if fi.Mode()&os.ModeSocket != 0 || fi.Mode()&os.ModeNamedPipe != 0 {
			continue
		}

		lower := strings.ToLower(name)
		isBuildCache := strings.HasPrefix(name, "go-build") || strings.Contains(lower, "cache")
		isTestRun := false
		for _, pat := range patterns {
			if MatchTmpPattern(pat, name) {
				isTestRun = true
				break
			}
		}

		if !isBuildCache && !isTestRun {
			keywords := []string{"test", "build", "review", "candidate", "probe", "profile", "repro"}
			matched := false
			for _, k := range keywords {
				if strings.Contains(lower, k) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			isTestRun = true
		}

		var size int64
		newestMtime := fi.ModTime()

		if entry.IsDir() {
			_ = filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr == nil && d != nil {
					if info, err := d.Info(); err == nil {
						if !d.IsDir() {
							size += info.Size()
						}
						if info.ModTime().After(newestMtime) {
							newestMtime = info.ModTime()
						}
					}
				}
				return nil
			})
		} else {
			size = fi.Size()
		}

		age := now.Sub(fi.ModTime())
		lastActive := now.Sub(newestMtime)

		cleanable := lastActive > maxAge
		var cat model.Category
		if cleanable {
			if isBuildCache {
				cat = model.CatUntouchedCache
			} else {
				cat = model.CatAbandonedRun
			}
		} else {
			cat = model.CatActiveCache
		}

		items = append(items, model.SwapItem{
			Name:         name,
			Command:      fullPath,
			Path:         fullPath,
			Category:     cat,
			SwapBytes:    0,
			SizeBytes:    size,
			Age:          age,
			LastActive:   lastActive,
			LastActiveAt: newestMtime,
			Cleanable:    cleanable,
			Count:        1,
		})
	}

	return items
}
