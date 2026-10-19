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

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
	"github.com/jadmadi/swapaudit/internal/term"
)

var (
	defaultStdinReader = bufio.NewReader(os.Stdin)
	readProcessUID     = proc.ReadProcUID
)

// CleanupFlags selects the cleanup categories to execute.
type CleanupFlags struct {
	Untouched bool
	Abandoned bool
	Dormant   bool
	Orphaned  bool
}

// CleanupOptions controls how cleanup is previewed and executed.
type CleanupOptions struct {
	DryRun    bool
	Yes       bool
	AllUsers  bool
	PathWidth int
	Prefix    string
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

	shown := 0
	limit := 10
	var hiddenBytes int64
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
				fmt.Printf("  %s%s Skipped %s: target is a symlink\n", opts.Prefix, term.Yellow(term.GlyphWarn()), cleanPath)
				stats.skip("symlink-rejected")
				continue
			}
			if stat_t, ok := fi.Sys().(*syscall.Stat_t); ok {
				if stat_t.Uid != uint32(os.Getuid()) {
					fmt.Printf("  %s%s Skipped %s: not owned by current user\n", opts.Prefix, term.Yellow(term.GlyphWarn()), cleanPath)
					stats.skip("not-owned")
					continue
				}
			}
			if err := os.RemoveAll(cleanPath); err != nil {
				fmt.Printf("  %s%s failed to prune %s: %v\n", opts.Prefix, term.Red(term.GlyphCross()), cleanPath, err)
				stats.skip("remove-failed")
				continue
			}
		}
		if shown < limit || t.SizeBytes >= 10*1024*1024 {
			tPath := term.Truncate(cleanPath, opts.PathWidth)
			pad := opts.PathWidth - term.StrWidth(tPath)
			if pad < 0 {
				pad = 0
			}
			fmt.Printf("  %s%s %-6s %s%s %10s\n",
				opts.Prefix, term.Green(term.GlyphCheck()), "pruned", tPath, strings.Repeat(" ", pad), term.FormatBytes(t.SizeBytes))
			shown++
		} else {
			hiddenBytes += t.SizeBytes
		}
		stats.Pruned++
		reclaimed += t.SizeBytes
	}
	if len(targets) > shown {
		fmt.Printf("  %s... and %d smaller targets (%s)\n", opts.Prefix, len(targets)-shown, term.FormatBytes(hiddenBytes))
	}
	return reclaimed, stats
}

// KillProcessTargets sends SIGTERM to selected processes after UID ownership
// and start-ticks verification.
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

		if !opts.DryRun {
			currTicks := proc.ReadStartTicks(p.PID)
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
			if err := syscall.Kill(p.PID, syscall.SIGTERM); err != nil {
				fmt.Printf("  %s%s failed to kill PID %d (%s): %v\n", opts.Prefix, term.Red(term.GlyphCross()), p.PID, p.Name, err)
				stats.skip("kill-failed")
				continue
			}
		}

		pDesc := fmt.Sprintf("PID %d: %s", p.PID, p.Name)
		tDesc := term.Truncate(pDesc, opts.PathWidth)
		pad := opts.PathWidth - term.StrWidth(tDesc)
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("  %s%s %-6s %s%s %10s\n",
			opts.Prefix, term.Green(term.GlyphCheck()), "ended", tDesc, strings.Repeat(" ", pad), term.FormatBytes(p.SwapBytes))
		stats.Ended++
		reclaimed += p.SwapBytes
	}
	return reclaimed, stats
}

// ExecuteCleanup coordinates the deletion of stale caches and termination of orphaned/dormant processes.
func ExecuteCleanup(items []model.SwapItem, flags CleanupFlags, opts CleanupOptions) {
	title := "Cleanup Execution"
	if opts.DryRun {
		title = "Cleanup Execution (Dry Run)"
	}
	term.PrintBoxHeader(title)

	opts = normalizeOptions(opts)

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

	if flags.Untouched {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == model.CatUntouchedCache && it.Cleanable {
				targets = append(targets, it)
			}
		}
		separate()
		reclaimed, stats := PruneTmpTargets(targets, "Untouched Build Caches", "Prune stale untouched build caches?", opts)
		totalReclaimedTmp += reclaimed
		report.merge(stats)
	}

	if flags.Abandoned {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == model.CatAbandonedRun && it.Cleanable {
				targets = append(targets, it)
			}
		}
		separate()
		reclaimed, stats := PruneTmpTargets(targets, "Abandoned Test Runs & Workspaces", "Prune abandoned test workspaces and review runs?", opts)
		totalReclaimedTmp += reclaimed
		report.merge(stats)
	}

	if flags.Orphaned {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == model.CatAbandonedAgent && it.Cleanable {
				targets = append(targets, it)
			}
		}
		separate()
		reclaimed, stats := KillProcessTargets(targets, "Orphaned Agent Processes", "End orphaned agent processes whose parent shell died?", opts)
		totalReclaimedSwap += reclaimed
		report.merge(stats)
	}

	if flags.Dormant {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == model.CatDormantAgent && it.Cleanable {
				targets = append(targets, it)
			}
		}
		separate()
		reclaimed, stats := KillProcessTargets(targets, "Dormant Agent Sessions", "End dormant agent processes? This sends SIGTERM; unsaved in-memory state may be lost. Verify the agent's persistence behavior first.", opts)
		totalReclaimedSwap += reclaimed
		report.merge(stats)
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

	fmt.Printf("Cleanup summary%s: %s\n", summarySuffix(opts.DryRun), formatStats(report, opts.DryRun))
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
	if len(stats.Reasons) > 0 {
		keys := make([]string, 0, len(stats.Reasons))
		for reason := range stats.Reasons {
			keys = append(keys, reason)
		}
		sort.Strings(keys)
		reasonParts := make([]string, 0, len(keys))
		for _, reason := range keys {
			reasonParts = append(reasonParts, fmt.Sprintf("%d %s", stats.Reasons[reason], reason))
		}
		parts = append(parts, "("+strings.Join(reasonParts, ", ")+")")
	}
	return strings.Join(parts, ", ")
}
