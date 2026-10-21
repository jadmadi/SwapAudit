//go:build linux

package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jadmadi/swapaudit/internal/cache"
	"github.com/jadmadi/swapaudit/internal/cleaner"
	"github.com/jadmadi/swapaudit/internal/live"
	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/notify"
	"github.com/jadmadi/swapaudit/internal/picker"
	"github.com/jadmadi/swapaudit/internal/proc"
	"github.com/jadmadi/swapaudit/internal/snapshot"
	"github.com/jadmadi/swapaudit/internal/term"
	"github.com/jadmadi/swapaudit/internal/triage"
	"github.com/jadmadi/swapaudit/internal/updater"
	"github.com/jadmadi/swapaudit/internal/version"
	"github.com/jadmadi/swapaudit/internal/watch"
)

// stringSliceFlag implements flag.Value to allow repeatable string flags.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(val string) error {
	*s = append(*s, val)
	return nil
}

// CollectSwapItems aggregates stale /tmp caches and processes with VmSwap > 0.
func CollectSwapItems(now time.Time, uptimeSec float64, maxAge, idleThreshold time.Duration, tmpPatterns []string) []model.SwapItem {
	var items []model.SwapItem
	uptimeMs := uptimeSec * 1000.0
	bootTime := now.Add(-time.Duration(uptimeSec * float64(time.Second)))

	items = append(items, cache.ScanTmpDirectories(now, maxAge, tmpPatterns)...)

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return items
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}

		swapBytes, ppid := proc.ReadProcStatus(pid)
		if swapBytes <= 0 {
			continue
		}

		comm := proc.ReadComm(pid)
		argv := proc.ReadCmdlineArgs(pid)
		cmd := proc.FormatCmdline(argv, comm)
		startTicks := proc.ReadStartTicks(pid)
		age := proc.CalcProcessAge(startTicks, bootTime)
		lastActive := proc.ReadProcessLastActive(pid, uptimeMs)
		lastActiveAt := now.Add(-lastActive)
		isOrphaned := proc.IsProcessOrphaned(ppid)

		cat, cleanable := triage.CategorizeProcess(comm, argv, pid, ppid, isOrphaned, lastActive, idleThreshold)

		dispName := comm
		hasConv := false
		for i, arg := range argv {
			if arg == "--conversation" && i+1 < len(argv) {
				targetId := argv[i+1]
				if runes := []rune(targetId); len(runes) > 8 {
					targetId = string(runes[:8])
				}
				dispName = fmt.Sprintf("agy [%s]", targetId)
				hasConv = true
				break
			}
		}
		if !hasConv {
			for _, arg := range argv {
				if arg == "chrome-devtools-mcp" || filepath.Base(arg) == "chrome-devtools-mcp" {
					dispName = "chrome-devtools"
					break
				}
			}
		}

		items = append(items, model.SwapItem{
			PID:          pid,
			StartTicks:   startTicks,
			Name:         dispName,
			Command:      cmd,
			Category:     cat,
			SwapBytes:    swapBytes,
			Age:          age,
			LastActive:   lastActive,
			LastActiveAt: lastActiveAt,
			Cleanable:    cleanable,
			Count:        1,
		})
	}

	return items
}

// executeCleanup is a seam so mode tests can prove read-only paths never reach
// the destructive executor.
var executeCleanup = cleaner.ExecuteCleanup

type modeFlags struct {
	json          bool
	noCommand     bool
	dryRun        bool
	yes           bool
	cleanup       bool
	interactive   bool
	noInteractive bool
}

// validateModes rejects incompatible flag combinations before any scan.
func validateModes(f modeFlags) error {
	if f.interactive && f.noInteractive {
		return errors.New("--interactive and --no-interactive are mutually exclusive")
	}
	if f.json {
		if f.cleanup {
			return errors.New("--json is read-only and cannot be combined with cleanup flags")
		}
		if f.dryRun {
			return errors.New("--json is read-only and cannot be combined with --dry-run")
		}
		if f.yes {
			return errors.New("--json is read-only and cannot be combined with --yes")
		}
		if f.interactive {
			return errors.New("--json cannot be combined with --interactive")
		}
	}
	if f.interactive {
		if f.cleanup {
			return errors.New("--interactive cannot be combined with cleanup flags")
		}
		if f.yes {
			return errors.New("--interactive cannot be combined with --yes")
		}
	}
	if f.noCommand && !f.json {
		return errors.New("--no-command requires --json")
	}
	return nil
}

// procStatusPath is the JSON preflight target; a seam for hard-failure tests.
var procStatusPath = "/proc/self/status"

// runJSON performs the read-only scan and writes exactly one document.
func runJSON(noCommand bool, maxAge, idleThreshold time.Duration, customPatterns []string) int {
	if _, err := os.Stat(procStatusPath); err != nil {
		if werr := snapshot.WriteError(os.Stdout, "proc_unavailable", "cannot read /proc/self/status"); werr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", werr)
		}
		return 1
	}

	now := time.Now()
	uptimeSec := proc.ReadUptime()
	sysSwap := proc.ReadSystemSwap()
	tmpPatterns := append(append([]string{}, cache.DefaultTmpPatterns...), customPatterns...)
	items := CollectSwapItems(now, uptimeSec, maxAge, idleThreshold, tmpPatterns)
	doc := snapshot.BuildSnapshot(now, uptimeSec, sysSwap, items, maxAge, idleThreshold, noCommand)
	if err := snapshot.Write(os.Stdout, doc); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// validateSampleFlags enforces the bounded sampling contract.
func validateSampleFlags(once, stream bool, interval, window, tmpInterval, duration time.Duration) error {
	if once && stream {
		return errors.New("--once and --stream are mutually exclusive")
	}
	if interval < 100*time.Millisecond || interval > 60*time.Second {
		return errors.New("--interval must be between 100ms and 60s")
	}
	if window < time.Second || window > time.Hour {
		return errors.New("--window must be between 1s and 1h")
	}
	if tmpInterval < time.Second || tmpInterval > 10*time.Minute {
		return errors.New("--tmp-interval must be between 1s and 10m")
	}
	if duration < 0 {
		return errors.New("--duration cannot be negative")
	}
	if window/interval > 600 {
		return errors.New("window/interval exceeds 600 samples; increase --interval")
	}
	return nil
}

func runSample(args []string) int {
	fs := flag.NewFlagSet("swapaudit sample", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		once        = fs.Bool("once", false, "Emit one JSON document and exit")
		stream      = fs.Bool("stream", false, "Emit one NDJSON line per tick")
		interval    = fs.Duration("interval", time.Second, "Fast-lane cadence (100ms..60s)")
		window      = fs.Duration("window", time.Minute, "Estimator window (1s..1h)")
		tmpInterval = fs.Duration("tmp-interval", 30*time.Second, "Slow-lane cadence (1s..10m)")
		duration    = fs.Duration("duration", 0, "Stop after this duration (0 = until signal)")
		record      = fs.String("record", "", "Write raw NDJSON samples to path")
		noCommand   = fs.Bool("no-command", false, "Omit command lines from the once snapshot")
		_           = fs.Bool("json", false, "Accepted no-op; sample is always machine-readable")
	)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printSampleUsage(os.Stdout)
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	if err := validateSampleFlags(*once, *stream, *interval, *window, *tmpInterval, *duration); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	modeOnce := !*stream
	if *once {
		modeOnce = true
	}
	if !*once && !*stream && *record != "" {
		modeOnce = false
	}

	const maxAge, idleThreshold = 12 * time.Hour, 2 * time.Hour
	patterns := cache.DefaultTmpPatterns
	cfg := live.Config{
		Interval: *interval, Window: *window, TmpInterval: *tmpInterval,
		Duration: *duration, RecordPath: *record, NoCommand: *noCommand,
		Once: modeOnce, Out: os.Stdout,
	}
	if modeOnce {
		now := time.Now()
		uptime := proc.ReadUptime()
		sys := proc.ReadSystemSwap()
		items := CollectSwapItems(now, uptime, maxAge, idleThreshold, patterns)
		cfg.ScanFast = live.NewFixedScanner(items, sys).Scan
		cfg.ScanTmp = func() []model.SwapItem { return items }
		cfg.Snapshot = func(at time.Time) snapshot.Snapshot {
			return snapshot.BuildSnapshot(at, uptime, sys, items, maxAge, idleThreshold, *noCommand)
		}
	} else {
		cfg.ScanFast = live.NewScanner().Scan
		cfg.ScanTmp = func() []model.SwapItem {
			return cache.ScanTmpDirectories(time.Now(), maxAge, patterns)
		}
	}
	if err := live.Run(context.Background(), cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func printSampleUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: swapaudit sample [--once | --stream] [FLAGS]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Headless sampler with rates, attribution, and a gated ETA.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  --once                 Emit one JSON document and exit (default)")
	fmt.Fprintln(w, "  --stream               Emit one NDJSON sample per tick")
	fmt.Fprintln(w, "  --interval <duration>  Fast-lane cadence (default 1s, range 100ms..60s)")
	fmt.Fprintln(w, "  --window <duration>    Estimator window (default 60s, range 1s..1h)")
	fmt.Fprintln(w, "  --tmp-interval <dur>   Slow-lane cadence (default 30s, range 1s..10m)")
	fmt.Fprintln(w, "  --duration <duration>  Stop after this duration (0 = until signal)")
	fmt.Fprintln(w, "  --record <path>        Write raw NDJSON samples with mode 0600")
	fmt.Fprintln(w, "  --no-command           Omit command lines from the once snapshot")
	fmt.Fprintln(w, "  --json                 Accepted no-op; sample is always machine-readable")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "EXAMPLES:")
	fmt.Fprintln(w, "  swapaudit sample --once | jq .")
	fmt.Fprintln(w, "  swapaudit sample --stream --interval=1s --record=/tmp/trace.ndjson")
}

type watchOptions struct {
	eta, sustain, cooldown, rearmETA                     time.Duration
	pressure, rearmPressure                              float64
	maxPerHour                                           int
	resource, notifyMode                                 string
	notifyCmd                                            []string
	notifyRecovery, testNotify, quiet, json              bool
	statusEvery, interval, window, tmpInterval, duration time.Duration
}

func validateWatchOptions(o watchOptions) error {
	if err := validateSampleFlags(false, false, o.interval, o.window, o.tmpInterval, o.duration); err != nil {
		return err
	}
	switch o.notifyMode {
	case "auto", "desktop", "osc", "stderr", "command":
	default:
		return errors.New("--notify must be one of auto, desktop, osc, stderr, command")
	}
	if o.eta <= 0 || o.eta > 24*time.Hour {
		return errors.New("--eta must be between 1s and 24h")
	}
	if o.pressure < 1 || o.pressure > 100 {
		return errors.New("--pressure must be between 1 and 100")
	}
	if o.rearmETA <= o.eta {
		return errors.New("--rearm-eta must be greater than --eta")
	}
	if o.rearmPressure >= o.pressure {
		return errors.New("--rearm-pressure must be less than --pressure")
	}
	if o.sustain < o.interval {
		return errors.New("--sustain must be at least one interval")
	}
	if o.cooldown < 0 {
		return errors.New("--cooldown cannot be negative")
	}
	if o.maxPerHour < 1 {
		return errors.New("--max-per-hour must be at least 1")
	}
	if o.statusEvery < 0 {
		return errors.New("--status-every cannot be negative")
	}
	if len(o.notifyCmd) > 0 && o.notifyMode != "command" {
		return errors.New("--notify-cmd requires --notify=command")
	}
	if o.json && o.notifyMode == "command" {
		return errors.New("--json cannot be combined with --notify=command")
	}
	if o.notifyMode == "command" && len(o.notifyCmd) == 0 {
		return errors.New("--notify=command requires --notify-cmd")
	}
	if o.resource != "both" && o.resource != "swap" && o.resource != "tmpfs" {
		return errors.New("--resource must be both, swap, or tmpfs")
	}
	return nil
}

func runWatch(args []string) int {
	fs := flag.NewFlagSet("swapaudit watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o watchOptions
	var cmdArgs stringSliceFlag
	fs.DurationVar(&o.eta, "eta", 15*time.Minute, "Alert when the ETA is at or below this duration")
	fs.Float64Var(&o.pressure, "pressure", 90, "Fallback pressure threshold (percent)")
	fs.DurationVar(&o.sustain, "sustain", 10*time.Second, "Continuous danger before alerting")
	fs.DurationVar(&o.cooldown, "cooldown", 10*time.Minute, "Minimum time between incidents")
	fs.IntVar(&o.maxPerHour, "max-per-hour", 4, "Maximum alerts per resource per rolling hour")
	fs.DurationVar(&o.rearmETA, "rearm-eta", 20*time.Minute, "ETA required to re-arm")
	fs.Float64Var(&o.rearmPressure, "rearm-pressure", 85, "Pressure required to re-arm")
	fs.StringVar(&o.resource, "resource", "both", "Resource to watch: both, swap, or tmpfs")
	fs.StringVar(&o.notifyMode, "notify", "auto", "Transport: auto, desktop, osc, stderr, command")
	fs.Var(&cmdArgs, "notify-cmd", "Command argv element for --notify=command (repeatable)")
	fs.BoolVar(&o.notifyRecovery, "notify-recovery", false, "Notify when a resource recovers")
	fs.BoolVar(&o.testNotify, "test-notify", false, "Send one probe notification and exit")
	fs.DurationVar(&o.statusEvery, "status-every", 10*time.Second, "Status line cadence (0 disables)")
	fs.BoolVar(&o.quiet, "quiet", false, "Suppress the status line")
	fs.BoolVar(&o.json, "json", false, "Emit event lines to stdout and send nothing")
	fs.DurationVar(&o.interval, "interval", time.Second, "Fast-lane cadence")
	fs.DurationVar(&o.window, "window", time.Minute, "Estimator window")
	fs.DurationVar(&o.tmpInterval, "tmp-interval", 30*time.Second, "Slow-lane cadence")
	fs.DurationVar(&o.duration, "duration", 0, "Stop after this duration (0 = until signal)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printWatchUsage(os.Stdout)
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	o.notifyCmd = cmdArgs
	if o.testNotify {
		return runTestNotify(o)
	}
	if err := validateWatchOptions(o); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	return runWatchLoop(o)
}

func runTestNotify(o watchOptions) int {
	switch o.notifyMode {
	case "auto", "desktop", "osc", "stderr", "command":
	default:
		fmt.Fprintln(os.Stderr, "error: --notify must be one of auto, desktop, osc, stderr, command")
		return 2
	}
	tr, err := notify.Select(notify.Config{Mode: o.notifyMode, Command: o.notifyCmd, Stderr: os.Stderr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	name, err := notify.Send(context.Background(), tr, notify.Message{
		Title: "swapaudit test notification", Body: "Notification transport is working.",
	})
	if err != nil {
		fmt.Printf("transport: %s failed\n", name)
		return 1
	}
	fmt.Printf("transport: %s ok\n", name)
	return 0
}

func runWatchLoop(o watchOptions) int {
	wcfg := watch.Config{
		ETAThreshold: o.eta, PressureThreshold: o.pressure, Sustain: o.sustain,
		Cooldown: o.cooldown, MaxPerHour: o.maxPerHour, RearmETA: o.rearmETA,
		RearmPressure: o.rearmPressure, Resource: o.resource, NotifyRecovery: o.notifyRecovery,
		Interval: o.interval, TmpInterval: o.tmpInterval, Now: time.Now,
	}
	ncfg := notify.Config{Mode: o.notifyMode, Command: o.notifyCmd, Stderr: os.Stderr}
	ctx, cancel := context.WithCancel(context.Background())

	var messages chan notify.Message
	var senderDone chan struct{}
	dropped := 0

	if !o.json {
		transport, err := notify.Select(ncfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			cancel()
			return 2
		}
		messages = make(chan notify.Message, 4)
		senderDone = make(chan struct{})
		go func() {
			defer close(senderDone)
			notify.Sender(ctx, messages, transport)
		}()
	}
	defer func() {
		if messages != nil {
			close(messages)
			<-senderDone
		}
		cancel()
	}()

	states := map[string]*watch.State{}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	var emitErr error
	lastStatus := time.Time{}

	onSample := func(stats live.WindowStats) {
		for _, decision := range watch.Evaluate(wcfg, states, stats) {
			if o.json {
				if err := enc.Encode(decision.Event); err != nil {
					emitErr = err
					cancel()
				}
				continue
			}
			if decision.Kind == "alert" || (decision.Kind == "recovered" && o.notifyRecovery) {
				msg := notify.Message{Title: decision.Title, Body: decision.Body, Severity: decision.Severity}
				select {
				case messages <- msg:
				default:
					dropped++
				}
			}
		}
		if o.json || o.quiet || o.statusEvery == 0 {
			return
		}
		now := time.Now()
		if lastStatus.IsZero() || now.Sub(lastStatus) >= o.statusEvery {
			line := watch.FormatStatus(o.resource, stats, watch.TopMover(stats, o.resource), o.pressure)
			if dropped > 0 {
				line += fmt.Sprintf(" | dropped %d", dropped)
			}
			fmt.Fprintln(os.Stderr, line)
			lastStatus = now
		}
	}
	onStop := func(reason string) {
		if !o.json {
			return
		}
		line := map[string]any{"type": "stopped", "reason": reason, "sampled_at": time.Now().Format(time.RFC3339)}
		if err := enc.Encode(line); err != nil {
			emitErr = err
		}
	}

	lcfg := live.Config{
		Interval: o.interval, Window: o.window, TmpInterval: o.tmpInterval, Duration: o.duration,
		Out:      io.Discard,
		ScanFast: live.NewScanner().Scan,
		ScanTmp: func() []model.SwapItem {
			return cache.ScanTmpDirectories(time.Now(), 12*time.Hour, cache.DefaultTmpPatterns)
		},
		OnSample: onSample,
		OnStop:   onStop,
	}
	if err := live.Run(ctx, lcfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if emitErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", emitErr)
		return 1
	}
	return 0
}

func printWatchUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: swapaudit watch [FLAGS]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Foreground threshold notifications over the live sampler.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  --eta <duration>        Alert when the ETA is at or below this (default 15m)")
	fmt.Fprintln(w, "  --pressure <percent>    Fallback pressure threshold (default 90)")
	fmt.Fprintln(w, "  --sustain <duration>    Continuous danger before alerting (default 10s)")
	fmt.Fprintln(w, "  --cooldown <duration>   Minimum time between incidents (default 10m)")
	fmt.Fprintln(w, "  --max-per-hour <N>      Maximum alerts per resource per hour (default 4)")
	fmt.Fprintln(w, "  --rearm-eta <duration>  ETA required to re-arm (default 20m)")
	fmt.Fprintln(w, "  --rearm-pressure <pct>  Pressure required to re-arm (default 85)")
	fmt.Fprintln(w, "  --resource <r>          both, swap, or tmpfs (default both)")
	fmt.Fprintln(w, "  --notify <mode>         auto, desktop, osc, stderr, command (default auto)")
	fmt.Fprintln(w, "  --notify-cmd <arg>      Command argv element; repeat for each argument")
	fmt.Fprintln(w, "  --notify-recovery       Notify when a resource recovers")
	fmt.Fprintln(w, "  --test-notify           Send one probe notification and exit")
	fmt.Fprintln(w, "  --status-every <dur>    Status cadence to stderr (default 10s, 0 disables)")
	fmt.Fprintln(w, "  --quiet                 Suppress the status line")
	fmt.Fprintln(w, "  --json                  Emit event NDJSON to stdout and send nothing")
	fmt.Fprintln(w, "  --duration <duration>   Stop after this duration (0 = until signal)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "EXAMPLES:")
	fmt.Fprintln(w, "  swapaudit watch --eta=15m --pressure=90")
	fmt.Fprintln(w, "  swapaudit watch --json --duration=30s")
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if nullInfo, err := os.Stat(os.DevNull); err == nil && os.SameFile(fi, nullInfo) {
		return false
	}
	return true
}

func stdinIsTerminal() bool {
	return isTerminal(os.Stdin)
}

func stdoutIsTerminal() bool {
	return isTerminal(os.Stdout)
}

// pickerEligible applies flags, the environment, and the TTY gate.
func pickerEligible(interactive, noInteractive bool) bool {
	if noInteractive {
		return false
	}
	if interactive {
		return true
	}
	if os.Getenv("SWAPAUDIT_NO_INTERACTIVE") == "1" {
		return false
	}
	return stdinIsTerminal() && stdoutIsTerminal()
}

func runPicker(items []model.SwapItem, dryRun bool, maxAge, idleThreshold time.Duration, patterns []string) {
	cfg := picker.Config{
		In:       bufio.NewReader(os.Stdin),
		Out:      os.Stdout,
		Items:    items,
		DryRun:   dryRun,
		Width:    term.TermWidth(),
		Snapshot: proc.ReadSystemSwap,
		Scan: func() []model.SwapItem {
			return CollectSwapItems(time.Now(), proc.ReadUptime(), maxAge, idleThreshold, patterns)
		},
		Execute: func(items []model.SwapItem, untouched, abandoned, dormant, orphaned, dr, yes bool) {
			executeCleanup(items, untouched, abandoned, dormant, orphaned, dr, yes)
		},
	}
	picker.Run(cfg)
}

// Run parses CLI flags and executes the swapaudit command workflow.
func Run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			PrintUsage(os.Stdout)
			return 0
		case "upgrade", "--upgrade":
			if len(args) > 1 && (args[1] == "-h" || args[1] == "--help" || args[1] == "help") {
				fmt.Println("Usage: swapaudit upgrade")
				fmt.Println()
				fmt.Println("Check GitHub for the latest release and atomically upgrade the swapaudit binary.")
				return 0
			}
			return updater.Upgrade(version.Version, os.Stdout, os.Stderr)
		case "sample":
			return runSample(args[1:])
		case "watch":
			return runWatch(args[1:])
		}
	}

	fs := flag.NewFlagSet("swapaudit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		sortBy         = fs.String("sort", "size", "Sort column: 'size' or 'age'")
		cleanUntouched = fs.Bool("clean-untouched", false, "Clean untouched /tmp build and compile caches")
		cleanAbandoned = fs.Bool("clean-abandoned", false, "Clean abandoned /tmp test and review runs")
		cleanDormant   = fs.Bool("clean-dormant", false, "Stop dormant agent sessions")
		cleanOrphaned  = fs.Bool("clean-orphaned", false, "End orphaned agent sessions (parent shell deceased)")
		cleanTmp       = fs.Bool("clean-tmp", false, "Clean all stale /tmp caches (untouched + abandoned)")
		cleanAll       = fs.Bool("clean-all", false, "Clean all safe cleanable groups")
		dryRun         = fs.Bool("dry-run", false, "Simulate cleanup actions without deleting or stopping")
		yes            = fs.Bool("yes", false, "Automatically answer yes to confirmation prompts")
		maxAge         = fs.Duration("max-age", 12*time.Hour, "Cutoff age for stale /tmp folders")
		idleThreshold  = fs.Duration("idle-threshold", 2*time.Hour, "Idle time threshold for dormant agents")
		limit          = fs.Int("limit", 12, "Maximum rows to show in the main occupants table")
		showAll        = fs.Bool("all", false, "Show all individual processes without aggregation")
		noColor        = fs.Bool("no-color", false, "Disable colored output")
		ascii          = fs.Bool("ascii", false, "Use plain ASCII characters instead of Unicode glyphs")
		showVersion    = fs.Bool("version", false, "Print swapaudit version and exit")
		noUpdateCheck  = fs.Bool("no-update-check", false, "Disable daily release update check")
		jsonOutput     = fs.Bool("json", false, "Emit one machine-readable JSON document and exit")
		noCommand      = fs.Bool("no-command", false, "Omit command lines from JSON output (requires --json)")
		interactive    = fs.Bool("interactive", false, "Force the interactive action picker (requires a terminal)")
		noInteractive  = fs.Bool("no-interactive", false, "Disable the interactive action picker")
	)
	var customTmpPatterns stringSliceFlag
	fs.Var(&customTmpPatterns, "tmp-pattern", "Additional /tmp pattern to identify artifacts (repeatable)")
	fs.BoolVar(yes, "y", false, "Alias for --yes")
	fs.BoolVar(showVersion, "v", false, "Alias for --version")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			PrintUsage(os.Stdout)
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	if *showVersion {
		fmt.Println(version.String())
		return 0
	}

	cleanupRequested := *cleanUntouched || *cleanAbandoned || *cleanDormant || *cleanOrphaned || *cleanTmp || *cleanAll
	if err := validateModes(modeFlags{
		json:          *jsonOutput,
		noCommand:     *noCommand,
		dryRun:        *dryRun,
		yes:           *yes,
		cleanup:       cleanupRequested,
		interactive:   *interactive,
		noInteractive: *noInteractive,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	if *interactive && (!stdinIsTerminal() || !stdoutIsTerminal()) {
		fmt.Fprintln(os.Stderr, "error: --interactive requires a terminal on stdin and stdout")
		return 2
	}

	if *jsonOutput {
		return runJSON(*noCommand, *maxAge, *idleThreshold, customTmpPatterns)
	}

	var cachedNotice *updater.UpdateInfo
	var updateCancel context.CancelFunc
	var updateDone chan struct{}
	var updateNotice *updater.UpdateInfo

	if !*noUpdateCheck && os.Getenv("SWAPAUDIT_NO_UPDATE_CHECK") == "" && os.Getenv("CI") == "" {
		if info := updater.GetCachedUpdate(version.Version); info != nil {
			cachedNotice = info
		} else {
			state, err := updater.LoadState(updater.CacheFilePath())
			checkDue := err != nil || state == nil || time.Since(state.LastChecked) >= updater.CheckInterval
			if checkDue {
				var ctx context.Context
				ctx, updateCancel = context.WithTimeout(context.Background(), 2*time.Second)
				defer updateCancel()
				updateDone = make(chan struct{})
				go func() {
					defer close(updateDone)
					info, _ := updater.CheckDailyUpdateWithContext(ctx, version.Version)
					if info != nil {
						updateNotice = info
					}
				}()
			}
		}
	}

	term.InitTerminal(*noColor, *ascii)

	if *sortBy != "size" && *sortBy != "age" {
		fmt.Fprintf(os.Stderr, "error: invalid --sort value %q (must be 'size' or 'age')\n", *sortBy)
		if updateCancel != nil {
			updateCancel()
			<-updateDone
		}
		return 2
	}

	now := time.Now()
	uptimeSec := proc.ReadUptime()
	sysSwap := proc.ReadSystemSwap()

	tmpPatterns := append(append([]string{}, cache.DefaultTmpPatterns...), customTmpPatterns...)
	items := CollectSwapItems(now, uptimeSec, *maxAge, *idleThreshold, tmpPatterns)

	PrintHeader(sysSwap)
	PrintActionableSection(items, *showAll)
	PrintMainTable(items, *sortBy, *limit, *showAll)
	PrintCategorySummary(items)

	// Consolidate cleanup actions
	if *cleanAll {
		*cleanUntouched = true
		*cleanAbandoned = true
		*cleanDormant = true
		*cleanOrphaned = true
	}
	if *cleanTmp {
		*cleanUntouched = true
		*cleanAbandoned = true
	}

	if cleanupRequested {
		executeCleanup(items, *cleanUntouched, *cleanAbandoned, *cleanDormant, *cleanOrphaned, *dryRun, *yes)
	} else {
		if pickerEligible(*interactive, *noInteractive) {
			runPicker(items, *dryRun, *maxAge, *idleThreshold, tmpPatterns)
		}
		PrintQuickCommands()
	}

	if cachedNotice != nil {
		term.PrintUpdateNotice(cachedNotice.Current, cachedNotice.Latest)
	} else if updateDone != nil {
		select {
		case <-updateDone:
			if updateNotice != nil {
				term.PrintUpdateNotice(updateNotice.Current, updateNotice.Latest)
			}
		case <-time.After(150 * time.Millisecond):
			if updateCancel != nil {
				updateCancel()
			}
			<-updateDone
		}
	}

	return 0
}
