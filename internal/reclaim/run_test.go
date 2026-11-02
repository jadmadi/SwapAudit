//go:build linux

package reclaim

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
)

type fakeSystem struct {
	sys            model.SystemSwap
	devices        []proc.SwapDevice
	off            map[string]proc.SwapDevice
	swapOffErr     map[string]error
	swapOnFailures map[string]int
	devicesErr     error
	dropErr        error
	calls          []string
}

func newFake(used, avail int64) *fakeSystem {
	sys := model.SystemSwap{
		SwapTotal: 8 * gib, SwapUsed: used, SwapFree: 8*gib - used,
		MemTotal: 64 * gib, MemAvail: avail, MemFree: avail,
		Cached: 3 * gib, SReclaimable: 1 * gib,
	}
	dev := proc.SwapDevice{
		Filename: "/dev/sda2", Type: "partition", SizeBytes: 8 * gib, UsedBytes: used, Priority: -2,
	}
	return &fakeSystem{
		sys: sys, devices: []proc.SwapDevice{dev},
		off: map[string]proc.SwapDevice{}, swapOffErr: map[string]error{},
		swapOnFailures: map[string]int{},
	}
}

func (f *fakeSystem) SystemSwap() model.SystemSwap { return f.sys }

func (f *fakeSystem) Devices() ([]proc.SwapDevice, error) {
	if f.devicesErr != nil {
		return nil, f.devicesErr
	}
	return append([]proc.SwapDevice(nil), f.devices...), nil
}

func (f *fakeSystem) SwapOff(_ context.Context, name string) error {
	f.calls = append(f.calls, "swapoff "+name)
	if err := f.swapOffErr[name]; err != nil {
		return err
	}
	for i, d := range f.devices {
		if d.Filename != name {
			continue
		}
		f.off[name] = d
		f.devices = append(f.devices[:i], f.devices[i+1:]...)
		f.sys.SwapTotal -= d.SizeBytes
		f.sys.SwapUsed -= d.UsedBytes
		f.sys.SwapFree = f.sys.SwapTotal - f.sys.SwapUsed
		f.sys.MemAvail -= d.UsedBytes
		return nil
	}
	return nil
}

func (f *fakeSystem) SwapOn(_ context.Context, name string, priority int) error {
	f.calls = append(f.calls, "swapon "+name)
	if n := f.swapOnFailures[name]; n > 0 {
		f.swapOnFailures[name] = n - 1
		return errors.New("swapon failed")
	}
	dev, ok := f.off[name]
	if !ok {
		return nil
	}
	delete(f.off, name)
	f.devices = append(f.devices, proc.SwapDevice{
		Filename: name, Type: dev.Type, SizeBytes: dev.SizeBytes, UsedBytes: 0, Priority: priority,
	})
	f.sys.SwapTotal += dev.SizeBytes
	f.sys.SwapFree = f.sys.SwapTotal - f.sys.SwapUsed
	return nil
}

func (f *fakeSystem) DropCaches(context.Context) error {
	f.calls = append(f.calls, "drop_caches")
	if f.dropErr != nil {
		return f.dropErr
	}
	released := f.sys.PageCacheBytes()
	f.sys.Buffers, f.sys.Cached, f.sys.SReclaimable = 0, 0, 0
	f.sys.MemFree += released
	return nil
}

func (f *fakeSystem) Sync() { f.calls = append(f.calls, "sync") }

type runFixture struct {
	sys  *fakeSystem
	out  bytes.Buffer
	errb bytes.Buffer
	cfg  Config
}

func newRunFixture(used, avail int64) *runFixture {
	f := &runFixture{sys: newFake(used, avail)}
	f.cfg = Config{
		Out: &f.out, Err: &f.errb, In: bufio.NewReader(strings.NewReader("")),
		System: f.sys, IsRoot: true, Yes: true, DrainSwap: true,
		Reserve: 2 * gib, ReserveSet: true,
	}
	return f
}

func (f *runFixture) calls() string { return strings.Join(f.sys.calls, ",") }

func TestRunDryRunIsPure(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	f.cfg.DryRun = true
	f.cfg.IsRoot = false
	code := Run(f.cfg)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr: %s)", code, f.errb.String())
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("dry-run calls = %v, want none", f.sys.calls)
	}
	if !strings.Contains(f.out.String(), "Dry run") {
		t.Errorf("output missing dry-run line: %s", f.out.String())
	}
}

func TestRunRequiresRoot(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	f.cfg.IsRoot = false
	code := Run(f.cfg)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("calls = %v, want none", f.sys.calls)
	}
	if !strings.Contains(f.errb.String(), "require root") {
		t.Errorf("stderr = %q, want root error", f.errb.String())
	}
}

func TestRunRequiresYesWithoutTTY(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	f.cfg.Yes = false
	f.cfg.StdinIsTTY = false
	code := Run(f.cfg)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("calls = %v, want none", f.sys.calls)
	}
	if !strings.Contains(f.errb.String(), "requires --yes") {
		t.Errorf("stderr = %q, want --yes error", f.errb.String())
	}
}

func TestRunConfirmationCancel(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	f.cfg.Yes = false
	f.cfg.StdinIsTTY = true
	f.cfg.In = bufio.NewReader(strings.NewReader("n\n"))
	code := Run(f.cfg)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("calls = %v, want none", f.sys.calls)
	}
	if !strings.Contains(f.out.String(), "cancelled") {
		t.Errorf("output = %q, want cancelled", f.out.String())
	}
}

func TestRunDrainsAndRestores(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	code := Run(f.cfg)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr: %s)", code, f.errb.String())
	}
	want := "swapoff /dev/sda2,swapon /dev/sda2"
	if got := f.calls(); got != want {
		t.Errorf("calls = %q, want %q", got, want)
	}
	if f.sys.sys.SwapUsed != 0 {
		t.Errorf("swap used = %d, want 0", f.sys.sys.SwapUsed)
	}
	if len(f.sys.devices) != 1 || f.sys.devices[0].Filename != "/dev/sda2" {
		t.Errorf("devices = %+v, want /dev/sda2 restored", f.sys.devices)
	}
	if !strings.Contains(f.out.String(), "RECEIPT") {
		t.Errorf("output missing receipt: %s", f.out.String())
	}
}

func TestRunPartialDrainExecutesSelectedOnly(t *testing.T) {
	f := newRunFixture(6*gib, 8*gib)
	f.sys.devices = []proc.SwapDevice{
		{Filename: "/dev/sda2", Type: "partition", SizeBytes: 8 * gib, UsedBytes: 6 * gib, Priority: -2},
		{Filename: "/dev/sdb1", Type: "partition", SizeBytes: 8 * gib, UsedBytes: 5 * gib, Priority: 10},
	}
	f.sys.sys.SwapTotal = 16 * gib
	f.sys.sys.SwapUsed = 11 * gib
	f.sys.sys.SwapFree = 5 * gib
	f.cfg.JSON = true
	code := Run(f.cfg)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr: %s)", code, f.errb.String())
	}
	if got := f.calls(); got != "swapoff /dev/sda2,swapon /dev/sda2" {
		t.Errorf("calls = %q, want only the selected device", got)
	}
	var rec Receipt
	if err := json.Unmarshal(f.out.Bytes(), &rec); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, f.out.String())
	}
	if !rec.Partial || rec.SkippedForHeadroomBytes != 5*gib {
		t.Errorf("receipt = %+v, want partial with 5 GiB skipped", rec)
	}
	if len(rec.Devices) != 1 || rec.Devices[0].Filename != "/dev/sda2" {
		t.Errorf("devices = %+v, want only /dev/sda2", rec.Devices)
	}
	if len(f.sys.devices) != 2 {
		t.Errorf("active devices = %d, want 2", len(f.sys.devices))
	}
}

func TestRunAlreadyCleanDoesNoChurn(t *testing.T) {
	f := newRunFixture(0, 8*gib)
	f.sys.sys.SwapUsed = 0
	f.sys.devices[0].UsedBytes = 0
	code := Run(f.cfg)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("calls = %v, want none", f.sys.calls)
	}
	if !strings.Contains(f.out.String(), "already clean") {
		t.Errorf("output = %q, want already clean", f.out.String())
	}
}

func TestRunInfeasibleExecutesNothing(t *testing.T) {
	f := newRunFixture(4*gib, 2*gib)
	code := Run(f.cfg)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("calls = %v, want none", f.sys.calls)
	}
	if !strings.Contains(f.errb.String(), "blocked") {
		t.Errorf("stderr = %q, want blocked", f.errb.String())
	}
}

func TestRunZramOnlySkips(t *testing.T) {
	f := newRunFixture(2*gib, 8*gib)
	f.sys.devices = []proc.SwapDevice{{
		Filename: "/dev/zram0", Type: "partition", SizeBytes: 4 * gib, UsedBytes: 2 * gib, Priority: 100,
	}}
	code := Run(f.cfg)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("calls = %v, want none", f.sys.calls)
	}
	if !strings.Contains(f.out.String(), "zram only") {
		t.Errorf("output = %q, want zram note", f.out.String())
	}
}

func TestRunDropCaches(t *testing.T) {
	f := newRunFixture(0, 8*gib)
	f.sys.sys.SwapUsed = 0
	f.sys.devices[0].UsedBytes = 0
	f.cfg.DrainSwap = false
	f.cfg.DropCaches = true
	code := Run(f.cfg)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr: %s)", code, f.errb.String())
	}
	if got := f.calls(); got != "sync,drop_caches" {
		t.Errorf("calls = %q, want sync,drop_caches", got)
	}
	if f.sys.sys.PageCacheBytes() != 0 {
		t.Errorf("page cache = %d, want 0", f.sys.sys.PageCacheBytes())
	}
}

func TestRunDropCachesDryRunIsPure(t *testing.T) {
	f := newRunFixture(0, 8*gib)
	f.sys.sys.SwapUsed = 0
	f.sys.devices[0].UsedBytes = 0
	f.cfg.DrainSwap = false
	f.cfg.DropCaches = true
	f.cfg.DryRun = true
	code := Run(f.cfg)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("calls = %v, want none", f.sys.calls)
	}
}

func TestRunSwaponFailureLeavesCriticalAndExit3(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	f.sys.swapOnFailures["/dev/sda2"] = 99
	f.cfg.JSON = true
	code := Run(f.cfg)
	if code != 3 {
		t.Fatalf("code = %d, want 3", code)
	}
	if !strings.Contains(f.errb.String(), "CRITICAL") {
		t.Errorf("stderr = %q, want CRITICAL", f.errb.String())
	}
	var rec Receipt
	if err := json.Unmarshal(f.out.Bytes(), &rec); err != nil {
		t.Fatalf("stdout is not a receipt: %v\n%s", err, f.out.String())
	}
	if rec.Result != "failed" {
		t.Errorf("result = %q, want failed", rec.Result)
	}
	if len(rec.Devices) != 1 || rec.Devices[0].Restored {
		t.Errorf("devices = %+v, want one unrestored device", rec.Devices)
	}
	wantCalls := "swapoff /dev/sda2,swapon /dev/sda2,swapon /dev/sda2,swapon /dev/sda2"
	if got := f.calls(); got != wantCalls {
		t.Errorf("calls = %q, want %q", got, wantCalls)
	}
}

func TestRunSwapoffFailureStops(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	f.sys.swapOffErr["/dev/sda2"] = errors.New("device busy")
	code := Run(f.cfg)
	if code != 3 {
		t.Fatalf("code = %d, want 3", code)
	}
	if got := f.calls(); got != "swapoff /dev/sda2" {
		t.Errorf("calls = %q, want only swapoff", got)
	}
	if len(f.sys.devices) != 1 {
		t.Error("device should remain active after failed swapoff")
	}
}

func TestRunJSONDryRun(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	f.cfg.DryRun = true
	f.cfg.JSON = true
	code := Run(f.cfg)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	var rec Receipt
	if err := json.Unmarshal(f.out.Bytes(), &rec); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, f.out.String())
	}
	if rec.SchemaVersion != SchemaVersion {
		t.Errorf("schema = %d, want %d", rec.SchemaVersion, SchemaVersion)
	}
	if rec.Result != "dry-run" || !rec.Feasible {
		t.Errorf("receipt = %+v, want dry-run feasible", rec)
	}
	if len(f.sys.calls) != 0 {
		t.Errorf("calls = %v, want none", f.sys.calls)
	}
}

func TestRunAppliesTimeoutToRealSystem(t *testing.T) {
	rs := NewRealSystem()
	rs.Timeout = time.Second
	var out, errb bytes.Buffer
	code := Run(Config{
		Out: &out, Err: &errb, System: rs, DryRun: true,
		Timeout: 42 * time.Second, Reserve: gib, ReserveSet: true,
	})
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr: %s)", code, errb.String())
	}
	if rs.Timeout != 42*time.Second {
		t.Errorf("RealSystem.Timeout = %v, want %v", rs.Timeout, 42*time.Second)
	}
}

func TestExecuteDrainRestoresBeforeNextDevice(t *testing.T) {
	f := newRunFixture(4*gib, 10*gib)
	f.sys.devices = []proc.SwapDevice{
		{Filename: "/dev/sda2", Type: "partition", SizeBytes: 4 * gib, UsedBytes: 2 * gib, Priority: -2},
		{Filename: "/dev/sdb1", Type: "partition", SizeBytes: 4 * gib, UsedBytes: 2 * gib, Priority: 10},
	}
	f.sys.sys.SwapUsed = 4 * gib
	f.sys.sys.SwapTotal = 8 * gib
	f.sys.sys.SwapFree = 4 * gib
	plan := BuildPlan(f.sys.sys, f.sys.devices, 1*gib, false)
	rec := &Receipt{Devices: []DeviceReceipt{}}
	cfg := f.cfg
	code := executeDrain(context.Background(), cfg, plan, rec)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr: %s)", code, f.errb.String())
	}
	want := "swapoff /dev/sda2,swapon /dev/sda2,swapoff /dev/sdb1,swapon /dev/sdb1"
	if got := f.calls(); got != want {
		t.Errorf("calls = %q, want %q", got, want)
	}
	if len(rec.Devices) != 2 || !rec.Devices[0].Restored || !rec.Devices[1].Restored {
		t.Errorf("receipts = %+v, want both restored", rec.Devices)
	}
}

func TestExecuteDrainAbortsWhenHeadroomVanishes(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	plan := BuildPlan(f.sys.sys, f.sys.devices, 2*gib, false)
	f.sys.sys.MemAvail = 3 * gib
	rec := &Receipt{Devices: []DeviceReceipt{}}
	code := executeDrain(context.Background(), f.cfg, plan, rec)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("calls = %v, want none", f.sys.calls)
	}
	if !strings.Contains(f.errb.String(), "headroom vanished") {
		t.Errorf("stderr = %q, want headroom error", f.errb.String())
	}
}

func TestExecuteDrainSkipsDisappearedDevice(t *testing.T) {
	f := newRunFixture(4*gib, 8*gib)
	plan := BuildPlan(f.sys.sys, f.sys.devices, 2*gib, false)
	f.sys.devices = nil
	rec := &Receipt{Devices: []DeviceReceipt{}}
	code := executeDrain(context.Background(), f.cfg, plan, rec)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if len(f.sys.calls) != 0 {
		t.Fatalf("calls = %v, want none", f.sys.calls)
	}
	if !strings.Contains(f.out.String(), "disappeared") {
		t.Errorf("output = %q, want disappeared note", f.out.String())
	}
}
