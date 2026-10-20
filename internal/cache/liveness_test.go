//go:build linux

package cache

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

func completeIndex() OpenIndex {
	return OpenIndex{Paths: map[string]int{}, Cwds: map[string]int{}, Mapped: map[string]int{}}
}

func TestAllocatedBytesSparseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sparse.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(1 << 30); err != nil {
		t.Fatal(err)
	}
	f.Close()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	allocated, apparent, fallback := MeasureTree(path, fi)
	if fallback {
		t.Fatal("unexpected fallback on a Linux filesystem")
	}
	if apparent < 1<<30 {
		t.Errorf("apparent = %d, want >= 1 GiB", apparent)
	}
	if allocated > 64*1024 {
		t.Errorf("allocated = %d, want <= 64 KiB for a sparse file", allocated)
	}
}

func TestSkipReasons(t *testing.T) {
	base := t.TempDir()
	mustMkdir := func(name string) string {
		p := filepath.Join(base, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	mustMkdir(".hidden")
	mustMkdir("tmux-1000")
	mustMkdir("systemd-test")
	mustMkdir("unrelated-name")
	mustMkdir("go-build1234")

	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(base, "test-link")); err != nil {
		t.Fatal(err)
	}

	socketPath := filepath.Join(base, "test-socket")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	now := time.Now()
	if err := os.Chtimes(filepath.Join(base, "go-build1234"), now.Add(-24*time.Hour), now.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	idx := completeIndex()
	items, stats := ScanTmpDirectoriesDetailed(base, ScanOptions{
		Now: now, MaxAge: 12 * time.Hour, OrphanCacheAge: time.Hour, OpenIndex: &idx,
	})
	counts := map[string]int{}
	for _, s := range stats.Skipped {
		counts[s.Reason] = s.Count
	}
	for _, reason := range []string{"dotfile", "daemon-prefix", "systemd-or-pulse", "symlink", "special-file", "non-matching"} {
		if counts[reason] != 1 {
			t.Errorf("reason %q count = %d, want 1; all=%+v", reason, counts[reason], stats.Skipped)
		}
	}
	if len(items) != 1 || items[0].Name != "go-build1234" {
		t.Fatalf("items = %+v, want only go-build1234", items)
	}
}

func TestClassifyCachePrecedence(t *testing.T) {
	now := time.Now()
	th := Thresholds{MaxAge: 12 * time.Hour, OrphanCacheAge: time.Hour}

	tests := []struct {
		name      string
		meta      CacheMeta
		flags     Flags
		wantCat   model.Category
		cleanable bool
	}{
		{
			name:    "open handle wins over stale build",
			meta:    CacheMeta{Kind: model.CacheBuild, NewestMtime: now.Add(-48 * time.Hour), OpenHandles: 1},
			wantCat: model.CatInUseCache,
		},
		{
			name:    "cwd wins over stale run",
			meta:    CacheMeta{Kind: model.CacheRun, NewestMtime: now.Add(-48 * time.Hour), CwdInside: true},
			wantCat: model.CatInUseCache,
		},
		{
			name:    "mapped wins over stale run",
			meta:    CacheMeta{Kind: model.CacheRun, NewestMtime: now.Add(-48 * time.Hour), MappedInside: true},
			wantCat: model.CatInUseCache,
		},
		{
			name:    "incomplete evidence fails closed",
			meta:    CacheMeta{Kind: model.CacheBuild, NewestMtime: now.Add(-48 * time.Hour), EvidenceIncomplete: true},
			wantCat: model.CatInUseCache,
		},
		{
			name:      "stale build is untouched and cleanable",
			meta:      CacheMeta{Kind: model.CacheBuild, NewestMtime: now.Add(-13 * time.Hour)},
			wantCat:   model.CatUntouchedCache,
			cleanable: true,
		},
		{
			name:      "stale run is abandoned and cleanable",
			meta:      CacheMeta{Kind: model.CacheRun, NewestMtime: now.Add(-13 * time.Hour)},
			wantCat:   model.CatAbandonedRun,
			cleanable: true,
		},
		{
			name:    "stale generic is protected by default",
			meta:    CacheMeta{Kind: model.CacheGeneric, NewestMtime: now.Add(-13 * time.Hour)},
			wantCat: model.CatRecentCache,
		},
		{
			name:      "stale generic is abandoned when allowed",
			meta:      CacheMeta{Kind: model.CacheGeneric, NewestMtime: now.Add(-13 * time.Hour)},
			flags:     Flags{AllowGenericCaches: true},
			wantCat:   model.CatAbandonedRun,
			cleanable: true,
		},
		{
			name:    "young orphan cache is not cleanable by default",
			meta:    CacheMeta{Kind: model.CacheBuild, NewestMtime: now.Add(-2 * time.Hour)},
			wantCat: model.CatOrphanedCache,
		},
		{
			name:      "young orphan cache is cleanable on opt-in",
			meta:      CacheMeta{Kind: model.CacheBuild, NewestMtime: now.Add(-2 * time.Hour)},
			flags:     Flags{CleanOrphanedCaches: true},
			wantCat:   model.CatOrphanedCache,
			cleanable: true,
		},
		{
			name:    "young recent cache",
			meta:    CacheMeta{Kind: model.CacheBuild, NewestMtime: now.Add(-10 * time.Minute)},
			wantCat: model.CatRecentCache,
		},
		{
			name:    "idle exactly max age is orphaned, not stale",
			meta:    CacheMeta{Kind: model.CacheBuild, NewestMtime: now.Add(-12 * time.Hour)},
			wantCat: model.CatOrphanedCache,
		},
		{
			name:    "idle exactly orphan age is recent",
			meta:    CacheMeta{Kind: model.CacheBuild, NewestMtime: now.Add(-time.Hour)},
			wantCat: model.CatRecentCache,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, cleanable, evidence := ClassifyCache(tt.meta, now, th, tt.flags)
			if cat != tt.wantCat || cleanable != tt.cleanable {
				t.Errorf("ClassifyCache() = (%v, %v), want (%v, %v); evidence=%q", cat, cleanable, tt.wantCat, tt.cleanable, evidence)
			}
			if evidence == "" {
				t.Errorf("expected non-empty evidence")
			}
		})
	}
}

func TestClassifyCacheKind(t *testing.T) {
	tests := []struct {
		name string
		want model.CacheKind
	}{
		{"go-build123", model.CacheBuild},
		{"cargo-install123", model.CacheBuild},
		{"npm-cache", model.CacheBuild},
		{"TestFoo123", model.CacheRun},
		{"test-bwrap999", model.CacheRun},
		{"tsx-123", model.CacheRun},
		{"tmp-123", model.CacheRun},
		{"some-cache", model.CacheGeneric},
	}
	for _, tt := range tests {
		if got := ClassifyCacheKind(tt.name, nil); got != tt.want {
			t.Errorf("ClassifyCacheKind(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestBuildOpenIndexFakeProc(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, "go-build123")
	if err := os.MkdirAll(filepath.Join(cacheDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	openFile := filepath.Join(cacheDir, "sub", "open.o")
	if err := os.WriteFile(openFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	procRoot := t.TempDir()
	pidDir := filepath.Join(procRoot, "123")
	if err := os.MkdirAll(filepath.Join(pidDir, "fd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(openFile, filepath.Join(pidDir, "fd", "3")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(cacheDir, filepath.Join(pidDir, "cwd")); err != nil {
		t.Fatal(err)
	}
	maps := "7f00-7f01 r--p 00000000 00:00 1 " + openFile + " (deleted)\n"
	maps += "7f02-7f03 rw-p 00000000 00:00 0 [heap]\n"
	if err := os.WriteFile(filepath.Join(pidDir, "maps"), []byte(maps), 0o644); err != nil {
		t.Fatal(err)
	}
	status := "Name:\ttest\nUid:\t" + itoa(int(os.Getuid())) + "\t0\t0\t0\n"
	if err := os.WriteFile(filepath.Join(pidDir, "status"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}

	idx := BuildOpenIndex(procRoot, []string{root})
	if !idx.Complete() {
		t.Fatalf("index should be complete: %+v", idx)
	}
	handles, cwds, mapped := idx.Lookup(cacheDir)
	if handles == 0 {
		t.Errorf("handles = 0, want > 0 (nested open file should mark the ancestor)")
	}
	if cwds == 0 {
		t.Errorf("cwds = 0, want > 0")
	}
	if mapped == 0 {
		t.Errorf("mapped = 0, want > 0 (deleted suffix should be stripped and counted)")
	}
}

func TestOpenIndexPermissionErrors(t *testing.T) {
	procRoot := t.TempDir()
	pidDir := filepath.Join(procRoot, "456")
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "fd"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	status := "Uid:\t" + itoa(int(os.Getuid())) + "\t0\t0\t0\n"
	if err := os.WriteFile(filepath.Join(pidDir, "status"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := BuildOpenIndex(procRoot, []string{"/tmp"})
	if idx.Errors == 0 {
		t.Fatalf("expected an error to be counted")
	}
	if idx.Complete() {
		t.Errorf("index with errors must not be complete")
	}
}

func TestOpenIndexDeletedSuffix(t *testing.T) {
	line := "7f00-7f01 r--p 00000000 00:00 1 /tmp/cache dir/file.o (deleted)"
	if got := mapsPath(line); got != "/tmp/cache dir/file.o" {
		t.Errorf("mapsPath() = %q, want %q", got, "/tmp/cache dir/file.o")
	}
	if got := mapsPath("garbage"); got != "" {
		t.Errorf("mapsPath(garbage) = %q, want empty", got)
	}
}

func TestOpenIndexComplete(t *testing.T) {
	if !(OpenIndex{}).Complete() {
		t.Errorf("zero index should be complete")
	}
	if (OpenIndex{Truncated: true}).Complete() {
		t.Errorf("truncated index should be incomplete")
	}
	if (OpenIndex{Errors: 1}).Complete() {
		t.Errorf("errored index should be incomplete")
	}
}

func TestScanOrphanedCacheOptIn(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "go-build999")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-6 * time.Hour)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	idx := completeIndex()
	now := time.Now()

	items, _ := ScanTmpDirectoriesDetailed(base, ScanOptions{
		Now: now, MaxAge: 12 * time.Hour, OrphanCacheAge: time.Hour, OpenIndex: &idx,
	})
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Category != model.CatOrphanedCache || items[0].Cleanable {
		t.Errorf("default classification = (%v, cleanable=%v), want orphaned, false", items[0].Category, items[0].Cleanable)
	}

	items, _ = ScanTmpDirectoriesDetailed(base, ScanOptions{
		Now: now, MaxAge: 12 * time.Hour, OrphanCacheAge: time.Hour, OpenIndex: &idx,
		CleanOrphanedCaches: true,
	})
	if !items[0].Cleanable {
		t.Errorf("opt-in orphaned cache should be cleanable")
	}
}

func TestScanInUseAndEvidenceIncomplete(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "go-build555")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	now := time.Now()

	idx := OpenIndex{Paths: map[string]int{dir: 1}, Cwds: map[string]int{}, Mapped: map[string]int{}}
	items, _ := ScanTmpDirectoriesDetailed(base, ScanOptions{
		Now: now, MaxAge: 12 * time.Hour, OrphanCacheAge: time.Hour, OpenIndex: &idx,
	})
	if len(items) != 1 || items[0].Category != model.CatInUseCache || items[0].Cleanable {
		t.Fatalf("open handle should be protected, got %+v", items)
	}

	idx = OpenIndex{Paths: map[string]int{}, Cwds: map[string]int{}, Mapped: map[string]int{}, Errors: 1}
	items, stats := ScanTmpDirectoriesDetailed(base, ScanOptions{
		Now: now, MaxAge: 12 * time.Hour, OrphanCacheAge: time.Hour, OpenIndex: &idx,
	})
	if len(items) != 1 || items[0].Category != model.CatInUseCache || items[0].Cleanable {
		t.Fatalf("incomplete evidence should fail closed, got %+v", items)
	}
	if stats.EvidenceIncomplete != 1 {
		t.Errorf("evidence incomplete count = %d, want 1", stats.EvidenceIncomplete)
	}
}

func TestScanUsesAllocatedSize(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "go-build777")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Repeat("x", 4096)
	if err := os.WriteFile(filepath.Join(dir, "file"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	idx := completeIndex()
	items, _ := ScanTmpDirectoriesDetailed(base, ScanOptions{
		Now: time.Now(), MaxAge: 12 * time.Hour, OrphanCacheAge: time.Hour, OpenIndex: &idx,
	})
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].SizeBytes != items[0].AllocatedBytes {
		t.Errorf("SizeBytes = %d, AllocatedBytes = %d; display must use allocated", items[0].SizeBytes, items[0].AllocatedBytes)
	}
	if items[0].ApparentBytes < int64(len(content)) {
		t.Errorf("ApparentBytes = %d, want >= %d", items[0].ApparentBytes, len(content))
	}
}

func BenchmarkBuildOpenIndex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = BuildOpenIndex("/proc", []string{"/tmp"})
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}
