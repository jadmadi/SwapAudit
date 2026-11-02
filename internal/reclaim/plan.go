//go:build linux

// Package reclaim plans and executes a no-reboot swap drain and page-cache
// reclaim with fail-closed safety checks.
package reclaim

import (
	"sort"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
)

// DefaultReserveFloor is the minimum RAM kept out of the drain budget.
const DefaultReserveFloor int64 = 512 << 20

// DefaultReserve returns the RAM that must remain available after every page
// in swap is faulted back in. It is 10% of physical RAM, but never less than
// 512 MiB.
func DefaultReserve(memTotal int64) int64 {
	r := memTotal / 10
	if r < DefaultReserveFloor {
		return DefaultReserveFloor
	}
	return r
}

// SkipReason explains why a device is not part of the drain.
type SkipReason string

const (
	SkipEmpty    SkipReason = "already-empty"
	SkipZram     SkipReason = "zram"
	SkipHeadroom SkipReason = "insufficient-headroom"
)

// BlockReason explains why a plan cannot drain any swap.
type BlockReason string

const (
	BlockNone         BlockReason = ""
	BlockNoSwap       BlockReason = "no-swap"
	BlockAlreadyClean BlockReason = "already-clean"
	BlockZramOnly     BlockReason = "zram-only"
	BlockInsufficient BlockReason = "insufficient-headroom"
)

// DevicePlan is the decision for one active swap area.
type DevicePlan struct {
	Device     proc.SwapDevice
	Drain      bool
	SkipReason SkipReason
}

// Plan is the pure drain decision for a system snapshot.
type Plan struct {
	System             model.SystemSwap
	Devices            []DevicePlan
	SwapUsed           int64
	DrainableUsed      int64
	AllDrainableUsed   int64
	DrainableDevices   int
	SkippedDevices     int
	SkippedForHeadroom int64
	ZramDevices        int
	ZramUsed           int64
	Reserve            int64
	Headroom           int64
	SelectedHeadroom   int64
	Partial            bool
	Feasible           bool
	NoSwap             bool
	AlreadyClean       bool
	BlockReason        BlockReason
}

// Executable reports whether the plan has a drain to run.
func (p Plan) Executable() bool {
	return p.Feasible && p.DrainableDevices > 0 && !p.AlreadyClean && !p.NoSwap
}

// BuildPlan is pure and deterministic. The budget is
// MemAvailable - reserve. If every drainable device fits, all are selected.
// Otherwise a subset is selected so a partial drain can still clean swap
// without a reboot, and the rest is reported as skipped for headroom.
func BuildPlan(sys model.SystemSwap, devices []proc.SwapDevice, reserve int64, includeZram bool) Plan {
	if reserve < 0 {
		reserve = 0
	}
	p := Plan{
		System:  sys,
		Reserve: reserve,
		Devices: make([]DevicePlan, 0, len(devices)),
	}

	var sumUsed int64
	var candidates []int
	for _, d := range devices {
		sumUsed += d.UsedBytes
		dp := DevicePlan{Device: d}
		switch {
		case d.UsedBytes <= 0:
			dp.SkipReason = SkipEmpty
		case proc.IsZramDevice(d.Filename) && !includeZram:
			dp.SkipReason = SkipZram
			p.ZramDevices++
			p.ZramUsed += d.UsedBytes
		default:
			candidates = append(candidates, len(p.Devices))
		}
		p.Devices = append(p.Devices, dp)
	}

	p.SwapUsed = sys.SwapUsed
	if p.SwapUsed < 0 {
		p.SwapUsed = 0
	}
	if sumUsed > p.SwapUsed {
		p.SwapUsed = sumUsed
	}
	p.NoSwap = len(devices) == 0
	p.AlreadyClean = !p.NoSwap && p.SwapUsed == 0

	for _, idx := range candidates {
		p.AllDrainableUsed += p.Devices[idx].Device.UsedBytes
	}
	budget := sys.MemAvail - reserve
	if budget < 0 {
		budget = 0
	}
	selected := selectDrainSubset(p.Devices, candidates, budget)
	for _, idx := range selected {
		p.Devices[idx].Drain = true
		p.DrainableUsed += p.Devices[idx].Device.UsedBytes
	}
	p.DrainableDevices = len(selected)
	for _, idx := range candidates {
		if p.Devices[idx].Drain {
			continue
		}
		p.Devices[idx].SkipReason = SkipHeadroom
		p.SkippedDevices++
		p.SkippedForHeadroom += p.Devices[idx].Device.UsedBytes
	}

	p.Headroom = sys.MemAvail - p.AllDrainableUsed - reserve
	p.SelectedHeadroom = sys.MemAvail - p.DrainableUsed - reserve
	p.Feasible = p.DrainableDevices > 0
	p.Partial = p.SkippedForHeadroom > 0 && p.DrainableDevices > 0

	switch {
	case p.NoSwap:
		p.BlockReason = BlockNoSwap
		p.Feasible = false
	case p.AlreadyClean:
		p.BlockReason = BlockAlreadyClean
		p.Feasible = false
	case p.DrainableDevices == 0 && p.AllDrainableUsed > 0:
		p.BlockReason = BlockInsufficient
		p.Feasible = false
	case p.DrainableDevices == 0 && p.ZramUsed > 0:
		p.BlockReason = BlockZramOnly
		p.Feasible = false
	}

	return p
}

// selectDrainSubset chooses devices whose used bytes fit the budget. All
// values equal their weights, so ascending and descending greedy passes are
// compared and the larger total wins. The two passes cover both
// "small devices together" and "one large device" shapes without a full
// subset-sum search.
func selectDrainSubset(devices []DevicePlan, candidates []int, budget int64) []int {
	if len(candidates) == 0 || budget <= 0 {
		return nil
	}
	var total int64
	for _, idx := range candidates {
		total += devices[idx].Device.UsedBytes
	}
	if total <= budget {
		return append([]int(nil), candidates...)
	}

	desc := greedySubset(devices, candidates, budget, true)
	asc := greedySubset(devices, candidates, budget, false)
	descUsed, ascUsed := subsetBytes(devices, desc), subsetBytes(devices, asc)

	switch {
	case descUsed > ascUsed:
		return desc
	case ascUsed > descUsed:
		return asc
	case len(desc) <= len(asc):
		return desc
	default:
		return asc
	}
}

func greedySubset(devices []DevicePlan, candidates []int, budget int64, descending bool) []int {
	ordered := append([]int(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a := devices[ordered[i]].Device.UsedBytes
		b := devices[ordered[j]].Device.UsedBytes
		if a != b {
			if descending {
				return a > b
			}
			return a < b
		}
		return devices[ordered[i]].Device.Filename < devices[ordered[j]].Device.Filename
	})

	var selected []int
	var used int64
	for _, idx := range ordered {
		u := devices[idx].Device.UsedBytes
		if used+u <= budget {
			selected = append(selected, idx)
			used += u
		}
	}
	return selected
}

func subsetBytes(devices []DevicePlan, subset []int) int64 {
	var total int64
	for _, idx := range subset {
		total += devices[idx].Device.UsedBytes
	}
	return total
}
