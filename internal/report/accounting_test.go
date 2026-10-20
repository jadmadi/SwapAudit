package report

import (
	"math/rand"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

func procItem(name string, cat model.Category, swap int64, cleanable bool) model.SwapItem {
	return model.SwapItem{Kind: model.KindProcess, Name: name, Category: cat, SwapBytes: swap, Cleanable: cleanable, Count: 1}
}

func cacheItem(name string, cat model.Category, allocated int64, cleanable bool) model.SwapItem {
	return model.SwapItem{Kind: model.KindCache, Name: name, Path: "/tmp/" + name, Category: cat, AllocatedBytes: allocated, SizeBytes: allocated, Cleanable: cleanable, Count: 1}
}

func TestBuildReconcilesSwap(t *testing.T) {
	system := model.SystemSwap{SwapUsed: 1000}
	items := []model.SwapItem{
		procItem("a", model.CatDormantAgent, 100, true),
		procItem("b", model.CatActiveProcess, 200, false),
	}
	r, err := BuildWithScan(time.Now(), system, items, Thresholds{}, Scan{
		SwapReasons: []ReasonBytes{{Reason: "zero-swap", Count: 3}},
	})
	if err != nil {
		t.Fatalf("BuildWithScan error: %v", err)
	}
	if r.Swap.AttributedBytes != 300 {
		t.Errorf("attributed = %d, want 300", r.Swap.AttributedBytes)
	}
	if r.Swap.UnattributedBytes != 700 {
		t.Errorf("unattributed = %d, want 700", r.Swap.UnattributedBytes)
	}
	if r.Swap.OverlapBytes != 0 {
		t.Errorf("overlap = %d, want 0", r.Swap.OverlapBytes)
	}
	if r.Swap.AttributedBytes+r.Swap.UnattributedBytes != r.Swap.TotalBytes {
		t.Errorf("swap does not reconcile")
	}
	found := false
	for _, rr := range r.Swap.Reasons {
		if rr.Reason == "kernel-or-shmem" && rr.Bytes == 700 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected kernel-or-shmem reason with 700 bytes, got %+v", r.Swap.Reasons)
	}
	if err := r.Reconciles(); err != nil {
		t.Errorf("Reconciles() = %v", err)
	}
}

func TestBuildReconcilesTmpfs(t *testing.T) {
	system := model.SystemSwap{TmpfsUsed: 10000}
	items := []model.SwapItem{
		cacheItem("c1", model.CatUntouchedCache, 3000, true),
		cacheItem("c2", model.CatInUseCache, 2000, false),
	}
	r, err := BuildWithScan(time.Now(), system, items, Thresholds{}, Scan{
		TmpfsReasons: []ReasonBytes{{Reason: "other-uid", Bytes: 1000, Count: 1}},
	})
	if err != nil {
		t.Fatalf("BuildWithScan error: %v", err)
	}
	if r.Tmpfs.AttributedBytes != 5000 || r.Tmpfs.UnattributedBytes != 5000 {
		t.Errorf("tmpfs attribution = %+v, want 5000/5000", r.Tmpfs)
	}
	var other, metadata int64
	for _, rr := range r.Tmpfs.Reasons {
		switch rr.Reason {
		case "other-uid":
			other = rr.Bytes
		case "metadata-or-other":
			metadata = rr.Bytes
		}
	}
	if other != 1000 {
		t.Errorf("other-uid reason = %d, want 1000", other)
	}
	if metadata != 4000 {
		t.Errorf("metadata-or-other reason = %d, want 4000", metadata)
	}
	if err := r.Reconciles(); err != nil {
		t.Errorf("Reconciles() = %v", err)
	}
}

func TestBuildOverlap(t *testing.T) {
	system := model.SystemSwap{SwapUsed: 100, TmpfsUsed: 50}
	items := []model.SwapItem{
		procItem("a", model.CatDormantAgent, 150, true),
		cacheItem("c", model.CatUntouchedCache, 80, true),
	}
	r, err := Build(time.Now(), system, items, Thresholds{})
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if r.Swap.OverlapBytes != 50 || r.Swap.UnattributedBytes != 0 {
		t.Errorf("swap overlap = %d, unattributed = %d, want 50/0", r.Swap.OverlapBytes, r.Swap.UnattributedBytes)
	}
	if r.Tmpfs.OverlapBytes != 30 || r.Tmpfs.UnattributedBytes != 0 {
		t.Errorf("tmpfs overlap = %d, unattributed = %d, want 30/0", r.Tmpfs.OverlapBytes, r.Tmpfs.UnattributedBytes)
	}
	if r.Swap.OverlapBytes < 0 || r.Tmpfs.OverlapBytes < 0 {
		t.Errorf("overlap must never be negative")
	}
	if err := r.Reconciles(); err != nil {
		t.Errorf("Reconciles() = %v", err)
	}
}

func TestBuildNoHybridItems(t *testing.T) {
	item := model.SwapItem{Kind: model.KindCache, Name: "hybrid", SwapBytes: 10, AllocatedBytes: 20}
	if _, err := Build(time.Now(), model.SystemSwap{}, []model.SwapItem{item}, Thresholds{}); err == nil {
		t.Fatal("expected an error for an item with both swap and allocated bytes")
	}
	item.Kind = model.KindHybrid
	if _, err := Build(time.Now(), model.SystemSwap{SwapUsed: 10, TmpfsUsed: 20}, []model.SwapItem{item}, Thresholds{}); err != nil {
		t.Fatalf("hybrid item should be accepted: %v", err)
	}
}

func TestBuildCategoriesDeterministic(t *testing.T) {
	items := []model.SwapItem{
		cacheItem("c", model.CatRecentCache, 10, false),
		procItem("p", model.CatDormantAgent, 20, true),
		cacheItem("u", model.CatUntouchedCache, 30, true),
	}
	now := time.Unix(1700000000, 0)
	a, err := Build(now, model.SystemSwap{SwapUsed: 100, TmpfsUsed: 100}, items, Thresholds{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(now, model.SystemSwap{SwapUsed: 100, TmpfsUsed: 100}, items, Thresholds{})
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Categories) != len(b.Categories) {
		t.Fatalf("category length differs: %d vs %d", len(a.Categories), len(b.Categories))
	}
	for i := range a.Categories {
		if a.Categories[i] != b.Categories[i] {
			t.Errorf("category %d differs: %+v vs %+v", i, a.Categories[i], b.Categories[i])
		}
	}
	if a.Category(model.CatDormantAgent).ReclaimableBytes != 20 {
		t.Errorf("dormant reclaimable = %d, want 20", a.Category(model.CatDormantAgent).ReclaimableBytes)
	}
	if a.Reclaim.SwapBytes != 20 || a.Reclaim.TmpfsBytes != 30 {
		t.Errorf("reclaim = %+v, want swap 20 tmpfs 30", a.Reclaim)
	}
}

func TestBuildBiggestWin(t *testing.T) {
	items := []model.SwapItem{
		procItem("b", model.CatDormantAgent, 100, true),
		procItem("a", model.CatDormantAgent, 100, true),
		procItem("c", model.CatDormantAgent, 50, false),
	}
	items[0].PID = 20
	items[1].PID = 10
	r, err := Build(time.Now(), model.SystemSwap{SwapUsed: 1000}, items, Thresholds{})
	if err != nil {
		t.Fatal(err)
	}
	best, ok := r.BiggestWin()
	if !ok {
		t.Fatal("expected a biggest win")
	}
	if best.PID != 10 {
		t.Errorf("biggest win PID = %d, want 10 (deterministic tie-break)", best.PID)
	}
}

func TestBuildDoesNotMutate(t *testing.T) {
	items := []model.SwapItem{procItem("a", model.CatDormantAgent, 100, true)}
	before := items[0]
	_, err := Build(time.Now(), model.SystemSwap{SwapUsed: 100}, items, Thresholds{})
	if err != nil {
		t.Fatal(err)
	}
	if items[0] != before {
		t.Errorf("Build mutated the input item: %+v", items[0])
	}
}

func TestRandomFixturesNoNegativeResidual(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		var items []model.SwapItem
		for j := 0; j < rng.Intn(20); j++ {
			if rng.Intn(2) == 0 {
				items = append(items, procItem("p", model.CatDormantAgent, rng.Int63n(1<<30), rng.Intn(2) == 0))
			} else {
				items = append(items, cacheItem("c", model.CatUntouchedCache, rng.Int63n(1<<30), rng.Intn(2) == 0))
			}
		}
		system := model.SystemSwap{
			SwapTotal: rng.Int63n(1 << 34),
			SwapUsed:  rng.Int63n(1 << 34),
			TmpfsUsed: rng.Int63n(1 << 34),
		}
		r, err := Build(time.Now(), system, items, Thresholds{})
		if err != nil {
			t.Fatalf("fixture %d: %v", i, err)
		}
		if err := r.Reconciles(); err != nil {
			t.Fatalf("fixture %d: %v", i, err)
		}
		if r.Swap.UnattributedBytes < 0 || r.Tmpfs.UnattributedBytes < 0 || r.Swap.OverlapBytes < 0 || r.Tmpfs.OverlapBytes < 0 {
			t.Fatalf("fixture %d produced a negative residual: %+v %+v", i, r.Swap, r.Tmpfs)
		}
	}
}

func TestCategoriesTieToAttribution(t *testing.T) {
	items := []model.SwapItem{
		procItem("a", model.CatDormantAgent, 100, true),
		procItem("b", model.CatActiveProcess, 200, false),
		cacheItem("c", model.CatUntouchedCache, 300, true),
		cacheItem("d", model.CatRecentCache, 400, false),
	}
	r, err := Build(time.Now(), model.SystemSwap{SwapUsed: 1000, TmpfsUsed: 1000}, items, Thresholds{})
	if err != nil {
		t.Fatal(err)
	}
	var swapAttributed, tmpfsAttributed, swapReclaim, tmpfsReclaim int64
	for _, c := range r.Categories {
		if c.Pool == PoolProcessSwap {
			swapAttributed += c.AttributedBytes
			swapReclaim += c.ReclaimableBytes
		} else {
			tmpfsAttributed += c.AttributedBytes
			tmpfsReclaim += c.ReclaimableBytes
		}
	}
	if swapAttributed != r.Swap.AttributedBytes || tmpfsAttributed != r.Tmpfs.AttributedBytes {
		t.Errorf("category attribution does not tie: swap %d/%d tmpfs %d/%d",
			swapAttributed, r.Swap.AttributedBytes, tmpfsAttributed, r.Tmpfs.AttributedBytes)
	}
	if swapReclaim != r.Reclaim.SwapBytes || tmpfsReclaim != r.Reclaim.TmpfsBytes {
		t.Errorf("category reclaim does not tie: swap %d/%d tmpfs %d/%d",
			swapReclaim, r.Reclaim.SwapBytes, tmpfsReclaim, r.Reclaim.TmpfsBytes)
	}
}

func TestFormatReasonsOmitsZeroBytes(t *testing.T) {
	got := FormatReasons([]ReasonBytes{
		{Reason: "zero-swap", Count: 3},
		{Reason: "other-uid", Bytes: 1024, Count: 1},
	})
	if got != "other-uid 1.0 KB" {
		t.Errorf("FormatReasons() = %q, want %q", got, "other-uid 1.0 KB")
	}
}
