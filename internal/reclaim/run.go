//go:build linux

package reclaim

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
	"github.com/jadmadi/swapaudit/internal/term"
)

// SchemaVersion is the reclaim receipt schema.
const SchemaVersion = 1

// System is the kernel boundary used by the planner and executor. It is an
// interface so the destructive contract can be proven with a fake.
type System interface {
	SystemSwap() model.SystemSwap
	Devices() ([]proc.SwapDevice, error)
	SwapOff(ctx context.Context, device string) error
	SwapOn(ctx context.Context, device string, priority int) error
	DropCaches(ctx context.Context) error
	Sync()
}

// RealSystem executes swapoff, swapon, and drop_caches on the host.
type RealSystem struct {
	SwapOffPath    string
	SwapOnPath     string
	DropCachesPath string
	Timeout        time.Duration
}

// NewRealSystem resolves the swap tools outside PATH, because /sbin is often
// absent from an unprivileged PATH.
func NewRealSystem() *RealSystem {
	return &RealSystem{
		SwapOffPath:    findTool("swapoff"),
		SwapOnPath:     findTool("swapon"),
		DropCachesPath: "/proc/sys/vm/drop_caches",
		Timeout:        5 * time.Minute,
	}
}

func findTool(name string) string {
	for _, dir := range []string{"/sbin", "/usr/sbin", "/bin", "/usr/bin"} {
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}

// SystemSwap reads /proc/meminfo and the /tmp statfs counters.
func (r *RealSystem) SystemSwap() model.SystemSwap {
	return proc.ReadSystemSwap()
}

// Devices reads the active swap areas from /proc/swaps.
func (r *RealSystem) Devices() ([]proc.SwapDevice, error) {
	return proc.ReadSwapDevices()
}

func (r *RealSystem) command(ctx context.Context, path string, args ...string) error {
	if path == "" {
		return errors.New("swap tool path is empty")
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(runCtx, path, args...).CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg != "" {
		return fmt.Errorf("%s %s: %w: %s", filepath.Base(path), strings.Join(args, " "), err, msg)
	}
	return fmt.Errorf("%s %s: %w", filepath.Base(path), strings.Join(args, " "), err)
}

// SwapOff disables one swap area.
func (r *RealSystem) SwapOff(ctx context.Context, device string) error {
	return r.command(ctx, r.SwapOffPath, device)
}

// SwapOn re-enables one swap area with its original priority.
func (r *RealSystem) SwapOn(ctx context.Context, device string, priority int) error {
	args := make([]string, 0, 3)
	if priority != -2 {
		args = append(args, "-p", strconv.Itoa(priority))
	}
	args = append(args, device)
	return r.command(ctx, r.SwapOnPath, args...)
}

// DropCaches writes 3 to /proc/sys/vm/drop_caches. Sync must be called first.
func (r *RealSystem) DropCaches(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if err := os.WriteFile(r.DropCachesPath, []byte("3\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", r.DropCachesPath, err)
	}
	return nil
}

// Sync flushes dirty pages so drop_caches can release them.
func (r *RealSystem) Sync() {
	syscall.Sync()
}

// Config injects every input, output, and side effect.
type Config struct {
	Out         io.Writer
	Err         io.Writer
	In          *bufio.Reader
	System      System
	Pressure    func() proc.MemoryPressure
	IsRoot      bool
	StdinIsTTY  bool
	DryRun      bool
	Yes         bool
	JSON        bool
	DrainSwap   bool
	DropCaches  bool
	IncludeZram bool
	Reserve     int64
	ReserveSet  bool
	Timeout     time.Duration
}

// Metrics is the measured memory state in a receipt.
type Metrics struct {
	SwapUsedBytes     int64   `json:"swap_used_bytes"`
	SwapTotalBytes    int64   `json:"swap_total_bytes"`
	MemAvailableBytes int64   `json:"mem_available_bytes"`
	MemFreeBytes      int64   `json:"mem_free_bytes"`
	PageCacheBytes    int64   `json:"page_cache_bytes"`
	PressureSome10    float64 `json:"pressure_some_avg10"`
	PressureFull10    float64 `json:"pressure_full_avg10"`
}

// DeviceReceipt records one device outcome.
type DeviceReceipt struct {
	Filename        string `json:"filename"`
	Type            string `json:"type"`
	Priority        int    `json:"priority"`
	UsedBeforeBytes int64  `json:"used_before_bytes"`
	Drained         bool   `json:"drained"`
	Restored        bool   `json:"restored"`
	Error           string `json:"error,omitempty"`
}

// CacheReceipt records the drop_caches outcome.
type CacheReceipt struct {
	BeforeBytes   int64 `json:"before_bytes"`
	AfterBytes    int64 `json:"after_bytes"`
	ReleasedBytes int64 `json:"released_bytes"`
	MemFreeBefore int64 `json:"mem_free_before"`
	MemFreeAfter  int64 `json:"mem_free_after"`
}

// Receipt is the machine-readable reclaim record.
type Receipt struct {
	SchemaVersion           int             `json:"schema_version"`
	Action                  string          `json:"action"`
	DryRun                  bool            `json:"dry_run"`
	Root                    bool            `json:"root"`
	Feasible                bool            `json:"feasible"`
	Partial                 bool            `json:"partial"`
	Blocked                 string          `json:"blocked,omitempty"`
	ReserveBytes            int64           `json:"reserve_bytes"`
	HeadroomBytes           int64           `json:"headroom_bytes"`
	AllHeadroomBytes        int64           `json:"all_headroom_bytes"`
	SkippedForHeadroomBytes int64           `json:"skipped_for_headroom_bytes"`
	Before                  Metrics         `json:"before"`
	After                   *Metrics        `json:"after,omitempty"`
	Devices                 []DeviceReceipt `json:"devices"`
	Caches                  *CacheReceipt   `json:"caches,omitempty"`
	Result                  string          `json:"result"`
	Errors                  []string        `json:"errors"`
}

func metrics(sys model.SystemSwap, pressure proc.MemoryPressure) Metrics {
	return Metrics{
		SwapUsedBytes:     sys.SwapUsed,
		SwapTotalBytes:    sys.SwapTotal,
		MemAvailableBytes: sys.MemAvail,
		MemFreeBytes:      sys.MemFree,
		PageCacheBytes:    sys.PageCacheBytes(),
		PressureSome10:    pressure.SomeAvg10,
		PressureFull10:    pressure.FullAvg10,
	}
}

func actionName(drain, drop bool) string {
	var parts []string
	if drain {
		parts = append(parts, "drain-swap")
	}
	if drop {
		parts = append(parts, "drop-caches")
	}
	return strings.Join(parts, ",")
}

// Run plans and optionally executes the reclaim. It returns a process exit
// code: 0 success, 2 precondition, 3 reclaim failure, 1 internal error.
func Run(cfg Config) int {
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	if cfg.Err == nil {
		cfg.Err = os.Stderr
	}
	if cfg.In == nil {
		cfg.In = bufio.NewReader(os.Stdin)
	}
	if cfg.System == nil {
		cfg.System = NewRealSystem()
	}
	if cfg.Pressure == nil {
		cfg.Pressure = proc.ReadMemoryPressure
	}
	if !cfg.DrainSwap && !cfg.DropCaches {
		cfg.DrainSwap = true
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Minute
	}
	if rs, ok := cfg.System.(*RealSystem); ok {
		rs.Timeout = cfg.Timeout
	}
	if !cfg.ReserveSet {
		sys := cfg.System.SystemSwap()
		cfg.Reserve = DefaultReserve(sys.MemTotal)
	}

	display := cfg
	if cfg.JSON {
		display.Out = io.Discard
	}

	sys := cfg.System.SystemSwap()
	if sys.MemTotal <= 0 {
		fmt.Fprintln(cfg.Err, "error: cannot read memory counters from /proc/meminfo")
		return 1
	}
	devices, err := cfg.System.Devices()
	if err != nil {
		fmt.Fprintf(cfg.Err, "error: cannot read /proc/swaps: %v\n", err)
		return 1
	}
	pressure := cfg.Pressure()
	plan := BuildPlan(sys, devices, cfg.Reserve, cfg.IncludeZram)

	rec := &Receipt{
		SchemaVersion:           SchemaVersion,
		Action:                  actionName(cfg.DrainSwap, cfg.DropCaches),
		DryRun:                  cfg.DryRun,
		Root:                    cfg.IsRoot,
		Feasible:                plan.Feasible,
		Partial:                 plan.Partial,
		Blocked:                 string(plan.BlockReason),
		ReserveBytes:            plan.Reserve,
		HeadroomBytes:           plan.SelectedHeadroom,
		AllHeadroomBytes:        plan.Headroom,
		SkippedForHeadroomBytes: plan.SkippedForHeadroom,
		Before:                  metrics(sys, pressure),
		Devices:                 []DeviceReceipt{},
		Errors:                  []string{},
	}
	if !cfg.JSON {
		printPlan(display.Out, plan, pressure, cfg.DropCaches)
	}

	drainRequested := cfg.DrainSwap
	dropRequested := cfg.DropCaches
	drainPossible := drainRequested && plan.Executable()

	if cfg.DryRun {
		if drainPossible {
			if plan.Partial {
				fmt.Fprintf(display.Out, "  Dry run     would drain %d of %d device(s), leaving %s in swap\n",
					plan.DrainableDevices, plan.DrainableDevices+plan.SkippedDevices,
					term.FormatBytes(plan.SkippedForHeadroom))
			} else {
				fmt.Fprintf(display.Out, "  Dry run     would drain %d device(s); no commands executed\n", plan.DrainableDevices)
			}
		}
		if dropRequested {
			fmt.Fprintf(display.Out, "  Dry run     would sync and write 3 to /proc/sys/vm/drop_caches\n")
		}
		rec.Result = "dry-run"
		return finish(cfg, rec, 0)
	}

	if drainRequested && plan.BlockReason == BlockInsufficient {
		printBlock(cfg.Err, plan)
		rec.Result = "blocked"
		rec.Errors = append(rec.Errors, string(plan.BlockReason))
		return finish(cfg, rec, 2)
	}
	if !drainPossible && !dropRequested {
		fmt.Fprintf(display.Out, "  Result      nothing to do\n")
		rec.Result = "nothing-to-do"
		return finish(cfg, rec, 0)
	}

	if !cfg.IsRoot {
		fmt.Fprintf(cfg.Err, "error: swapoff, swapon, and drop_caches require root\n")
		fmt.Fprintf(cfg.Err, "hint: sudo swapaudit reclaim %s\n", suggestedFlags(cfg))
		rec.Result = "not-root"
		rec.Errors = append(rec.Errors, "not-root")
		return finish(cfg, rec, 2)
	}

	if !cfg.Yes {
		if !cfg.StdinIsTTY {
			fmt.Fprintln(cfg.Err, "error: reclaim requires --yes when stdin is not a terminal")
			rec.Result = "confirmation-required"
			rec.Errors = append(rec.Errors, "confirmation-required")
			return finish(cfg, rec, 2)
		}
		if !confirm(display, plan, dropRequested) {
			fmt.Fprintln(display.Out, "  Result      cancelled; nothing was changed")
			rec.Result = "cancelled"
			return finish(cfg, rec, 0)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	code := 0
	if drainPossible {
		code = executeDrain(ctx, display, plan, rec)
	} else if drainRequested && plan.AlreadyClean {
		fmt.Fprintln(display.Out, "  Result      swap is already clean; no swapoff/swapon churn")
	}
	if code != 0 {
		rec.Result = "failed"
		return finish(cfg, rec, code)
	}
	if dropRequested {
		if ctx.Err() != nil {
			rec.Errors = append(rec.Errors, "interrupted")
			rec.Result = "failed"
			return finish(cfg, rec, 3)
		}
		if dropCode := executeDropCaches(ctx, display, rec); dropCode != 0 {
			code = dropCode
		}
	}

	after := cfg.System.SystemSwap()
	m := metrics(after, cfg.Pressure())
	rec.After = &m
	if code == 0 {
		rec.Result = "clean"
	} else {
		rec.Result = "failed"
	}
	printReceipt(display.Out, rec)
	return finish(cfg, rec, code)
}

func finish(cfg Config, rec *Receipt, code int) int {
	if cfg.JSON {
		enc := json.NewEncoder(cfg.Out)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rec)
	}
	return code
}

func suggestedFlags(cfg Config) string {
	flags := make([]string, 0, 3)
	if cfg.DrainSwap {
		flags = append(flags, "--drain-swap")
	}
	if cfg.DropCaches {
		flags = append(flags, "--drop-caches")
	}
	if cfg.Yes {
		flags = append(flags, "--yes")
	}
	return strings.Join(flags, " ")
}

func confirm(cfg Config, plan Plan, drop bool) bool {
	var parts []string
	if plan.Executable() {
		parts = append(parts, fmt.Sprintf("drain %d of %d swap device(s), %s, into RAM",
			plan.DrainableDevices, plan.DrainableDevices+plan.SkippedDevices,
			term.FormatBytes(plan.DrainableUsed)))
		if plan.Partial {
			parts = append(parts, fmt.Sprintf("leaving %s in swap", term.FormatBytes(plan.SkippedForHeadroom)))
		}
	}
	if drop {
		parts = append(parts, fmt.Sprintf("drop %s of page cache", term.FormatBytes(plan.System.PageCacheBytes())))
	}
	fmt.Fprintf(cfg.Out, "\nThis will %s.\nSwap is briefly disabled per device. [y/N]: ", strings.Join(parts, " and "))
	line, err := cfg.In.ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func printPlan(out io.Writer, p Plan, pressure proc.MemoryPressure, dropRequested bool) {
	fmt.Fprintln(out, "SYSTEM RECLAIM")
	fmt.Fprintln(out, strings.Repeat("-", 66))
	fmt.Fprintf(out, "  Swap         %s used / %s total across %d device(s)\n",
		term.FormatBytes(p.SwapUsed), term.FormatBytes(p.System.SwapTotal), len(p.Devices))
	for _, dp := range p.Devices {
		d := dp.Device
		line := fmt.Sprintf("  Device       %-22s %-9s %s used / %s",
			truncate(d.Filename, 22), d.Type, term.FormatBytes(d.UsedBytes), term.FormatBytes(d.SizeBytes))
		switch dp.SkipReason {
		case SkipZram:
			line += "  SKIP zram"
		case SkipEmpty:
			line += "  already empty"
		case SkipHeadroom:
			line += "  SKIP headroom"
		}
		fmt.Fprintln(out, line)
	}
	fmt.Fprintf(out, "  RAM          %s available / %s total\n",
		term.FormatBytes(p.System.MemAvail), term.FormatBytes(p.System.MemTotal))
	reserveNote := ""
	if p.Reserve == DefaultReserve(p.System.MemTotal) {
		reserveNote = " (10% of RAM, floor 512.0 MB)"
	}
	fmt.Fprintf(out, "  Reserve      %s%s\n", term.FormatBytes(p.Reserve), reserveNote)
	if p.DrainableDevices > 0 {
		when := "after drain"
		if p.Partial {
			when = "after selected drain"
		}
		fmt.Fprintf(out, "  Headroom     %s %s\n", term.FormatBytes(p.SelectedHeadroom), when)
	}
	if pressure.Available {
		fmt.Fprintf(out, "  Pressure     some %.2f / full %.2f (avg10)\n", pressure.SomeAvg10, pressure.FullAvg10)
	}
	fmt.Fprintf(out, "  Page cache   %s reclaimable with --drop-caches (root)\n",
		term.FormatBytes(p.System.PageCacheBytes()))
	if dropRequested {
		fmt.Fprintf(out, "  Dirty        %s dirty, %s writeback\n",
			term.FormatBytes(p.System.Dirty), term.FormatBytes(p.System.Writeback))
	}

	switch p.BlockReason {
	case BlockNoSwap:
		fmt.Fprintln(out, "  Plan         no swap configured; nothing to drain")
	case BlockAlreadyClean:
		fmt.Fprintln(out, "  Plan         swap is already clean; no swapoff/swapon churn")
	case BlockZramOnly:
		fmt.Fprintln(out, "  Plan         zram swap only; draining increases RAM use")
		fmt.Fprintln(out, "  Next         add --include-zram only if you accept that trade-off")
	case BlockInsufficient:
		fmt.Fprintf(out, "  Plan         BLOCKED: need %s more available RAM\n", term.FormatBytes(-p.Headroom))
		fmt.Fprintf(out, "               used swap %s + reserve %s > available %s\n",
			term.FormatBytes(p.AllDrainableUsed), term.FormatBytes(p.Reserve), term.FormatBytes(p.System.MemAvail))
		fmt.Fprintln(out, "  Next         close applications, then: swapaudit reclaim --dry-run")
	default:
		if p.Partial {
			fmt.Fprintf(out, "  Plan         drain %d of %d device(s), %s -> RAM, partial\n",
				p.DrainableDevices, p.DrainableDevices+p.SkippedDevices, term.FormatBytes(p.DrainableUsed))
			fmt.Fprintf(out, "               %s remains in swap (insufficient headroom)\n",
				term.FormatBytes(p.SkippedForHeadroom))
		} else {
			fmt.Fprintf(out, "  Plan         drain %d device(s), %s -> RAM, feasible\n",
				p.DrainableDevices, term.FormatBytes(p.DrainableUsed))
		}
		for _, dp := range p.Devices {
			if dp.Drain {
				fmt.Fprintf(out, "  Command      swapoff %s && swapon%s %s\n",
					dp.Device.Filename, priorityArg(dp.Device.Priority), dp.Device.Filename)
			}
		}
	}
	fmt.Fprintln(out)
}

func priorityArg(priority int) string {
	if priority == -2 {
		return ""
	}
	return " -p " + strconv.Itoa(priority)
}

func printBlock(errOut io.Writer, p Plan) {
	fmt.Fprintf(errOut, "error: swap drain blocked: need %s more available RAM\n", term.FormatBytes(-p.Headroom))
	fmt.Fprintf(errOut, "       used swap %s + reserve %s > available %s\n",
		term.FormatBytes(p.AllDrainableUsed), term.FormatBytes(p.Reserve), term.FormatBytes(p.System.MemAvail))
	fmt.Fprintln(errOut, "       no swapoff was executed. Close applications and retry.")
}

func truncate(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

func findDevice(devices []proc.SwapDevice, name string) (proc.SwapDevice, bool) {
	for _, d := range devices {
		if d.Filename == name {
			return d, true
		}
	}
	return proc.SwapDevice{}, false
}

// executeDrain runs swapoff -> verify -> swapon -> verify per device. Every
// disabled device is restored on any error, cancellation, or signal.
func executeDrain(ctx context.Context, cfg Config, plan Plan, rec *Receipt) (code int) {
	disabled := map[string]proc.SwapDevice{}

	setDevice := func(dev proc.SwapDevice, drained, restored bool, errText string) {
		for i := range rec.Devices {
			if rec.Devices[i].Filename == dev.Filename {
				rec.Devices[i].Drained = drained
				rec.Devices[i].Restored = restored
				rec.Devices[i].Error = errText
				return
			}
		}
		rec.Devices = append(rec.Devices, DeviceReceipt{
			Filename: dev.Filename, Type: dev.Type, Priority: dev.Priority,
			UsedBeforeBytes: dev.UsedBytes, Drained: drained, Restored: restored, Error: errText,
		})
	}

	restore := func(current int) int {
		for name, dev := range disabled {
			if active, err := cfg.System.Devices(); err == nil {
				if _, on := findDevice(active, name); on {
					delete(disabled, name)
					setDevice(dev, true, true, "")
					fmt.Fprintf(cfg.Out, "  Restored     %s (verified active)\n", name)
					continue
				}
			}
			if err := cfg.System.SwapOn(context.Background(), name, dev.Priority); err != nil {
				if active, readErr := cfg.System.Devices(); readErr == nil {
					if _, on := findDevice(active, name); on {
						delete(disabled, name)
						setDevice(dev, true, true, "")
						fmt.Fprintf(cfg.Out, "  Restored     %s (verified active)\n", name)
						continue
					}
				}
				fmt.Fprintf(cfg.Err, "CRITICAL: %s is still disabled: %v\n", name, err)
				fmt.Fprintf(cfg.Err, "recover with: sudo swapon%s %s\n", priorityArg(dev.Priority), name)
				rec.Errors = append(rec.Errors, name+": "+err.Error())
				setDevice(dev, true, false, err.Error())
				current = 3
				continue
			}
			if active, err := cfg.System.Devices(); err == nil {
				if _, on := findDevice(active, name); !on {
					msg := "not-active-after-restore"
					fmt.Fprintf(cfg.Err, "CRITICAL: %s is not active after swapon\n", name)
					fmt.Fprintf(cfg.Err, "recover with: sudo swapon%s %s\n", priorityArg(dev.Priority), name)
					rec.Errors = append(rec.Errors, name+": "+msg)
					setDevice(dev, true, false, msg)
					current = 3
					continue
				}
			}
			delete(disabled, name)
			setDevice(dev, true, true, "")
			fmt.Fprintf(cfg.Out, "  Restored     %s\n", name)
		}
		return current
	}
	defer func() { code = restore(code) }()

	total := plan.DrainableDevices
	index := 0
	for _, dp := range plan.Devices {
		if !dp.Drain {
			continue
		}
		index++
		if ctx.Err() != nil {
			fmt.Fprintln(cfg.Err, "error: interrupted; restoring swap devices")
			rec.Errors = append(rec.Errors, "interrupted")
			return 3
		}

		current, err := cfg.System.Devices()
		if err != nil {
			fmt.Fprintf(cfg.Err, "error: cannot re-read /proc/swaps: %v\n", err)
			rec.Errors = append(rec.Errors, err.Error())
			return 3
		}
		dev, ok := findDevice(current, dp.Device.Filename)
		if !ok {
			fmt.Fprintf(cfg.Out, "  [%d/%d] %s disappeared; skipped\n", index, total, dp.Device.Filename)
			continue
		}
		if dev.UsedBytes <= 0 {
			fmt.Fprintf(cfg.Out, "  [%d/%d] %s is already empty; skipped\n", index, total, dev.Filename)
			continue
		}

		sys := cfg.System.SystemSwap()
		if sys.MemAvail < dev.UsedBytes+plan.Reserve {
			fmt.Fprintf(cfg.Err, "error: headroom vanished before %s: need %s, have %s\n",
				dev.Filename, term.FormatBytes(dev.UsedBytes+plan.Reserve), term.FormatBytes(sys.MemAvail))
			rec.Errors = append(rec.Errors, "headroom-vanished:"+dev.Filename)
			return 2
		}

		fmt.Fprintf(cfg.Out, "  [%d/%d] swapoff %s ... ", index, total, dev.Filename)
		if err := cfg.System.SwapOff(ctx, dev.Filename); err != nil {
			fmt.Fprintln(cfg.Out, "failed")
			fmt.Fprintf(cfg.Err, "error: swapoff %s: %v\n", dev.Filename, err)
			rec.Errors = append(rec.Errors, err.Error())
			if active, readErr := cfg.System.Devices(); readErr == nil {
				if _, stillOn := findDevice(active, dev.Filename); !stillOn {
					disabled[dev.Filename] = dev
					setDevice(dev, true, false, err.Error())
				}
			}
			return 3
		}
		fmt.Fprintln(cfg.Out, "ok")
		disabled[dev.Filename] = dev

		active, err := cfg.System.Devices()
		if err != nil {
			fmt.Fprintf(cfg.Err, "error: cannot verify swapoff for %s: %v\n", dev.Filename, err)
			rec.Errors = append(rec.Errors, err.Error())
			setDevice(dev, true, false, err.Error())
			return 3
		}
		if _, stillOn := findDevice(active, dev.Filename); stillOn {
			fmt.Fprintf(cfg.Err, "error: swapoff returned success but %s is still active\n", dev.Filename)
			rec.Errors = append(rec.Errors, "still-active:"+dev.Filename)
			delete(disabled, dev.Filename)
			setDevice(dev, false, true, "still-active")
			return 3
		}

		fmt.Fprintf(cfg.Out, "  [%d/%d] swapon%s %s ... ", index, total, priorityArg(dev.Priority), dev.Filename)
		err = cfg.System.SwapOn(ctx, dev.Filename, dev.Priority)
		if err != nil {
			if active, readErr := cfg.System.Devices(); readErr == nil {
				if _, on := findDevice(active, dev.Filename); on {
					err = nil
				}
			}
		}
		if err != nil {
			fmt.Fprintln(cfg.Out, "failed; retrying")
			err = cfg.System.SwapOn(ctx, dev.Filename, dev.Priority)
			if err != nil {
				if active, readErr := cfg.System.Devices(); readErr == nil {
					if _, on := findDevice(active, dev.Filename); on {
						err = nil
					}
				}
			}
		}
		if err != nil {
			fmt.Fprintln(cfg.Out, "FAILED")
			fmt.Fprintf(cfg.Err, "CRITICAL: %s is disabled: %v\n", dev.Filename, err)
			fmt.Fprintf(cfg.Err, "recover with: sudo swapon%s %s\n", priorityArg(dev.Priority), dev.Filename)
			rec.Errors = append(rec.Errors, dev.Filename+": "+err.Error())
			setDevice(dev, true, false, err.Error())
			return 3
		}
		fmt.Fprintln(cfg.Out, "ok")

		active, err = cfg.System.Devices()
		if err != nil {
			fmt.Fprintf(cfg.Err, "error: cannot verify swapon for %s: %v\n", dev.Filename, err)
			rec.Errors = append(rec.Errors, err.Error())
			setDevice(dev, true, false, err.Error())
			return 3
		}
		reenabled, on := findDevice(active, dev.Filename)
		if !on {
			err = cfg.System.SwapOn(ctx, dev.Filename, dev.Priority)
			if err != nil {
				fmt.Fprintf(cfg.Err, "CRITICAL: %s is not active after swapon: %v\n", dev.Filename, err)
				rec.Errors = append(rec.Errors, dev.Filename+": "+err.Error())
				setDevice(dev, true, false, err.Error())
				return 3
			}
			active, err = cfg.System.Devices()
			if err != nil {
				fmt.Fprintf(cfg.Err, "error: cannot verify swapon for %s: %v\n", dev.Filename, err)
				rec.Errors = append(rec.Errors, err.Error())
				setDevice(dev, true, false, err.Error())
				return 3
			}
			reenabled, on = findDevice(active, dev.Filename)
			if !on {
				msg := "not-active-after-swapon"
				fmt.Fprintf(cfg.Err, "CRITICAL: %s is not active after swapon\n", dev.Filename)
				rec.Errors = append(rec.Errors, dev.Filename+": "+msg)
				setDevice(dev, true, false, msg)
				return 3
			}
		}
		delete(disabled, dev.Filename)
		if reenabled.SizeBytes != dev.SizeBytes {
			fmt.Fprintf(cfg.Err, "warning: %s size changed from %s to %s\n",
				dev.Filename, term.FormatBytes(dev.SizeBytes), term.FormatBytes(reenabled.SizeBytes))
		}
		if reenabled.Priority != dev.Priority {
			fmt.Fprintf(cfg.Err, "warning: %s priority changed from %d to %d\n",
				dev.Filename, dev.Priority, reenabled.Priority)
		}
		setDevice(dev, true, true, "")
	}

	return 0
}

func executeDropCaches(ctx context.Context, cfg Config, rec *Receipt) int {
	before := cfg.System.SystemSwap()
	fmt.Fprintln(cfg.Out, "  sync + drop_caches ... ")
	cfg.System.Sync()
	if err := cfg.System.DropCaches(ctx); err != nil {
		fmt.Fprintf(cfg.Err, "error: drop_caches: %v\n", err)
		rec.Errors = append(rec.Errors, err.Error())
		return 3
	}
	after := cfg.System.SystemSwap()
	released := before.PageCacheBytes() - after.PageCacheBytes()
	if released < 0 {
		released = 0
	}
	rec.Caches = &CacheReceipt{
		BeforeBytes:   before.PageCacheBytes(),
		AfterBytes:    after.PageCacheBytes(),
		ReleasedBytes: released,
		MemFreeBefore: before.MemFree,
		MemFreeAfter:  after.MemFree,
	}
	fmt.Fprintln(cfg.Out, "ok")
	return 0
}

func printReceipt(out io.Writer, rec *Receipt) {
	fmt.Fprintln(out, "RECEIPT")
	fmt.Fprintln(out, strings.Repeat("-", 66))
	before := rec.Before
	after := rec.Before
	if rec.After != nil {
		after = *rec.After
	}
	fmt.Fprintf(out, "  Swap         %s -> %s (delta %s)\n",
		term.FormatBytes(before.SwapUsedBytes), term.FormatBytes(after.SwapUsedBytes),
		signedBytes(after.SwapUsedBytes-before.SwapUsedBytes))
	if rec.Partial {
		fmt.Fprintf(out, "  Partial      %s left in swap (insufficient headroom)\n",
			term.FormatBytes(rec.SkippedForHeadroomBytes))
	}
	fmt.Fprintf(out, "  RAM avail    %s -> %s\n",
		term.FormatBytes(before.MemAvailableBytes), term.FormatBytes(after.MemAvailableBytes))
	if rec.Caches != nil {
		fmt.Fprintf(out, "  Page cache   %s -> %s (%s released)\n",
			term.FormatBytes(rec.Caches.BeforeBytes), term.FormatBytes(rec.Caches.AfterBytes),
			term.FormatBytes(rec.Caches.ReleasedBytes))
		fmt.Fprintf(out, "  MemFree      %s -> %s\n",
			term.FormatBytes(rec.Caches.MemFreeBefore), term.FormatBytes(rec.Caches.MemFreeAfter))
	}
	for _, d := range rec.Devices {
		state := "restored"
		if !d.Restored {
			state = "FAILED"
		}
		fmt.Fprintf(out, "  Device       %s: drained %s, %s\n",
			d.Filename, term.FormatBytes(d.UsedBeforeBytes), state)
	}
	if len(rec.Errors) > 0 {
		fmt.Fprintf(out, "  Errors       %d (see stderr)\n", len(rec.Errors))
	}
	fmt.Fprintf(out, "  Result       %s\n", rec.Result)
}

func signedBytes(v int64) string {
	if v == 0 {
		return "0 B"
	}
	sign := "+"
	if v < 0 {
		sign = "-"
		v = -v
	}
	return sign + term.FormatBytes(v)
}
