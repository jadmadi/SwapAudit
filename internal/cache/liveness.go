//go:build linux

package cache

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxScannedPIDs = 10000
	maxScannedFDs  = 200000
)

// OpenIndex is a snapshot of open file descriptors, working directories, and
// file-backed mappings under a set of roots. It is built without external tools.
type OpenIndex struct {
	Paths       map[string]int
	Cwds        map[string]int
	Mapped      map[string]int
	Roots       []string
	Truncated   bool
	Errors      int
	ScannedPIDs int
	ScannedFDs  int
}

// BuildOpenIndex scans procRoot for descriptors, cwds, and mappings whose
// resolved path is under one of roots. It never follows symlinks outside /proc.
func BuildOpenIndex(procRoot string, roots []string) OpenIndex {
	idx := OpenIndex{
		Paths:  make(map[string]int),
		Cwds:   make(map[string]int),
		Mapped: make(map[string]int),
	}
	for _, r := range roots {
		if r == "" {
			continue
		}
		idx.Roots = append(idx.Roots, filepath.Clean(r))
	}
	if len(idx.Roots) == 0 {
		return idx
	}

	entries, err := os.ReadDir(procRoot)
	if err != nil {
		idx.Errors++
		idx.Truncated = true
		return idx
	}

	currentUID := uint32(os.Getuid())
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if idx.ScannedPIDs >= maxScannedPIDs {
			idx.Truncated = true
			break
		}
		idx.ScannedPIDs++
		pidDir := filepath.Join(procRoot, entry.Name())

		fdEntries, err := os.ReadDir(filepath.Join(pidDir, "fd"))
		if err != nil {
			if uidKnown, uid := procUID(pidDir); !uidKnown || uid == currentUID {
				idx.Errors++
			}
		} else {
			for _, fd := range fdEntries {
				if idx.ScannedFDs >= maxScannedFDs {
					idx.Truncated = true
					break
				}
				idx.ScannedFDs++
				target, err := os.Readlink(filepath.Join(pidDir, "fd", fd.Name()))
				if err != nil {
					continue
				}
				idx.addPath(idx.Paths, target)
			}
		}

		if target, err := os.Readlink(filepath.Join(pidDir, "cwd")); err == nil {
			idx.addPath(idx.Cwds, target)
		}

		if f, err := os.Open(filepath.Join(pidDir, "maps")); err == nil {
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			for scanner.Scan() {
				if p := mapsPath(scanner.Text()); p != "" {
					idx.addPath(idx.Mapped, p)
				}
			}
			f.Close()
		}
	}
	return idx
}

// Lookup returns the open-handle, cwd, and mapping counts for a path.
func (idx OpenIndex) Lookup(path string) (handles, cwds, mapped int) {
	clean := filepath.Clean(path)
	return idx.Paths[clean], idx.Cwds[clean], idx.Mapped[clean]
}

// Complete reports whether the index gathered every same-UID descriptor it saw.
func (idx OpenIndex) Complete() bool {
	return !idx.Truncated && idx.Errors == 0
}

// IncompleteReason returns a stable reason string for incomplete evidence.
func (idx OpenIndex) IncompleteReason() string {
	if idx.Truncated {
		return "index-truncated"
	}
	if idx.Errors > 0 {
		return "proc-unreadable"
	}
	return ""
}

func (idx OpenIndex) addPath(dest map[string]int, target string) {
	clean := filepath.Clean(target)
	if !filepath.IsAbs(clean) {
		return
	}
	for _, root := range idx.Roots {
		if clean != root && !strings.HasPrefix(clean, root+"/") {
			continue
		}
		for p := clean; ; p = filepath.Dir(p) {
			dest[p]++
			if p == root || p == "/" || p == "." {
				break
			}
		}
	}
}

func procUID(pidDir string) (bool, uint32) {
	f, err := os.Open(filepath.Join(pidDir, "status"))
	if err != nil {
		return false, 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return false, 0
		}
		uid, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil {
			return false, 0
		}
		return true, uint32(uid)
	}
	return false, 0
}

func mapsPath(line string) string {
	// Format: address perms offset dev inode pathname
	fields := strings.SplitN(line, " ", 6)
	if len(fields) < 6 {
		return ""
	}
	path := strings.TrimSpace(fields[5])
	path = strings.TrimSuffix(path, " (deleted)")
	if path == "" || path == "[heap]" || path == "[stack]" || strings.HasPrefix(path, "[") {
		return ""
	}
	return path
}
