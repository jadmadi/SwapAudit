//go:build linux

package app

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/term"
	"github.com/jadmadi/swapaudit/internal/triage"
)

// PrintHeader renders the system overview banner and RAM/Swap/tmpfs meter bars.
func PrintHeader(s model.SystemSwap) {
	term.PrintBoxHeader("SWAP & IN-MEMORY CACHE AUDITOR")

	barWidth := 16

	// 1. Swap
	if s.SwapTotal <= 0 {
		fmt.Printf("  %-5s  Swap not configured\n", "Swap")
	} else {
		swapPct := (float64(s.SwapUsed) / float64(s.SwapTotal)) * 100.0
		bar := term.RenderMeterBar(swapPct, barWidth)
		fmt.Printf("  %-5s %s %5.1f%% %8s / %8s (free %s)\n",
			"Swap", bar, swapPct, term.FormatBytes(s.SwapUsed), term.FormatBytes(s.SwapTotal), term.FormatBytes(s.SwapFree))
	}

	// 2. RAM
	ramPct := 0.0
	if s.MemTotal > 0 {
		ramPct = (float64(s.MemUsed) / float64(s.MemTotal)) * 100.0
	}
	ramBar := term.RenderMeterBar(ramPct, barWidth)
	fmt.Printf("  %-5s %s %5.1f%% %8s / %8s (avail %s)\n",
		"RAM", ramBar, ramPct, term.FormatBytes(s.MemUsed), term.FormatBytes(s.MemTotal), term.FormatBytes(s.MemAvail))

	// 3. /tmp
	tmpPct := 0.0
	if s.TmpfsSize > 0 {
		tmpPct = (float64(s.TmpfsUsed) / float64(s.TmpfsSize)) * 100.0
	}
	tmpBar := term.RenderMeterBar(tmpPct, barWidth)
	tmpFree := s.TmpfsSize - s.TmpfsUsed
	fmt.Printf("  %-5s %s %5.1f%% %8s / %8s (free %s)\n\n",
		"/tmp", tmpBar, tmpPct, term.FormatBytes(s.TmpfsUsed), term.FormatBytes(s.TmpfsSize), term.FormatBytes(tmpFree))
}

// PrintActionableSection renders the actionable reclaim candidates table.
func PrintActionableSection(items []model.SwapItem, showAll bool) {
	var cleanables []model.SwapItem
	var totalReclaimTmp int64
	var totalReclaimSwap int64

	for _, it := range items {
		if it.Cleanable {
			cleanables = append(cleanables, it)
			totalReclaimTmp += it.SizeBytes
			totalReclaimSwap += it.SwapBytes
		}
	}

	if len(cleanables) == 0 {
		fmt.Printf("  %s\n\n", term.Dim(term.GlyphCheck()+" nothing to reclaim"))
		return
	}

	sort.SliceStable(cleanables, func(i, j int) bool {
		si := model.ItemSize(cleanables[i])
		sj := model.ItemSize(cleanables[j])
		if si != sj {
			return si > sj
		}
		if cleanables[i].PID != cleanables[j].PID {
			return cleanables[i].PID < cleanables[j].PID
		}
		return cleanables[i].Name < cleanables[j].Name
	})

	boxTitle := fmt.Sprintf("Actionable Reclaim Candidates (Cleanable: %s)", term.FormatBytes(totalReclaimTmp+totalReclaimSwap))
	term.PrintBoxHeader(boxTitle)

	var cols []model.Col
	var targetWidth int
	w := term.TermWidth()
	if w >= 100 {
		targetWidth = w - 82
		if targetWidth < 12 {
			targetWidth = 12
		}
		cols = []model.Col{
			{Title: "PID", Width: 9},
			{Title: "Target / Name", Width: targetWidth},
			{Title: "Category", Width: 16},
			{Title: "Size", Width: 10, Right: true},
			{Title: "Idle", Width: 8, Right: true},
			{Title: "Action", Width: 22},
		}
	} else {
		targetWidth = w - 63
		if targetWidth < 12 {
			targetWidth = 12
		}
		cols = []model.Col{
			{Title: "PID", Width: 9},
			{Title: "Target / Name", Width: targetWidth},
			{Title: "Size", Width: 10, Right: true},
			{Title: "Idle", Width: 8, Right: true},
			{Title: "Action", Width: 22},
		}
	}

	limit := 12
	shown := 0
	var hiddenBytes int64
	var rows [][]string

	for _, it := range cleanables {
		shown++
		if !showAll && shown > limit {
			hiddenBytes += model.ItemSize(it)
			continue
		}

		pidStr := "-"
		if it.PID > 0 {
			pidStr = strconv.Itoa(it.PID)
		}
		colPID := term.Yellow(term.GlyphActiveIndicator() + " " + pidStr)

		name := term.Truncate(it.Name, targetWidth)

		action := "Prune stale cache"
		switch it.Category {
		case model.CatUntouchedCache:
			action = "Prune untouched cache"
		case model.CatAbandonedRun:
			action = "Prune abandoned run"
		case model.CatAbandonedAgent:
			action = "End orphaned process"
		case model.CatDormantAgent:
			action = "End dormant session"
		}

		if w >= 100 {
			rows = append(rows, []string{
				colPID,
				name,
				string(it.Category),
				term.FormatBytes(model.ItemSize(it)),
				term.FormatDuration(it.LastActive),
				action,
			})
		} else {
			rows = append(rows, []string{
				colPID,
				name,
				term.FormatBytes(model.ItemSize(it)),
				term.FormatDuration(it.LastActive),
				action,
			})
		}
	}

	term.RenderTable(cols, rows)

	if !showAll && len(cleanables) > limit {
		fmt.Printf("  ... and %d smaller candidates (%s). Run with --all to inspect full list.\n",
			len(cleanables)-limit, term.FormatBytes(hiddenBytes))
	}
	fmt.Println()
}

// PrintMainTable renders the occupants ranking table.
func PrintMainTable(items []model.SwapItem, sortBy string, limit int, showAll bool) {
	displayItems := items
	if !showAll {
		displayItems = triage.AggregateShells(items)
	}

	if sortBy == "age" {
		sort.SliceStable(displayItems, func(i, j int) bool {
			if displayItems[i].Age != displayItems[j].Age {
				return displayItems[i].Age > displayItems[j].Age
			}
			if displayItems[i].PID != displayItems[j].PID {
				return displayItems[i].PID < displayItems[j].PID
			}
			return displayItems[i].Name < displayItems[j].Name
		})
	} else {
		sort.SliceStable(displayItems, func(i, j int) bool {
			si := model.ItemSize(displayItems[i])
			sj := model.ItemSize(displayItems[j])
			if si != sj {
				return si > sj
			}
			if displayItems[i].PID != displayItems[j].PID {
				return displayItems[i].PID < displayItems[j].PID
			}
			return displayItems[i].Name < displayItems[j].Name
		})
	}

	boxTitle := fmt.Sprintf("Top Memory & Cache Occupants (Ranked by %s)", strings.ToUpper(sortBy))
	term.PrintBoxHeader(boxTitle)

	var cols []model.Col
	var targetWidth int
	w := term.TermWidth()
	if w >= 100 {
		targetWidth = w - 68
		if targetWidth < 12 {
			targetWidth = 12
		}
		cols = []model.Col{
			{Title: "PID", Width: 9},
			{Title: "Target / Command", Width: targetWidth},
			{Title: "Category", Width: 16},
			{Title: "Size", Width: 10, Right: true},
			{Title: "Age", Width: 8, Right: true},
			{Title: "Idle", Width: 8, Right: true},
		}
	} else {
		targetWidth = w - 38
		if targetWidth < 12 {
			targetWidth = 12
		}
		fourthCol := model.Col{Title: "Idle", Width: 8, Right: true}
		if sortBy == "age" {
			fourthCol = model.Col{Title: "Age", Width: 8, Right: true}
		}
		cols = []model.Col{
			{Title: "PID", Width: 9},
			{Title: "Target / Command", Width: targetWidth},
			{Title: "Size", Width: 10, Right: true},
			fourthCol,
		}
	}

	shown := 0
	hidden := 0
	var rows [][]string

	for _, it := range displayItems {
		sz := model.ItemSize(it)
		if sz < 10*1024*1024 && !it.Cleanable {
			hidden++
			continue
		}
		if limit > 0 && shown >= limit {
			hidden++
			continue
		}
		shown++

		pidStr := "-"
		if it.PID > 0 {
			pidStr = strconv.Itoa(it.PID)
		}

		var colPID string
		if it.Cleanable {
			colPID = term.Yellow(term.GlyphActiveIndicator() + " " + pidStr)
		} else if it.Category == model.CatDesktopDaemon {
			colPID = term.Dim("  " + pidStr)
		} else {
			colPID = "  " + pidStr
		}

		name := term.Truncate(it.Name, targetWidth)

		if w >= 100 {
			rows = append(rows, []string{
				colPID,
				name,
				string(it.Category),
				term.FormatBytes(sz),
				term.FormatDuration(it.Age),
				term.FormatDuration(it.LastActive),
			})
		} else {
			fourthVal := term.FormatDuration(it.LastActive)
			if sortBy == "age" {
				fourthVal = term.FormatDuration(it.Age)
			}
			rows = append(rows, []string{
				colPID,
				name,
				term.FormatBytes(sz),
				fourthVal,
			})
		}
	}

	term.RenderTable(cols, rows)

	if hidden > 0 {
		fmt.Printf("  ... and %d more hidden. Run with --all to inspect full list.\n", hidden)
	}
	fmt.Println()
}

// PrintCategorySummary renders the category breakdown table.
func PrintCategorySummary(items []model.SwapItem) {
	tmpSums := make(map[model.Category]int64)
	swapSums := make(map[model.Category]int64)
	cleanableTmpSums := make(map[model.Category]int64)
	cleanableSwapSums := make(map[model.Category]int64)
	counts := make(map[model.Category]int)

	var totalProcessSwap int64
	var totalTmpSize int64

	for _, it := range items {
		tmpSums[it.Category] += it.SizeBytes
		swapSums[it.Category] += it.SwapBytes
		counts[it.Category] += it.Count
		totalProcessSwap += it.SwapBytes
		totalTmpSize += it.SizeBytes
		if it.Cleanable {
			cleanableTmpSums[it.Category] += it.SizeBytes
			cleanableSwapSums[it.Category] += it.SwapBytes
		}
	}

	term.PrintBoxHeader("Category Breakdown & Actionable Totals")

	catWidth := term.TermWidth() - 61
	if catWidth < 14 {
		catWidth = 14
	}

	cols := []model.Col{
		{Title: "Category", Width: catWidth},
		{Title: "Entities", Width: 8, Right: true},
		{Title: "Size", Width: 10, Right: true},
		{Title: "Share", Width: 19},
		{Title: "Actionable", Width: 10, Right: true},
	}

	var rows [][]string

	// Group 1: Process Swap
	swapCats := []model.Category{
		model.CatDormantAgent,
		model.CatAbandonedAgent,
		model.CatDesktopDaemon,
		model.CatActiveProcess,
	}

	hasSwapRows := false
	for _, c := range swapCats {
		if counts[c] > 0 {
			hasSwapRows = true
			break
		}
	}

	if hasSwapRows {
		rows = append(rows, []string{
			term.Bold("Process Swap (RAM)"), "", "", "", "",
		})
		for _, c := range swapCats {
			if counts[c] == 0 {
				continue
			}
			swapBytes := swapSums[c]
			sharePct := 0.0
			if totalProcessSwap > 0 {
				sharePct = (float64(swapBytes) / float64(totalProcessSwap)) * 100.0
			}
			shareBar := term.RenderShareMiniBar(sharePct, 10)
			actStr := term.FormatBytes(cleanableSwapSums[c])

			rows = append(rows, []string{
				"  " + string(c),
				strconv.Itoa(counts[c]),
				term.FormatBytes(swapBytes),
				shareBar,
				actStr,
			})
		}
	}

	// Group 2: /tmp Caches
	tmpCats := []model.Category{
		model.CatUntouchedCache,
		model.CatAbandonedRun,
		model.CatActiveCache,
	}

	hasTmpRows := false
	for _, c := range tmpCats {
		if counts[c] > 0 {
			hasTmpRows = true
			break
		}
	}

	if hasTmpRows {
		rows = append(rows, []string{
			term.Bold("/tmp Caches (tmpfs)"), "", "", "", "",
		})
		for _, c := range tmpCats {
			if counts[c] == 0 {
				continue
			}
			tmpBytes := tmpSums[c]
			sharePct := 0.0
			if totalTmpSize > 0 {
				sharePct = (float64(tmpBytes) / float64(totalTmpSize)) * 100.0
			}
			shareBar := term.RenderShareMiniBar(sharePct, 10)
			actStr := term.FormatBytes(cleanableTmpSums[c])

			rows = append(rows, []string{
				"  " + string(c),
				strconv.Itoa(counts[c]),
				term.FormatBytes(tmpBytes),
				shareBar,
				actStr,
			})
		}
	}

	term.RenderTable(cols, rows)
	fmt.Println()
}

// PrintQuickCommands renders recommended cleanup CLI commands.
func PrintQuickCommands() {
	term.PrintBoxHeader("Quick Maintenance Commands")
	cmd := func(flagName, comment string) {
		cyanFlag := term.Cyan(flagName)
		flagVisW := term.StrWidth(flagName)
		pad := 20 - flagVisW
		if pad < 1 {
			pad = 1
		}
		fmt.Printf("  swapaudit %s%s # %s\n", cyanFlag, strings.Repeat(" ", pad), comment)
	}

	cmd("--clean-untouched", "Prune stale build caches (> 12h untouched)")
	cmd("--clean-abandoned", "Prune abandoned test suites & review runs (> 12h idle)")
	cmd("--clean-dormant", "End dormant agent sessions (> 2h idle)")
	cmd("--clean-orphaned", "End orphaned agent sessions whose parent shell died")
	cmd("--clean-tmp", "Clean all stale /tmp caches (untouched + abandoned)")
	cmd("--clean-all", "Run all cleanup actions (untouched + abandoned + dormant + orphaned)")
	cmd("--dry-run", "Preview cleanup actions without touching files or processes")
	cmd("upgrade", "Upgrade swapaudit to the latest released version")
	fmt.Println()
}

// PrintUsage renders the CLI help and flag usage documentation.
func PrintUsage(w io.Writer) {
	fmt.Fprintln(w, "swapaudit — Linux workstation swap, RAM, and in-memory cache auditor")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "  swapaudit [FLAGS]")
	fmt.Fprintln(w, "  swapaudit sample [--once|--stream] [FLAGS]")
	fmt.Fprintln(w, "  swapaudit upgrade")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "COMMANDS:")
	fmt.Fprintln(w, "  upgrade                   Check GitHub releases and upgrade swapaudit in place")
	fmt.Fprintln(w, "  sample                    Headless sampler with rates, attribution, and a gated ETA")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "AUDIT & DISPLAY FLAGS:")
	fmt.Fprintln(w, "  --sort <size|age>         Sort column for occupant table (default: size)")
	fmt.Fprintln(w, "  --limit <N>               Maximum rows to display in main table (default: 12)")
	fmt.Fprintln(w, "  --all                     Display all processes individually without aggregation")
	fmt.Fprintln(w, "  --max-age <duration>      Cutoff age for stale /tmp caches (default: 12h)")
	fmt.Fprintln(w, "  --idle-threshold <dur>    Idle time threshold for dormant agents (default: 2h)")
	fmt.Fprintln(w, "  --tmp-pattern <pattern>   Additional /tmp directory glob pattern (repeatable)")
	fmt.Fprintln(w, "  --no-color                Disable ANSI color styling")
	fmt.Fprintln(w, "  --ascii                   Use plain ASCII box borders instead of Unicode glyphs")
	fmt.Fprintln(w, "  --no-update-check         Skip background 24-hour release check")
	fmt.Fprintln(w, "  --json                    Emit one stable machine-readable JSON document")
	fmt.Fprintln(w, "  --no-command              Omit command lines from JSON output (requires --json)")
	fmt.Fprintln(w, "  --interactive             Force the interactive action picker (requires a terminal)")
	fmt.Fprintln(w, "  --no-interactive          Disable the interactive action picker")
	fmt.Fprintln(w, "  -v, --version             Print version metadata and exit")
	fmt.Fprintln(w, "  -h, --help                Show this help message and exit")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "CLEANUP ACTIONS:")
	fmt.Fprintln(w, "  --clean-untouched         Prune untouched compiler & build caches (> 12h old)")
	fmt.Fprintln(w, "  --clean-abandoned         Prune abandoned test runner directories (> 12h idle)")
	fmt.Fprintln(w, "  --clean-dormant           Stop dormant AI agent processes (> 2h idle)")
	fmt.Fprintln(w, "  --clean-orphaned          End orphaned agent processes whose parent shell died")
	fmt.Fprintln(w, "  --clean-tmp               Clean all stale /tmp caches (untouched + abandoned)")
	fmt.Fprintln(w, "  --clean-all               Execute all safe cleanup actions")
	fmt.Fprintln(w, "  --dry-run                 Simulate cleanup actions without touching files/processes")
	fmt.Fprintln(w, "  -y, --yes                 Bypass confirmation prompts during cleanup")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "EXAMPLES:")
	fmt.Fprintln(w, "  swapaudit                               # Run complete interactive swap audit")
	fmt.Fprintln(w, "  swapaudit --sort age --limit 20         # Inspect older swap occupant processes")
	fmt.Fprintln(w, "  swapaudit --dry-run --clean-all         # Preview what would be reclaimed")
	fmt.Fprintln(w, "  swapaudit --clean-tmp                   # Prune stale /tmp build and test caches")
	fmt.Fprintln(w, "  swapaudit --json --no-command           # Machine-readable document without command lines")
	fmt.Fprintln(w, "  swapaudit --interactive                 # Force the tiered action picker in a terminal")
	fmt.Fprintln(w, "  swapaudit sample --once                 # One JSON document with rates and movers")
	fmt.Fprintln(w, "  swapaudit sample --stream --interval=1s # NDJSON samples until a signal")
	fmt.Fprintln(w, "  swapaudit upgrade                       # Upgrade binary to latest release")
}
