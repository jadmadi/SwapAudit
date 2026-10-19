//go:build linux

package app

import (
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
		cmd := proc.ReadCmdline(pid, comm)
		age := proc.ReadProcessAge(pid, bootTime)
		lastActive := proc.ReadProcessLastActive(pid, uptimeMs)
		lastActiveAt := now.Add(-lastActive)

		cat, cleanable := triage.CategorizeProcess(comm, argv, pid, ppid, lastActive, idleThreshold)

		dispName := comm
		hasConv := false
		for i, arg := range argv {
			if arg == "--conversation" && i+1 < len(argv) {
				targetId := argv[i+1]
				if len(targetId) > 8 {
					targetId = targetId[:8]
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

		startTicks := proc.ReadStartTicks(pid)
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

// Run parses CLI flags and executes the swapaudit command workflow.
func Run(args []string) int {
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
	)
	var customTmpPatterns stringSliceFlag
	fs.Var(&customTmpPatterns, "tmp-pattern", "Additional /tmp pattern to identify artifacts (repeatable)")
	fs.BoolVar(yes, "y", false, "Alias for --yes")
	fs.BoolVar(showVersion, "v", false, "Alias for --version")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	if *showVersion {
		fmt.Println(version.String())
		return 0
	}

	term.InitTerminal(*noColor, *ascii)

	if *sortBy != "size" && *sortBy != "age" {
		fmt.Fprintf(os.Stderr, "error: invalid --sort value %q (must be 'size' or 'age')\n", *sortBy)
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
		cleaner.ExecuteCleanup(items, *cleanUntouched, *cleanAbandoned, *cleanDormant, *cleanOrphaned, *dryRun, *yes)
	} else {
		PrintQuickCommands()
	}

	return 0
}
