//go:build linux

package cleaner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/jadmadi/swapaudit/internal/cache"
	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
	"github.com/jadmadi/swapaudit/internal/term"
	"github.com/jadmadi/swapaudit/internal/triage"
)

var (
	defaultStdinReader = bufio.NewReader(os.Stdin)
	readProcessUID     = proc.ReadProcUID
	readParentInfo     = proc.ReadParentInfo
	reclassifyOrphan   = proc.ClassifyOrphan
	readStartTicks     = proc.ReadStartTicks
	readUptime         = proc.ReadUptime
	readIdle           = func(pid int) (time.Duration, bool) {
		d := proc.ReadProcessLastActive(pid, readUptime()*1000.0)
		return d, d >= 0
	}
	killProcess    = func(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }
	buildOpenIndex = cache.BuildOpenIndex
)

// OpenCheckMode selects how often cache liveness is rechecked.
type OpenCheckMode string

const (
	OpenCheckGroup  OpenCheckMode = "group"
	OpenCheckTarget OpenCheckMode = "target"
)

// CleanupFlags selects the cleanup categories to execute.
type CleanupFlags struct {
	Untouched      bool
	Abandoned      bool
	Dormant        bool
	Orphaned       bool
	OrphanedCaches bool
}

// CleanupOptions controls how cleanup is previewed and executed.
type CleanupOptions struct {
	DryRun          bool
	Yes             bool
	AllUsers        bool
	AllowLiveParent bool
	PathWidth       int
	Prefix          string
	OpenCheck       OpenCheckMode
}

// ReasonCount is one cleanup skip reason.
type ReasonCount struct {
	Reason string
	Count  int
}

// CleanupGroupResult is the structured result of one cleanup group.
type CleanupGroupResult struct {
	Group      string
	Targets    int
	Bytes      int64
	WouldPrune int
	WouldEnd   int
	Pruned     int
	Ended      int
	Skipped    int
	Reasons    []ReasonCount
}

// CleanupResult is the structured result consumed by the human and JSON renderers.
type CleanupResult struct {
	DryRun         bool
	Groups         []CleanupGroupResult
	Summary        CleanupStats
	PartialFailure bool
}

// CleanupStats records what happened during a cleanup group.
type CleanupStats struct {
	Pruned  int
	Ended   int
	Skipped int
	Reasons map[string]int
}

func (s *CleanupStats) skip(reason string) {
	s.Skipped++
	if s.Reasons == nil {
		s.Reasons = make(map[string]int)
	}
	s.Reasons[reason]++
}

func (s *CleanupStats) skipN(reason string, n int) {
	for i := 0; i < n; i++ {
		s.skip(reason)
	}
}

func (s *CleanupStats) merge(other CleanupStats) {
	s.Pruned += other.Pruned
	s.Ended += other.Ended
	s.Skipped += other.Skipped
	for reason, count := range other.Reasons {
		if s.Reasons == nil {
			s.Reasons = make(map[string]int)
		}
		s.Reasons[reason] += count
	}
}

// SortedReasons returns reasons sorted by count desc then name.
func (s CleanupStats) SortedReasons() []ReasonCount {
	out := make([]ReasonCount, 0, len(s.Reasons))
	for reason, count := range s.Reasons {
		out = append(out, ReasonCount{Reason: reason, Count: count})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

// AskConfirmation prompts the user via stdin for [y/N] confirmation.
func AskConfirmation(prompt string) bool {
	return AskConfirmationReader(defaultStdinReader, prompt)
}

// AskConfirmationReader prompts via the provided reader for [y/N] confirmation.
func AskConfirmationReader(reader *bufio.Reader, prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	ans, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	return ans == "y" || ans == "yes"
}

func normalizeOptions(opts CleanupOptions) CleanupOptions {
	if opts.PathWidth < 20 {
		opts.PathWidth = 40
	}
	if opts.DryRun && opts.Prefix == "" {
		opts.Prefix = "[dry-run] "
	}
	return opts
}

// CacheRecheck is the result of a pre-deletion liveness check.
type CacheRecheck struct {
	Complete bool
	Handles  int
	Cwds     int
	Mapped   int
	Err      error
}

// RecheckCache builds a fresh open index and looks up one cache path.
func RecheckCache(path string) CacheRecheck {
	roots := []string{"/tmp"}
	if tmp := filepath.Clean(os.TempDir()); tmp != "/tmp" && tmp != "." {
		roots = append(roots, tmp)
	}
	idx := buildOpenIndex("/proc", roots)
	handles, cwds, mapped := idx.Lookup(path)
	return CacheRecheck{
		Complete: idx.Complete(),
		Handles:  handles,
		Cwds:     cwds,
		Mapped:   mapped,
	}
}

func recheckSkipReason(r CacheRecheck) string {
	if !r.Complete {
		return "evidence-incomplete"
	}
	if r.Handles > 0 {
		return "open-handle"
	}
	if r.Cwds > 0 {
		return "cwd-inside"
	}
	if r.Mapped > 0 {
		return "mapped-inside"
	}
	return ""
}

func warnSkip(opts CleanupOptions, reason, path string) {
	fmt.Printf("  %s%s SKIP %s: %s\n", opts.Prefix, term.Yellow(term.GlyphWarn()), term.TruncateMiddle(path, opts.PathWidth), reason)
}

// PruneTmpTargets removes stale tmp directories or files with interactive or automatic confirmation.
func PruneTmpTargets(targets []model.SwapItem, groupTitle, prompt string, opts CleanupOptions) (int64, CleanupStats) {
	var stats CleanupStats
	opts = normalizeOptions(opts)

	if len(targets) == 0 {
		fmt.Printf("Group: %s - %s\n", groupTitle, term.Dim("Nothing cleanable found."))
		return 0, stats
	}

	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].SizeBytes != targets[j].SizeBytes {
			return targets[i].SizeBytes > targets[j].SizeBytes
		}
		return targets[i].Path < targets[j].Path
	})

	var groupBytes int64
	for _, t := range targets {
		groupBytes += t.SizeBytes
	}
	fmt.Printf("Group: %s (%d targets, %s)\n", groupTitle, len(targets), term.FormatBytes(groupBytes))
	confirm := opts.Yes
	if !confirm && !opts.DryRun {
		confirm = AskConfirmation(prompt)
		if !confirm {
			fmt.Printf("  %s\n", term.Dim("Skipped."))
			stats.skipN("user-declined", len(targets))
			return 0, stats
		}
	}

	mode := opts.OpenCheck
	if mode == "" {
		mode = OpenCheckGroup
	}
	var groupIndex cache.OpenIndex
	var groupComplete bool
	if mode == OpenCheckGroup {
		roots := []string{"/tmp"}
		if tmp := filepath.Clean(os.TempDir()); tmp != "/tmp" && tmp != "." {
			roots = append(roots, tmp)
		}
		groupIndex = buildOpenIndex("/proc", roots)
		groupComplete = groupIndex.Complete()
	}

	shown := 0
	limit := 10
	var hiddenBytes int64
	var hiddenCount int
	var reclaimed int64
	for _, t := range targets {
		cleanPath := filepath.Clean(t.Path)
		if cleanPath == "" || cleanPath == "." || cleanPath == "/" || cleanPath == "/tmp" || cleanPath == "/var/tmp" || !filepath.IsAbs(cleanPath) {
			stats.skip("path-rejected")
			continue
		}
		tmpBase := filepath.Clean(os.TempDir())
		if !strings.HasPrefix(cleanPath, "/tmp/") && !strings.HasPrefix(cleanPath, "/var/tmp/") && !strings.HasPrefix(cleanPath, tmpBase+"/") {
			stats.skip("path-rejected")
			continue
		}
		if !opts.DryRun {
			fi, err := os.Lstat(cleanPath)
			if err != nil {
				stats.skip("stat-failed")
				continue
			}
			if fi.Mode()&os.ModeSymlink != 0 {
				fmt.Printf("  %s%s Skipped %s: target is a symlink\n", opts.Prefix, term.Yellow(term.GlyphWarn()), term.TruncateMiddle(cleanPath, opts.PathWidth))
				stats.skip("symlink-rejected")
				continue
			}
			if stat, ok := fi.Sys().(*syscall.Stat_t); ok {
				if stat.Uid != uint32(os.Getuid()) {
					fmt.Printf("  %s%s Skipped %s: not owned by current user\n", opts.Prefix, term.Yellow(term.GlyphWarn()), term.TruncateMiddle(cleanPath, opts.PathWidth))
					stats.skip("not-owned")
					continue
				}
			}
		}

		recheck := CacheRecheck{Complete: true}
		if mode == OpenCheckTarget {
			recheck = RecheckCache(cleanPath)
		} else {
			h, c, m := groupIndex.Lookup(cleanPath)
			recheck = CacheRecheck{Complete: groupComplete, Handles: h, Cwds: c, Mapped: m}
		}
		if reason := recheckSkipReason(recheck); reason != "" {
			if reason == "open-handle" && t.OpenHandles == 0 && t.CwdInside == false && t.MappedInside == false {
				reason = "opened-since-scan"
			}
			warnSkip(opts, reason, cleanPath)
			stats.skip(reason)
			continue
		}

		if !opts.DryRun {
			if err := os.RemoveAll(cleanPath); err != nil {
				fmt.Printf("  %s%s failed to prune %s: %v\n", opts.Prefix, term.Red(term.GlyphCross()), term.TruncateMiddle(cleanPath, opts.PathWidth), err)
				stats.skip("remove-failed")
				continue
			}
		}
		if shown < limit || t.SizeBytes >= 10*1024*1024 {
			tPath := term.TruncateMiddle(cleanPath, opts.PathWidth)
			pad := opts.PathWidth - term.StrWidth(tPath)
			if pad < 0 {
				pad = 0
			}
			verb := "pruned"
			if opts.DryRun {
				verb = "would prune"
			}
			fmt.Printf("  %s%s %-11s %s%s %10s\n",
				opts.Prefix, term.Green(term.GlyphCheck()), verb, tPath, strings.Repeat(" ", pad), term.FormatBytes(t.SizeBytes))
			shown++
		} else {
			hiddenBytes += t.SizeBytes
			hiddenCount++
		}
		stats.Pruned++
		reclaimed += t.SizeBytes
	}
	if hiddenCount > 0 {
		fmt.Printf("  %s... and %d smaller targets (%s)\n", opts.Prefix, hiddenCount, term.FormatBytes(hiddenBytes))
	}
	var groupParts []string
	for _, g := range triage.GroupItems(targets, true) {
		if g.Count > 1 && g.Key != "" {
			groupParts = append(groupParts, fmt.Sprintf("%s ×%d", g.Key, g.Count))
		}
	}
	if len(groupParts) > 0 {
		printWrappedLine(opts.Prefix, "Groups: "+strings.Join(groupParts, " · "), term.TermWidth())
	}
	return reclaimed, stats
}

func printWrappedLine(prefix, text string, width int) {
	if width <= 0 {
		width = 100
	}
	words := strings.Fields(text)
	line := prefix
	indent := strings.Repeat(" ", term.StrWidth(prefix))
	for _, word := range words {
		candidate := line + word
		if line != prefix {
			candidate = line + " " + word
		}
		if term.StrWidth(candidate) > width && line != prefix {
			fmt.Println(line)
			line = indent + word
		} else {
			line = candidate
		}
	}
	fmt.Println(line)
}

// KillProcessTargets sends SIGTERM to selected processes after UID ownership,
// parent liveness, orphan evidence, idle evidence, and start-ticks verification.
func KillProcessTargets(targets []model.SwapItem, groupTitle, prompt string, opts CleanupOptions) (int64, CleanupStats) {
	var stats CleanupStats
	opts = normalizeOptions(opts)

	if len(targets) == 0 {
		fmt.Printf("Group: %s - %s\n", groupTitle, term.Dim("Nothing cleanable found."))
		return 0, stats
	}

	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].SwapBytes != targets[j].SwapBytes {
			return targets[i].SwapBytes > targets[j].SwapBytes
		}
		return targets[i].PID < targets[j].PID
	})

	var groupBytes int64
	for _, t := range targets {
		groupBytes += t.SwapBytes
	}
	fmt.Printf("Group: %s (%d processes, %s)\n", groupTitle, len(targets), term.FormatBytes(groupBytes))
	confirm := opts.Yes
	if !confirm && !opts.DryRun {
		confirm = AskConfirmation(prompt)
		if !confirm {
			fmt.Printf("  %s\n", term.Dim("Skipped."))
			stats.skipN("user-declined", len(targets))
			return 0, stats
		}
	}

	warnedAllUsers := false
	var reclaimed int64
	for _, p := range targets {
		if p.PID <= 1 || p.PID == os.Getpid() {
			stats.skip("pid-floor")
			continue
		}

		targetUID, err := readProcessUID(p.PID)
		if err != nil {
			fmt.Printf("  %s%s Skipped PID %d (%s): UID unavailable\n", opts.Prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name)
			stats.skip("uid-unreadable")
			continue
		}
		if targetUID != uint32(os.Getuid()) {
			if !opts.AllUsers {
				fmt.Printf("  %s%s Skipped PID %d (%s): owned by UID %d\n", opts.Prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name, targetUID)
				stats.skip("uid-mismatch")
				continue
			}
			if !warnedAllUsers {
				fmt.Printf("WARNING: --all-users is set; signaling processes owned by other UIDs.\n")
				warnedAllUsers = true
			}
		}

		ppid, parentComm, parentAlive := readParentInfo(p.PID)
		if parentAlive && parentComm != "systemd" && parentComm != "init" {
			if p.Category == model.CatAbandonedAgent {
				fmt.Printf("  %s%s Skipped PID %d (%s): parent %s (PID %d) is alive; orphan evidence is stale\n",
					opts.Prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name, parentComm, ppid)
				stats.skip("parent-alive")
				continue
			}
			if !opts.AllowLiveParent {
				fmt.Printf("  %s%s Skipped PID %d (%s): parent %s (PID %d) is alive\n",
					opts.Prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name, parentComm, ppid)
				stats.skip("parent-alive")
				continue
			}
			fmt.Printf("WARNING: PID %d has a live parent %s (PID %d); --allow-live-parent is set.\n", p.PID, parentComm, ppid)
		}

		if p.Category == model.CatAbandonedAgent {
			_, stillOrphaned := reclassifyOrphan(p.PID, ppid)
			if !stillOrphaned {
				fmt.Printf("  %s%s Skipped PID %d (%s): orphan evidence changed\n", opts.Prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name)
				stats.skip("orphan-evidence-changed")
				continue
			}
		}

		if p.Category == model.CatDormantAgent {
			idle, known := readIdle(p.PID)
			if !known || idle < 0 {
				fmt.Printf("  %s%s Skipped PID %d (%s): idle evidence unavailable\n", opts.Prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name)
				stats.skip("idle-unknown")
				continue
			}
		}

		if p.StartTicks == 0 {
			fmt.Printf("  %s%s Skipped PID %d (%s): start ticks unavailable\n", opts.Prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name)
			stats.skip("start-ticks-unknown")
			continue
		}
		currTicks := readStartTicks(p.PID)
		if currTicks == 0 {
			fmt.Printf("  %s%s Skipped PID %d (%s): process no longer exists\n", opts.Prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name)
			stats.skip("process-gone")
			continue
		}
		if currTicks != p.StartTicks {
			fmt.Printf("  %s%s Skipped PID %d (%s): PID was reused by another process\n", opts.Prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name)
			stats.skip("start-ticks-mismatch")
			continue
		}
		if !opts.DryRun {
			if err := killProcess(p.PID); err != nil {
				fmt.Printf("  %s%s failed to kill PID %d (%s): %v\n", opts.Prefix, term.Red(term.GlyphCross()), p.PID, p.Name, err)
				stats.skip("kill-failed")
				continue
			}
		}

		pDesc := fmt.Sprintf("SIGTERM PID %d: %s", p.PID, p.Name)
		tDesc := term.TruncateMiddle(pDesc, opts.PathWidth)
		pad := opts.PathWidth - term.StrWidth(tDesc)
		if pad < 0 {
			pad = 0
		}
		verb := "signaled"
		if opts.DryRun {
			verb = "would-signal"
		}
		fmt.Printf("  %s%s %-12s %s%s %10s\n",
			opts.Prefix, term.Green(term.GlyphCheck()), verb, tDesc, strings.Repeat(" ", pad), term.FormatBytes(p.SwapBytes))
		stats.Ended++
		reclaimed += p.SwapBytes
	}
	return reclaimed, stats
}

// ExecuteCleanup coordinates the deletion of stale caches and termination of orphaned/dormant processes.
func ExecuteCleanup(items []model.SwapItem, flags CleanupFlags, opts CleanupOptions) CleanupResult {
	title := "Cleanup Execution"
	if opts.DryRun {
		title = "Cleanup Execution (Dry Run)"
	}
	term.PrintBoxHeader(title)

	opts = normalizeOptions(opts)
	result := CleanupResult{DryRun: opts.DryRun}

	var totalReclaimedTmp int64
	var totalReclaimedSwap int64
	var report CleanupStats
	firstGroup := true
	separate := func() {
		if !firstGroup {
			fmt.Println()
		}
		firstGroup = false
	}

	runTmpGroup := func(title, prompt string, category model.Category, mode OpenCheckMode) {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == category && it.Cleanable {
				targets = append(targets, it)
			}
		}
		separate()
		groupOpts := opts
		groupOpts.OpenCheck = mode
		reclaimed, stats := PruneTmpTargets(targets, title, prompt, groupOpts)
		totalReclaimedTmp += reclaimed
		report.merge(stats)
		result.Groups = append(result.Groups, groupResult(title, targets, stats, opts.DryRun, true))
		if stats.Reasons["remove-failed"] > 0 {
			result.PartialFailure = true
		}
	}

	runProcessGroup := func(title, prompt string, category model.Category) {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == category && it.Cleanable {
				targets = append(targets, it)
			}
		}
		separate()
		reclaimed, stats := KillProcessTargets(targets, title, prompt, opts)
		totalReclaimedSwap += reclaimed
		report.merge(stats)
		result.Groups = append(result.Groups, groupResult(title, targets, stats, opts.DryRun, false))
		if stats.Reasons["kill-failed"] > 0 {
			result.PartialFailure = true
		}
	}

	if flags.Untouched {
		runTmpGroup("Untouched Build Caches", "Prune stale untouched build caches?", model.CatUntouchedCache, OpenCheckGroup)
	}
	if flags.Abandoned {
		runTmpGroup("Abandoned Test Runs & Workspaces", "Prune abandoned test workspaces and review runs?", model.CatAbandonedRun, OpenCheckGroup)
	}
	if flags.OrphanedCaches {
		runTmpGroup("Orphaned Cache Directories", "Prune orphaned cache directories with no open handles?", model.CatOrphanedCache, OpenCheckTarget)
	}
	if flags.Orphaned {
		runProcessGroup("Orphaned Agent Processes", "End orphaned agent processes whose parent shell died?", model.CatAbandonedAgent)
	}
	if flags.Dormant {
		runProcessGroup("Dormant Agent Sessions", "End dormant agent processes? This sends SIGTERM; unsaved in-memory state may be lost. Verify the agent's persistence behavior first.", model.CatDormantAgent)
	}

	if totalReclaimedTmp > 0 || totalReclaimedSwap > 0 {
		var parts []string
		if totalReclaimedTmp > 0 {
			parts = append(parts, fmt.Sprintf("%s tmp", term.FormatBytes(totalReclaimedTmp)))
		}
		if totalReclaimedSwap > 0 {
			parts = append(parts, fmt.Sprintf("%s swap", term.FormatBytes(totalReclaimedSwap)))
		}
		joined := strings.Join(parts, ", ")
		if opts.DryRun {
			fmt.Printf("\nWould reclaim %s\n", joined)
		} else {
			fmt.Printf("\nReclaimed %s\n", joined)
		}
	} else {
		fmt.Printf("\n%s\n", term.Dim("Nothing to reclaim."))
	}

	result.Summary = report
	fmt.Printf("Cleanup summary%s: %s\n", summarySuffix(opts.DryRun), formatStats(report, opts.DryRun))
	return result
}

func groupResult(title string, targets []model.SwapItem, stats CleanupStats, dryRun, tmp bool) CleanupGroupResult {
	g := CleanupGroupResult{
		Group:   title,
		Targets: len(targets),
		Skipped: stats.Skipped,
		Reasons: stats.SortedReasons(),
	}
	for _, t := range targets {
		if tmp {
			g.Bytes += t.SizeBytes
		} else {
			g.Bytes += t.SwapBytes
		}
	}
	if dryRun {
		if tmp {
			g.WouldPrune = stats.Pruned
		} else {
			g.WouldEnd = stats.Ended
		}
	} else {
		g.Pruned = stats.Pruned
		g.Ended = stats.Ended
	}
	return g
}

func summarySuffix(dryRun bool) string {
	if dryRun {
		return " (dry-run)"
	}
	return ""
}

func formatStats(stats CleanupStats, dryRun bool) string {
	prunedVerb := "pruned"
	endedVerb := "ended"
	if dryRun {
		prunedVerb = "would prune"
		endedVerb = "would end"
	}
	parts := []string{
		fmt.Sprintf("%d %s", stats.Pruned, prunedVerb),
		fmt.Sprintf("%d %s", stats.Ended, endedVerb),
		fmt.Sprintf("%d skipped", stats.Skipped),
	}
	if reasons := stats.SortedReasons(); len(reasons) > 0 {
		reasonParts := make([]string, 0, len(reasons))
		for _, rc := range reasons {
			reasonParts = append(reasonParts, fmt.Sprintf("%d %s", rc.Count, rc.Reason))
		}
		parts = append(parts, "("+strings.Join(reasonParts, ", ")+")")
	}
	return strings.Join(parts, ", ")
}
