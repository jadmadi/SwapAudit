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
		_, isOrphaned := proc.ClassifyOrphan(pid, ppid)

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

// ResolveCleanupFlags maps the broad cleanup flags to the explicit categories.
// Orphan cleanup is never implicit: --clean-all does not include it.
func ResolveCleanupFlags(cleanAll, cleanUntouched, cleanAbandoned, cleanDormant, cleanOrphaned, includeOrphaned bool) cleaner.CleanupFlags {
	flags := cleaner.CleanupFlags{
		Untouched: cleanUntouched,
		Abandoned: cleanAbandoned,
		Dormant:   cleanDormant,
		Orphaned:  cleanOrphaned,
	}
	if cleanAll {
		flags.Untouched = true
		flags.Abandoned = true
		flags.Dormant = true
	}
	if includeOrphaned && (cleanAll || cleanOrphaned) {
		flags.Orphaned = true
	}
	return flags
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
		sortBy          = fs.String("sort", "size", "Sort column: 'size' or 'age'")
		cleanUntouched  = fs.Bool("clean-untouched", false, "Clean untouched /tmp build and compile caches")
		cleanAbandoned  = fs.Bool("clean-abandoned", false, "Clean abandoned /tmp test and review runs")
		cleanDormant    = fs.Bool("clean-dormant", false, "Stop dormant agent sessions")
		cleanOrphaned   = fs.Bool("clean-orphaned", false, "End orphaned agent sessions (evidence-gated)")
		cleanTmp        = fs.Bool("clean-tmp", false, "Clean all stale /tmp caches (untouched + abandoned)")
		cleanAll        = fs.Bool("clean-all", false, "Clean untouched, abandoned, and dormant groups")
		includeOrphaned = fs.Bool("include-orphaned", false, "Include orphaned cleanup with --clean-all")
		allUsers        = fs.Bool("all-users", false, "Allow signaling processes owned by other UIDs")
		dryRun          = fs.Bool("dry-run", false, "Simulate cleanup actions without deleting or stopping")
		yes             = fs.Bool("yes", false, "Automatically answer yes to confirmation prompts")
		maxAge          = fs.Duration("max-age", 12*time.Hour, "Cutoff age for stale /tmp folders")
		idleThreshold   = fs.Duration("idle-threshold", 2*time.Hour, "Idle time threshold for dormant agents")
		limit           = fs.Int("limit", 12, "Maximum rows to show in the main occupants table")
		showAll         = fs.Bool("all", false, "Show all individual processes without aggregation")
		noColor         = fs.Bool("no-color", false, "Disable colored output")
		ascii           = fs.Bool("ascii", false, "Use plain ASCII characters instead of Unicode glyphs")
		showVersion     = fs.Bool("version", false, "Print swapaudit version and exit")
		noUpdateCheck   = fs.Bool("no-update-check", false, "Disable daily release update check")
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

	// Consolidate cleanup actions. Orphan cleanup is always opt-in.
	if *includeOrphaned && !*cleanAll && !*cleanOrphaned {
		fmt.Fprintln(os.Stderr, "warning: --include-orphaned has no effect without --clean-all or --clean-orphaned")
	}
	flags := ResolveCleanupFlags(*cleanAll, *cleanUntouched, *cleanAbandoned, *cleanDormant, *cleanOrphaned, *includeOrphaned)
	if *cleanTmp {
		flags.Untouched = true
		flags.Abandoned = true
	}

	if flags.Untouched || flags.Abandoned || flags.Dormant || flags.Orphaned {
		cleaner.ExecuteCleanup(items, flags, cleaner.CleanupOptions{
			DryRun:   *dryRun,
			Yes:      *yes,
			AllUsers: *allUsers,
		})
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
