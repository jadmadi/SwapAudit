//go:build linux

package app

import (
	"context"
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
	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
	"github.com/jadmadi/swapaudit/internal/snapshot"
	"github.com/jadmadi/swapaudit/internal/term"
	"github.com/jadmadi/swapaudit/internal/triage"
	"github.com/jadmadi/swapaudit/internal/updater"
	"github.com/jadmadi/swapaudit/internal/version"
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

	if *cleanUntouched || *cleanAbandoned || *cleanDormant || *cleanOrphaned {
		executeCleanup(items, *cleanUntouched, *cleanAbandoned, *cleanDormant, *cleanOrphaned, *dryRun, *yes)
	} else {
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
