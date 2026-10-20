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
	"strings"
	"time"

	"github.com/jadmadi/swapaudit/internal/cache"
	"github.com/jadmadi/swapaudit/internal/cleaner"
	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
	"github.com/jadmadi/swapaudit/internal/report"
	"github.com/jadmadi/swapaudit/internal/term"
	"github.com/jadmadi/swapaudit/internal/triage"
	"github.com/jadmadi/swapaudit/internal/updater"
	"github.com/jadmadi/swapaudit/internal/version"
)

var (
	checkForUpdate   = updater.CheckForUpdate
	collectAuditData = CollectAuditData
	executeCleanup   = cleaner.ExecuteCleanup
	stdinIsTerminal  = func() bool {
		fi, err := os.Stdin.Stat()
		return err == nil && fi.Mode()&os.ModeCharDevice != 0
	}
)

// Exit codes are a stable contract.
const (
	ExitOK             = 0
	ExitRuntime        = 1
	ExitUsage          = 2
	ExitCleanupPartial = 3
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

// ViewOptions controls the human renderers.
type ViewOptions struct {
	Sort          string
	Limit         int
	ShowAll       bool
	NoGroup       bool
	Wide          bool
	ExplainAgents bool
	ExplainCaches bool
	Expand        string
}

// ScanResult is the shared result of one audit scan.
type ScanResult struct {
	Items      []model.SwapItem
	Report     report.AuditReport
	Agents     triage.AgentPopulation
	CacheStats cache.ScanStats
	System     model.SystemSwap
}

// CollectAuditData scans caches and processes once and builds the audit report.
func CollectAuditData(now time.Time, uptimeSec float64, maxAge, idleThreshold, orphanCacheAge time.Duration, tmpPatterns []string, allowGeneric, cleanOrphanedCaches bool) (ScanResult, error) {
	var res ScanResult
	res.System = proc.ReadSystemSwap()

	roots := []string{"/tmp"}
	if tmp := filepath.Clean(os.TempDir()); tmp != "/tmp" && tmp != "." {
		roots = append(roots, tmp)
	}
	openIndex := cache.BuildOpenIndex("/proc", roots)

	cacheItems, cacheStats := cache.ScanTmpDirectoriesDetailed("/tmp", cache.ScanOptions{
		Now:                 now,
		MaxAge:              maxAge,
		OrphanCacheAge:      orphanCacheAge,
		Patterns:            tmpPatterns,
		OpenIndex:           &openIndex,
		AllowGenericCaches:  allowGeneric,
		CleanOrphanedCaches: cleanOrphanedCaches,
	})
	res.CacheStats = cacheStats

	processResult := proc.ScanProcesses(now, uptimeSec)
	res.Agents = triage.BuildAgentPopulation(processResult, idleThreshold)

	items := append([]model.SwapItem(nil), cacheItems...)
	items = append(items, BuildProcessItems(now, processResult.Processes, idleThreshold)...)

	scan := report.Scan{
		FallbackCount:      cacheStats.FallbackCount,
		ScannedCaches:      cacheStats.Scanned,
		InUseCaches:        cacheStats.InUse,
		OrphanedCaches:     cacheStats.Orphaned,
		RecentCaches:       cacheStats.Recent,
		EvidenceIncomplete: cacheStats.EvidenceIncomplete,
		OpenIndexTruncated: cacheStats.OpenIndexTruncated,
		OpenIndexErrors:    cacheStats.OpenIndexErrors,
		OpenIndexPIDs:      cacheStats.OpenIndexPIDs,
		OpenIndexFDs:       cacheStats.OpenIndexFDs,
	}
	for _, s := range cacheStats.SortedSkips() {
		scan.TmpfsReasons = append(scan.TmpfsReasons, report.ReasonBytes{Reason: s.Reason, Bytes: s.Bytes, Count: s.Count})
	}
	zeroSwap := 0
	for _, p := range processResult.Processes {
		if p.VmSwap == 0 {
			zeroSwap++
		}
	}
	scan.SwapReasons = append(scan.SwapReasons, report.ReasonBytes{Reason: "zero-swap", Count: zeroSwap})
	if processResult.PermissionDenied > 0 {
		scan.SwapReasons = append(scan.SwapReasons, report.ReasonBytes{Reason: "inaccessible", Count: processResult.PermissionDenied})
	}
	if processResult.Vanished > 0 {
		scan.SwapReasons = append(scan.SwapReasons, report.ReasonBytes{Reason: "vanished", Count: processResult.Vanished})
	}

	rep, err := report.BuildWithScan(now, res.System, items, report.Thresholds{
		MaxAge:         maxAge,
		IdleThreshold:  idleThreshold,
		OrphanCacheAge: orphanCacheAge,
	}, scan)
	if err != nil {
		return res, err
	}
	rep.Host = report.Host{
		KernelRelease: readKernelRelease(),
		UptimeSeconds: int64(uptimeSec),
	}
	rep.Population.Processes = processResult.Total
	rep.Population.ProcessesWithSwap = countSwapped(processResult.Processes)
	rep.Population.Agents = res.Agents.Agents
	rep.Population.AgentsWithSwap = res.Agents.AgentsWithSwap
	rep.Population.AgentsZeroSwap = res.Agents.AgentsZeroSwap
	rep.Population.AgentsDormant = res.Agents.DormantAgents
	rep.Population.AgentsOrphaned = res.Agents.OrphanedAgents
	rep.Population.AgentsParentAlive = res.Agents.ParentAliveAgents
	rep.Population.AgentsIdleUnknown = res.Agents.IdleUnknownAgents
	rep.Population.AgentsActive = res.Agents.ActiveAgents
	rep.Population.Vanished = processResult.Vanished
	rep.Population.PermissionDenied = processResult.PermissionDenied

	res.Items = rep.Items
	res.Report = rep
	return res, nil
}

// BuildProcessItems converts process snapshots into display/cleanup items.
// Agents are retained even when VmSwap is zero; non-agent processes are kept
// only when they contribute to the swap attribution.
func BuildProcessItems(now time.Time, procs []proc.ProcessSnapshot, idleThreshold time.Duration) []model.SwapItem {
	items := make([]model.SwapItem, 0, len(procs))
	for _, p := range procs {
		isAgent := triage.IsAgent(p.Comm, p.Argv)
		if !isAgent && p.VmSwap <= 0 {
			continue
		}
		cat, cleanable, reason := triage.ClassifyAgent(p, idleThreshold)
		displayName := p.Comm
		if isAgent {
			displayName = triage.AgentDisplayName(p.Comm, p.Argv)
		}
		item := model.SwapItem{
			ID:              model.ShortID(model.SwapItem{PID: p.PID, StartTicks: p.StartTicks}),
			Kind:            model.KindProcess,
			Agent:           isAgent,
			PID:             p.PID,
			StartTicks:      p.StartTicks,
			UID:             p.UID,
			Name:            displayName,
			Command:         proc.FormatCmdline(p.Argv, p.Comm),
			Category:        cat,
			SwapBytes:       p.VmSwap,
			Age:             p.Age,
			LastActive:      p.Idle,
			LastActiveAt:    now.Add(-p.Idle),
			LastActiveKnown: p.IdleKnown,
			Cleanable:       cleanable,
			Count:           1,
			IdleKnown:       p.IdleKnown,
			OrphanReason:    reason,
			ParentPID:       p.PPID,
			ParentComm:      p.ParentComm,
			ParentAlive:     p.ParentAlive,
			TTYNr:           p.TTYNr,
			Cgroup:          p.Cgroup,
			RSS:             p.RSS,
			Evidence:        fmt.Sprintf("idle=%s idle-known=%t parent=%s orphan=%t reason=%s", formatIdle(p.Idle), p.IdleKnown, parentState(p), p.Orphaned, reason),
		}
		if cleanable {
			item.Signal = "SIGTERM"
		}
		items = append(items, item)
	}
	return items
}

// CollectSwapItems is a compatibility wrapper around CollectAuditData.
func CollectSwapItems(now time.Time, uptimeSec float64, maxAge, idleThreshold time.Duration, tmpPatterns []string) []model.SwapItem {
	res, err := collectAuditData(now, uptimeSec, maxAge, idleThreshold, time.Hour, tmpPatterns, false, false)
	if err != nil {
		return nil
	}
	return res.Items
}

func countSwapped(procs []proc.ProcessSnapshot) int {
	n := 0
	for _, p := range procs {
		if p.VmSwap > 0 {
			n++
		}
	}
	return n
}

func readKernelRelease() string {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func formatIdle(d time.Duration) string {
	if d < 0 {
		return "unknown"
	}
	return d.Truncate(time.Second).String()
}

func parentState(p proc.ProcessSnapshot) string {
	if p.ParentAlive {
		return "alive"
	}
	return "dead"
}

// ResolveCleanupFlags maps the broad cleanup flags to the explicit categories.
// Orphan cleanup is never implicit: --clean-all does not include it.
func ResolveCleanupFlags(cleanAll, cleanUntouched, cleanAbandoned, cleanDormant, cleanOrphaned, includeOrphaned, cleanOrphanedCaches, includeOrphanedCaches bool) cleaner.CleanupFlags {
	flags := cleaner.CleanupFlags{
		Untouched:      cleanUntouched,
		Abandoned:      cleanAbandoned,
		Dormant:        cleanDormant,
		Orphaned:       cleanOrphaned,
		OrphanedCaches: cleanOrphanedCaches,
	}
	if cleanAll {
		flags.Untouched = true
		flags.Abandoned = true
		flags.Dormant = true
	}
	if includeOrphaned && (cleanAll || cleanOrphaned) {
		flags.Orphaned = true
	}
	if includeOrphanedCaches && cleanAll {
		flags.OrphanedCaches = true
	}
	return flags
}

func cleanupRequested(cleanAll, cleanUntouched, cleanAbandoned, cleanDormant, cleanOrphaned, includeOrphaned, cleanOrphanedCaches, includeOrphanedCaches, cleanTmp bool) bool {
	return cleanAll || cleanUntouched || cleanAbandoned || cleanDormant || cleanOrphaned ||
		includeOrphaned || cleanOrphanedCaches || includeOrphanedCaches || cleanTmp
}

// Run parses CLI flags and executes the swapaudit command workflow.
func Run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			PrintUsage(os.Stdout)
			return ExitOK
		case "upgrade", "--upgrade":
			for _, a := range args[1:] {
				if a == "--json" {
					fmt.Fprintln(os.Stderr, "error: --json is not supported for the upgrade command")
					return ExitUsage
				}
			}
			if len(args) > 1 && (args[1] == "-h" || args[1] == "--help" || args[1] == "help") {
				fmt.Println("Usage: swapaudit upgrade")
				fmt.Println()
				fmt.Println("Check GitHub for the latest release and atomically upgrade the swapaudit binary.")
				return ExitOK
			}
			return updater.Upgrade(version.Version, os.Stdout, os.Stderr)
		}
	}

	fs := flag.NewFlagSet("swapaudit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		sortBy               = fs.String("sort", "size", "Sort column: 'size' or 'age'")
		cleanUntouched       = fs.Bool("clean-untouched", false, "Clean untouched /tmp build and compile caches")
		cleanAbandoned       = fs.Bool("clean-abandoned", false, "Clean abandoned /tmp test and review runs")
		cleanDormant         = fs.Bool("clean-dormant", false, "Stop dormant agent sessions")
		cleanOrphaned        = fs.Bool("clean-orphaned", false, "End orphaned agent sessions (evidence-gated)")
		cleanOrphanedCaches  = fs.Bool("clean-orphaned-caches", false, "Prune orphaned /tmp caches with no open handles (opt-in)")
		cleanTmp             = fs.Bool("clean-tmp", false, "Clean all stale /tmp caches (untouched + abandoned)")
		cleanAll             = fs.Bool("clean-all", false, "Clean untouched, abandoned, and dormant groups")
		includeOrphaned      = fs.Bool("include-orphaned", false, "Include orphaned agent cleanup with --clean-all")
		includeOrphanedCache = fs.Bool("include-orphaned-caches", false, "Include orphaned cache cleanup with --clean-all")
		allowGenericCaches   = fs.Bool("allow-generic-caches", false, "Allow generic caches to be classified and cleaned")
		allowLiveParent      = fs.Bool("allow-live-parent", false, "Allow ending dormant agents whose parent is alive")
		allUsers             = fs.Bool("all-users", false, "Allow signaling processes owned by other UIDs")
		dryRun               = fs.Bool("dry-run", false, "Simulate cleanup actions without deleting or stopping")
		yes                  = fs.Bool("yes", false, "Automatically answer yes to confirmation prompts")
		maxAge               = fs.Duration("max-age", 12*time.Hour, "Cutoff age for stale /tmp folders")
		idleThreshold        = fs.Duration("idle-threshold", 2*time.Hour, "Idle time threshold for dormant agents")
		orphanCacheAge       = fs.Duration("orphan-cache-age", time.Hour, "Idle threshold for orphaned caches (must be <= --max-age)")
		limit                = fs.Int("limit", 12, "Maximum rows to show in each table")
		showAll              = fs.Bool("all", false, "Show all items without aggregation or size filtering")
		noGroup              = fs.Bool("no-group", false, "Expand grouped rows into individual items")
		expand               = fs.String("expand", "", "Expand one group key (prefix match)")
		collapse             = fs.Bool("collapse", false, "Collapse groups (default)")
		wide                 = fs.Bool("wide", false, "Show full names without truncation")
		explainAgents        = fs.Bool("explain-agents", false, "Print per-agent evidence")
		explainCaches        = fs.Bool("explain-caches", false, "Print per-cache liveness evidence")
		noColor              = fs.Bool("no-color", false, "Disable colored output")
		ascii                = fs.Bool("ascii", false, "Use plain ASCII characters instead of Unicode glyphs")
		jsonOut              = fs.Bool("json", false, "Emit one machine-readable JSON object on stdout")
		showVersion          = fs.Bool("version", false, "Print swapaudit version and exit")
		checkUpdates         = fs.Bool("check-updates", false, "Check GitHub for a newer release (one network request; see PRIVACY.md)")
	)
	var customTmpPatterns stringSliceFlag
	fs.Var(&customTmpPatterns, "tmp-pattern", "Additional /tmp pattern to identify artifacts (repeatable)")
	fs.BoolVar(yes, "y", false, "Alias for --yes")
	fs.BoolVar(showVersion, "v", false, "Alias for --version")
	fs.BoolVar(wide, "no-truncate", false, "Alias for --wide")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			PrintUsage(os.Stdout)
			return ExitOK
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return ExitUsage
	}

	limitSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "limit" {
			limitSet = true
		}
	})

	if *showVersion {
		fmt.Println(version.String())
		return ExitOK
	}
	if len(fs.Args()) > 0 && (fs.Args()[0] == "upgrade" || fs.Args()[0] == "--upgrade") {
		if *jsonOut {
			fmt.Fprintln(os.Stderr, "error: --json is not supported for the upgrade command")
			return ExitUsage
		}
		return updater.Upgrade(version.Version, os.Stdout, os.Stderr)
	}
	if *jsonOut {
		term.InitTerminal(true, true)
	} else {
		term.InitTerminal(*noColor, *ascii)
	}

	if *sortBy != "size" && *sortBy != "age" {
		fmt.Fprintf(os.Stderr, "error: invalid --sort value %q (must be 'size' or 'age')\n", *sortBy)
		return ExitUsage
	}
	if *orphanCacheAge > *maxAge {
		fmt.Fprintf(os.Stderr, "error: --orphan-cache-age (%s) must be <= --max-age (%s)\n", *orphanCacheAge, *maxAge)
		return ExitUsage
	}

	anyCleanup := cleanupRequested(*cleanAll, *cleanUntouched, *cleanAbandoned, *cleanDormant, *cleanOrphaned, *includeOrphaned, *cleanOrphanedCaches, *includeOrphanedCache, *cleanTmp)
	if anyCleanup && !*dryRun && !*yes && !stdinIsTerminal() {
		fmt.Fprintln(os.Stderr, "error: cleanup requires --dry-run or --yes when stdin is not a terminal")
		return ExitUsage
	}
	if *jsonOut && anyCleanup && !*dryRun && !*yes {
		fmt.Fprintln(os.Stderr, "error: --json live cleanup requires --dry-run or --yes")
		return ExitUsage
	}
	if *dryRun && !anyCleanup {
		fmt.Fprintln(os.Stderr, "warning: --dry-run has no effect without a cleanup flag")
	}
	if *yes && !anyCleanup {
		fmt.Fprintln(os.Stderr, "warning: --yes has no effect without a cleanup flag")
	}
	if *includeOrphaned && !*cleanAll && !*cleanOrphaned {
		fmt.Fprintln(os.Stderr, "warning: --include-orphaned has no effect without --clean-all or --clean-orphaned")
	}
	if *includeOrphanedCache && !*cleanAll && !*cleanOrphanedCaches {
		fmt.Fprintln(os.Stderr, "warning: --include-orphaned-caches has no effect without --clean-all or --clean-orphaned-caches")
	}

	if *showAll && !limitSet {
		*limit = 0
	}

	flags := ResolveCleanupFlags(*cleanAll, *cleanUntouched, *cleanAbandoned, *cleanDormant, *cleanOrphaned, *includeOrphaned, *cleanOrphanedCaches, *includeOrphanedCache)
	if *cleanTmp {
		flags.Untouched = true
		flags.Abandoned = true
	}
	anyCleanup = flags.Untouched || flags.Abandoned || flags.Dormant || flags.Orphaned || flags.OrphanedCaches

	now := time.Now()
	uptimeSec := proc.ReadUptime()
	tmpPatterns := append(append([]string{}, cache.DefaultTmpPatterns...), customTmpPatterns...)

	res, err := collectAuditData(now, uptimeSec, *maxAge, *idleThreshold, *orphanCacheAge, tmpPatterns, *allowGenericCaches, *cleanOrphanedCaches)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: audit failed: %v\n", err)
		return ExitRuntime
	}
	rep := res.Report
	rep.Sort = report.SortMode(*sortBy)
	res.Items = rep.Items

	view := ViewOptions{
		Sort:          *sortBy,
		Limit:         *limit,
		ShowAll:       *showAll,
		NoGroup:       *noGroup,
		Wide:          *wide,
		ExplainAgents: *explainAgents,
		ExplainCaches: *explainCaches,
		Expand:        *expand,
	}
	if *collapse {
		view.Expand = ""
	}

	var cleanupResult *cleaner.CleanupResult
	cleanupOpts := cleaner.CleanupOptions{
		DryRun:          *dryRun,
		Yes:             *yes,
		AllUsers:        *allUsers,
		AllowLiveParent: *allowLiveParent,
	}

	if *jsonOut {
		pv := buildProtectedView(rep, view)
		rep.Population.AggregatedShells = pv.shellCount
		rep.Population.AggregatedRows = pv.shellRows
		rep.Population.Displayed = pv.shown
		rep.Population.HiddenByLimit = pv.hiddenByLimit
		rep.Population.HiddenBySizeFilter = pv.hiddenBySize
		if anyCleanup {
			origStdout := os.Stdout
			devnull, err := os.Open(os.DevNull)
			if err == nil {
				os.Stdout = devnull
				defer func() {
					os.Stdout = origStdout
					devnull.Close()
				}()
			}
			cr := executeCleanup(rep.Items, flags, cleanupOpts)
			os.Stdout = origStdout
			cleanupResult = &cr
		}
		if err := report.WriteJSON(os.Stdout, rep, cleanupResult); err != nil {
			fmt.Fprintf(os.Stderr, "error: writing JSON: %v\n", err)
			return ExitRuntime
		}
	} else {
		term.SetTableOverflow(*wide)
		defer term.SetTableOverflow(false)
		PrintHeader(rep)
		PrintActionableSection(rep, view)
		PrintProtectedTable(rep, view)
		PrintCategorySummary(rep)
		if *explainAgents {
			PrintAgentEvidence(rep, view)
		}
		if *explainCaches {
			PrintCacheEvidence(rep, view)
		}
		if anyCleanup {
			cr := executeCleanup(rep.Items, flags, cleanupOpts)
			cleanupResult = &cr
		} else {
			PrintQuickCommands(rep)
		}
	}

	if *checkUpdates || os.Getenv("SWAPAUDIT_CHECK_UPDATES") == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		info, err := checkForUpdate(ctx, version.Version)
		cancel()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: update check failed: %v\n", err)
		} else if info != nil {
			if *jsonOut {
				fmt.Fprintf(os.Stderr, "A new release of swapaudit is available: %s -> %s\n", info.Current, info.Latest)
			} else {
				term.PrintUpdateNotice(info.Current, info.Latest)
			}
		}
	}

	if cleanupResult != nil && cleanupResult.PartialFailure {
		return ExitCleanupPartial
	}
	return ExitOK
}

// RequireTTYForCleanup is exposed for tests of the non-interactive contract.
func RequireTTYForCleanup(anyCleanup, dryRun, yes, isTTY bool) error {
	if anyCleanup && !dryRun && !yes && !isTTY {
		return fmt.Errorf("cleanup requires --dry-run or --yes when stdin is not a terminal")
	}
	return nil
}
