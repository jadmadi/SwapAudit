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

var defaultStdinReader = bufio.NewReader(os.Stdin)

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

// PruneTmpTargets removes stale tmp directories or files with interactive or automatic confirmation.
func PruneTmpTargets(targets []model.SwapItem, groupTitle, prompt string, dryRun, yes bool, pathWidth int, prefix string) int64 {
	if len(targets) == 0 {
		fmt.Printf("Group: %s - %s\n", groupTitle, term.Dim("Nothing cleanable found."))
		return 0
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
	confirm := yes
	if !confirm && !dryRun {
		confirm = AskConfirmation(prompt)
		if !confirm {
			fmt.Printf("  %s\n", term.Dim("Skipped."))
			return 0
		}
	}

	shown := 0
	limit := 10
	var hiddenBytes int64
	var reclaimed int64
	for _, t := range targets {
		cleanPath := filepath.Clean(t.Path)
		if cleanPath == "" || cleanPath == "." || cleanPath == "/" || cleanPath == "/tmp" || cleanPath == "/var/tmp" || !filepath.IsAbs(cleanPath) {
			continue
		}
		if !dryRun {
			if err := os.RemoveAll(cleanPath); err != nil {
				fmt.Printf("  %s%s failed to prune %s: %v\n", prefix, term.Red(term.GlyphCross()), cleanPath, err)
				continue
			}
		}
		if shown < limit || t.SizeBytes >= 10*1024*1024 {
			tPath := term.Truncate(cleanPath, pathWidth)
			pad := pathWidth - term.StrWidth(tPath)
			if pad < 0 {
				pad = 0
			}
			fmt.Printf("  %s%s %-6s %s%s %10s\n",
				prefix, term.Green(term.GlyphCheck()), "pruned", tPath, strings.Repeat(" ", pad), term.FormatBytes(t.SizeBytes))
			shown++
		} else {
			hiddenBytes += t.SizeBytes
		}
		reclaimed += t.SizeBytes
	}
	if len(targets) > shown {
		fmt.Printf("  %s... and %d smaller targets (%s)\n", prefix, len(targets)-shown, term.FormatBytes(hiddenBytes))
	}
	return reclaimed
}

// KillProcessTargets sends SIGTERM to selected processes with start-ticks re-verification against PID reuse.
func KillProcessTargets(targets []model.SwapItem, groupTitle, prompt string, dryRun, yes bool, pathWidth int, prefix string) int64 {
	if len(targets) == 0 {
		fmt.Printf("Group: %s - %s\n", groupTitle, term.Dim("Nothing cleanable found."))
		return 0
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
	confirm := yes
	if !confirm && !dryRun {
		confirm = AskConfirmation(prompt)
		if !confirm {
			fmt.Printf("  %s\n", term.Dim("Skipped."))
			return 0
		}
	}

	var reclaimed int64
	for _, p := range targets {
		if !dryRun {
			currTicks := proc.ReadStartTicks(p.PID)
			if currTicks == 0 {
				fmt.Printf("  %s%s Skipped PID %d (%s): process no longer exists\n", prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name)
				continue
			}
			if currTicks != p.StartTicks {
				fmt.Printf("  %s%s Skipped PID %d (%s): PID was reused by another process\n", prefix, term.Yellow(term.GlyphWarn()), p.PID, p.Name)
				continue
			}
			if err := syscall.Kill(p.PID, syscall.SIGTERM); err != nil {
				fmt.Printf("  %s%s failed to kill PID %d (%s): %v\n", prefix, term.Red(term.GlyphCross()), p.PID, p.Name, err)
				continue
			}
		}
		pDesc := fmt.Sprintf("PID %d: %s", p.PID, p.Name)
		tDesc := term.Truncate(pDesc, pathWidth)
		pad := pathWidth - term.StrWidth(tDesc)
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("  %s%s %-6s %s%s %10s\n",
			prefix, term.Green(term.GlyphCheck()), "ended", tDesc, strings.Repeat(" ", pad), term.FormatBytes(p.SwapBytes))
		reclaimed += p.SwapBytes
	}
	return reclaimed
}

// ExecuteCleanup coordinates the deletion of stale caches and termination of orphaned/dormant processes.
func ExecuteCleanup(items []model.SwapItem, cleanUntouched, cleanAbandoned, cleanDormant, cleanOrphaned, dryRun, yes bool) {
	title := "Cleanup Execution"
	if dryRun {
		title = "Cleanup Execution (Dry Run)"
	}
	term.PrintBoxHeader(title)

	prefix := ""
	if dryRun {
		prefix = "[dry-run] "
	}

	var totalReclaimedTmp int64
	var totalReclaimedSwap int64

	pathWidth := term.TermWidth() - (len(prefix) + 2 + 1 + 1 + 6 + 1 + 10 + 2)
	if pathWidth < 20 {
		pathWidth = 20
	}

	if cleanUntouched {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == model.CatUntouchedCache && it.Cleanable {
				targets = append(targets, it)
			}
		}
		totalReclaimedTmp += PruneTmpTargets(targets, "Untouched Build Caches", "Prune stale untouched build caches?", dryRun, yes, pathWidth, prefix)
	}

	if cleanAbandoned {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == model.CatAbandonedRun && it.Cleanable {
				targets = append(targets, it)
			}
		}
		if cleanUntouched {
			fmt.Println()
		}
		totalReclaimedTmp += PruneTmpTargets(targets, "Abandoned Test Runs & Workspaces", "Prune abandoned test workspaces and review runs?", dryRun, yes, pathWidth, prefix)
	}

	if cleanOrphaned {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == model.CatAbandonedAgent && it.Cleanable {
				targets = append(targets, it)
			}
		}
		if cleanUntouched || cleanAbandoned {
			fmt.Println()
		}
		totalReclaimedSwap += KillProcessTargets(targets, "Orphaned Agent Processes", "End orphaned agent processes whose parent shell died?", dryRun, yes, pathWidth, prefix)
	}

	if cleanDormant {
		var targets []model.SwapItem
		for _, it := range items {
			if it.Category == model.CatDormantAgent && it.Cleanable {
				targets = append(targets, it)
			}
		}
		if cleanUntouched || cleanAbandoned || cleanOrphaned {
			fmt.Println()
		}
		totalReclaimedSwap += KillProcessTargets(targets, "Dormant Agent Sessions", "End dormant agent processes? (Conversations stay preserved on disk)", dryRun, yes, pathWidth, prefix)
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
		if dryRun {
			fmt.Printf("\nWould reclaim %s\n", joined)
		} else {
			fmt.Printf("\nReclaimed %s\n", joined)
		}
	} else {
		fmt.Printf("\n%s\n", term.Dim("Nothing to reclaim."))
	}
}
