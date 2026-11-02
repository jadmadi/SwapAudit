//go:build linux

package reclaim

import (
	"testing"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
)

const gib = int64(1) << 30

func systemWith(avail, total int64) model.SystemSwap {
	return model.SystemSwap{
		SwapTotal: 16 * gib,
		SwapUsed:  4 * gib,
		SwapFree:  12 * gib,
		MemTotal:  total,
		MemAvail:  avail,
	}
}

func device(name string, used int64) proc.SwapDevice {
	return proc.SwapDevice{
		Filename: name, Type: "partition", SizeBytes: 8 * gib, UsedBytes: used, Priority: -2,
	}
}

func TestDefaultReserve(t *testing.T) {
	cases := []struct {
		total, want int64
	}{
		{0, DefaultReserveFloor},
		{4 * gib, DefaultReserveFloor},
		{8 * gib, 8192 * 1024 * 1024 / 10},
		{32 * gib, 32 * gib / 10},
	}
	for _, tc := range cases {
		if got := DefaultReserve(tc.total); got != tc.want {
			t.Errorf("DefaultReserve(%d) = %d, want %d", tc.total, got, tc.want)
		}
	}
}

func TestBuildPlanFeasible(t *testing.T) {
	plan := BuildPlan(systemWith(8*gib, 64*gib), []proc.SwapDevice{device("/dev/sda2", 4*gib)}, 2*gib, false)
	if !plan.Feasible || !plan.Executable() {
		t.Fatalf("plan = %+v, want feasible and executable", plan)
	}
	if plan.Headroom != 2*gib {
		t.Errorf("headroom = %d, want %d", plan.Headroom, 2*gib)
	}
	if plan.DrainableUsed != 4*gib || plan.DrainableDevices != 1 {
		t.Errorf("drainable = %d bytes / %d devices, want 4 GiB / 1", plan.DrainableUsed, plan.DrainableDevices)
	}
}

func TestBuildPlanBoundaryIsFeasible(t *testing.T) {
	plan := BuildPlan(systemWith(6*gib, 64*gib), []proc.SwapDevice{device("/dev/sda2", 4*gib)}, 2*gib, false)
	if plan.Headroom != 0 || !plan.Feasible {
		t.Fatalf("headroom = %d feasible = %v, want 0 true", plan.Headroom, plan.Feasible)
	}
}

func TestBuildPlanInsufficient(t *testing.T) {
	plan := BuildPlan(systemWith(2*gib, 64*gib), []proc.SwapDevice{device("/dev/sda2", 4*gib)}, 2*gib, false)
	if plan.Feasible || plan.Executable() {
		t.Fatalf("plan = %+v, want infeasible", plan)
	}
	if plan.BlockReason != BlockInsufficient {
		t.Errorf("block = %q, want %q", plan.BlockReason, BlockInsufficient)
	}
	if plan.Headroom != -4*gib {
		t.Errorf("headroom = %d, want %d", plan.Headroom, -4*gib)
	}
	if plan.DrainableDevices != 0 || plan.SkippedDevices != 1 || plan.SkippedForHeadroom != 4*gib {
		t.Errorf("devices = %d selected / %d skipped / %d skipped bytes, want 0/1/4GiB",
			plan.DrainableDevices, plan.SkippedDevices, plan.SkippedForHeadroom)
	}
}

func TestBuildPlanPartialDrain(t *testing.T) {
	devices := []proc.SwapDevice{
		device("/dev/sda2", 6*gib),
		device("/dev/sdb1", 5*gib),
	}
	plan := BuildPlan(systemWith(8*gib, 64*gib), devices, 2*gib, false)
	if !plan.Feasible || !plan.Executable() || !plan.Partial {
		t.Fatalf("plan = %+v, want feasible executable partial", plan)
	}
	if plan.DrainableDevices != 1 || plan.SkippedDevices != 1 {
		t.Errorf("devices = %d selected / %d skipped, want 1/1", plan.DrainableDevices, plan.SkippedDevices)
	}
	if plan.DrainableUsed != 6*gib || plan.SkippedForHeadroom != 5*gib {
		t.Errorf("selected/skipped = %d/%d, want 6 GiB/5 GiB", plan.DrainableUsed, plan.SkippedForHeadroom)
	}
	if plan.SelectedHeadroom != 0 {
		t.Errorf("selected headroom = %d, want 0", plan.SelectedHeadroom)
	}
	if plan.Headroom != -5*gib {
		t.Errorf("all-device headroom = %d, want -5 GiB", plan.Headroom)
	}
}

func TestBuildPlanPartialPicksBetterSubset(t *testing.T) {
	devices := []proc.SwapDevice{
		device("/dev/sda2", 5*gib),
		device("/dev/sdb1", 4*gib),
		device("/dev/sdc1", 2*gib),
	}
	plan := BuildPlan(systemWith(6*gib, 64*gib), devices, 0, false)
	if !plan.Partial || !plan.Feasible {
		t.Fatalf("plan = %+v, want feasible partial", plan)
	}
	if plan.DrainableUsed != 6*gib || plan.SkippedForHeadroom != 5*gib {
		t.Errorf("selected/skipped = %d/%d, want 6 GiB/5 GiB", plan.DrainableUsed, plan.SkippedForHeadroom)
	}
	if plan.DrainableDevices != 2 {
		t.Errorf("selected devices = %d, want 2 (4 GiB + 2 GiB)", plan.DrainableDevices)
	}
}

func TestBuildPlanAlreadyClean(t *testing.T) {
	sys := systemWith(8*gib, 64*gib)
	sys.SwapUsed = 0
	plan := BuildPlan(sys, []proc.SwapDevice{device("/dev/sda2", 0)}, 2*gib, false)
	if !plan.AlreadyClean || plan.Executable() {
		t.Fatalf("plan = %+v, want already clean and not executable", plan)
	}
	if plan.BlockReason != BlockAlreadyClean {
		t.Errorf("block = %q, want %q", plan.BlockReason, BlockAlreadyClean)
	}
}

func TestBuildPlanNoSwap(t *testing.T) {
	plan := BuildPlan(systemWith(8*gib, 64*gib), nil, 2*gib, false)
	if !plan.NoSwap || plan.Executable() {
		t.Fatalf("plan = %+v, want no swap and not executable", plan)
	}
	if plan.BlockReason != BlockNoSwap {
		t.Errorf("block = %q, want %q", plan.BlockReason, BlockNoSwap)
	}
}

func TestBuildPlanZramOnlyIsSkipped(t *testing.T) {
	plan := BuildPlan(systemWith(8*gib, 64*gib), []proc.SwapDevice{device("/dev/zram0", 2*gib)}, 2*gib, false)
	if plan.Executable() || plan.DrainableDevices != 0 {
		t.Fatalf("plan = %+v, want nothing drainable", plan)
	}
	if plan.BlockReason != BlockZramOnly {
		t.Errorf("block = %q, want %q", plan.BlockReason, BlockZramOnly)
	}
	if plan.ZramUsed != 2*gib || plan.ZramDevices != 1 {
		t.Errorf("zram = %d bytes / %d devices, want 2 GiB / 1", plan.ZramUsed, plan.ZramDevices)
	}
}

func TestBuildPlanMixedZramAndDisk(t *testing.T) {
	devices := []proc.SwapDevice{device("/dev/zram0", 2*gib), device("/dev/sda2", 1*gib)}
	plan := BuildPlan(systemWith(8*gib, 64*gib), devices, 2*gib, false)
	if plan.DrainableDevices != 1 || plan.DrainableUsed != 1*gib {
		t.Fatalf("drainable = %d devices / %d bytes, want 1 / 1 GiB", plan.DrainableDevices, plan.DrainableUsed)
	}
	if !plan.Feasible {
		t.Error("mixed plan should be feasible")
	}
	if plan.ZramDevices != 1 {
		t.Errorf("zram devices = %d, want 1", plan.ZramDevices)
	}
}

func TestBuildPlanIncludeZram(t *testing.T) {
	plan := BuildPlan(systemWith(8*gib, 64*gib), []proc.SwapDevice{device("/dev/zram0", 2*gib)}, 2*gib, true)
	if !plan.Executable() || plan.DrainableUsed != 2*gib {
		t.Fatalf("plan = %+v, want zram drainable", plan)
	}
}

func TestBuildPlanPerDeviceDecisions(t *testing.T) {
	devices := []proc.SwapDevice{
		device("/dev/sda2", 0),
		device("/dev/zram0", 1*gib),
		device("/dev/sdb1", 3*gib),
	}
	plan := BuildPlan(systemWith(8*gib, 64*gib), devices, 2*gib, false)
	if len(plan.Devices) != 3 {
		t.Fatalf("devices = %d, want 3", len(plan.Devices))
	}
	if plan.Devices[0].Drain || plan.Devices[0].SkipReason != SkipEmpty {
		t.Errorf("empty device = %+v, want skipped empty", plan.Devices[0])
	}
	if plan.Devices[1].Drain || plan.Devices[1].SkipReason != SkipZram {
		t.Errorf("zram device = %+v, want skipped zram", plan.Devices[1])
	}
	if !plan.Devices[2].Drain {
		t.Errorf("disk device = %+v, want drain", plan.Devices[2])
	}
}

func TestBuildPlanUsesLargerOfMeminfoAndDevices(t *testing.T) {
	sys := systemWith(8*gib, 64*gib)
	sys.SwapUsed = 1 * gib
	plan := BuildPlan(sys, []proc.SwapDevice{device("/dev/sda2", 4*gib)}, 2*gib, false)
	if plan.SwapUsed != 4*gib {
		t.Errorf("swap used = %d, want the device total 4 GiB", plan.SwapUsed)
	}
}
