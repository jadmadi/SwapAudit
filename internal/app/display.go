//go:build linux

package app

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/report"
	"github.com/jadmadi/swapaudit/internal/term"
	"github.com/jadmadi/swapaudit/internal/triage"
)

// PrintHeader renders the system overview, verdict, and reconciliation lines.
func PrintHeader(r report.AuditReport) {
	term.PrintBoxHeader("SWAP & IN-MEMORY CACHE AUDITOR")

	barWidth := 16
	s := r.System

	if s.SwapTotal <= 0 {
		fmt.Printf("  %-8s Swap not configured\n", "Swap")
	} else {
		swapPct := r.SwapPercent()
		bar := term.RenderMeterBar(swapPct, barWidth)
		fmt.Printf("  %-8s %s %5.1f%% %8s / %8s (free %s)\n",
			"Swap", bar, swapPct, term.FormatBytes(s.SwapUsed), term.FormatBytes(s.SwapTotal), term.FormatBytes(s.SwapFree))
	}

	ramPct := 0.0
	if s.MemTotal > 0 {
		ramPct = (float64(s.MemUsed) / float64(s.MemTotal)) * 100.0
	}
	ramBar := term.RenderMeterBar(ramPct, barWidth)
	fmt.Printf("  %-8s %s %5.1f%% %8s / %8s (avail %s)\n",
		"RAM used", ramBar, ramPct, term.FormatBytes(s.MemUsed), term.FormatBytes(s.MemTotal), term.FormatBytes(s.MemAvail))

	tmpPct := 0.0
	if s.TmpfsSize > 0 {
		tmpPct = (float64(s.TmpfsUsed) / float64(s.TmpfsSize)) * 100.0
	}
	tmpBar := term.RenderMeterBar(tmpPct, barWidth)
	fmt.Printf("  %-8s %s %5.1f%% %8s / %8s (free %s)\n",
		"/tmp", tmpBar, tmpPct, term.FormatBytes(s.TmpfsUsed), term.FormatBytes(s.TmpfsSize), term.FormatBytes(s.TmpfsFree))
	fmt.Printf("  %s\n", term.Dim(term.PressureLegend()))
	fmt.Println()

	printWrapped("  ", FormatVerdict(r), term.TermWidth())
	fmt.Println()

	fmt.Printf("  Swap:  %s attributed + %s unattributed = %s used\n",
		term.FormatBytes(r.Swap.AttributedBytes),
		term.FormatBytes(r.Swap.UnattributedBytes),
		term.FormatBytes(r.Swap.TotalBytes))
	if r.Swap.OverlapBytes > 0 {
		fmt.Printf("         overlap: %s (shared swap accounting exceeds the kernel total)\n", term.FormatBytes(r.Swap.OverlapBytes))
	}
	fmt.Printf("  /tmp:  %s attributed + %s unattributed = %s used\n",
		term.FormatBytes(r.Tmpfs.AttributedBytes),
		term.FormatBytes(r.Tmpfs.UnattributedBytes),
		term.FormatBytes(r.Tmpfs.TotalBytes))
	if r.Tmpfs.OverlapBytes > 0 {
		fmt.Printf("         overlap: %s (scan/statfs race)\n", term.FormatBytes(r.Tmpfs.OverlapBytes))
	}
	printWrapped("  unattributed reasons: ", report.FormatReasons(mergeReasons(r.Swap.Reasons, r.Tmpfs.Reasons)), term.TermWidth())
	if err := r.Reconciles(); err != nil {
		fmt.Printf("  %s %s\n", term.Yellow("WARN"), term.Red("Reconciles: FAIL: "+err.Error()))
	} else {
		fmt.Printf("  Reconciles: %s\n", term.Green("OK"))
	}
	fmt.Println()
}

func mergeReasons(a, b []report.ReasonBytes) []report.ReasonBytes {
	merged := make(map[string]int64)
	for _, rr := range a {
		merged[rr.Reason] += rr.Bytes
	}
	for _, rr := range b {
		merged[rr.Reason] += rr.Bytes
	}
	out := make([]report.ReasonBytes, 0, len(merged))
	for reason, bytes := range merged {
		out = append(out, report.ReasonBytes{Reason: reason, Bytes: bytes})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

// FormatVerdict returns the one-line answer derived from the report.
func FormatVerdict(r report.AuditReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Verdict: Swap %.1f%% used", r.SwapPercent())

	parts := make([]string, 0, 2)
	if r.Reclaim.TmpfsBytes > 0 {
		parts = append(parts, term.FormatBytes(r.Reclaim.TmpfsBytes)+" tmpfs")
	}
	if r.Reclaim.SwapBytes > 0 {
		parts = append(parts, term.FormatBytes(r.Reclaim.SwapBytes)+" swap")
	}
	if len(parts) == 0 {
		b.WriteString(" · nothing safe to reclaim")
		var residual []string
		if r.Swap.UnattributedBytes > 0 {
			residual = append(residual, term.FormatBytes(r.Swap.UnattributedBytes)+" swap")
		}
		if r.Tmpfs.UnattributedBytes > 0 {
			residual = append(residual, term.FormatBytes(r.Tmpfs.UnattributedBytes)+" /tmp")
		}
		if len(residual) > 0 {
			b.WriteString(" · " + strings.Join(residual, " and ") + " unattributed")
		}
	} else {
		total := r.Reclaim.TmpfsBytes + r.Reclaim.SwapBytes
		fmt.Fprintf(&b, " · %s reclaimable (%s; upper bound; pools may overlap)",
			term.FormatBytes(total), strings.Join(parts, " + "))
		if best, ok := r.BiggestWin(); ok {
			identity := best.Name
			if best.PID > 0 {
				identity = fmt.Sprintf("%s PID %d", best.Name, best.PID)
			}
			fmt.Fprintf(&b, " · biggest win: %s (%s)", identity, term.FormatBytes(model.ItemSize(best)))
		}
	}
	fmt.Fprintf(&b, " · thresholds: cache %s, agent %s, orphan-cache %s",
		term.FormatDuration(r.Thresholds.MaxAge),
		term.FormatDuration(r.Thresholds.IdleThreshold),
		term.FormatDuration(r.Thresholds.OrphanCacheAge))
	return b.String()
}

// PrintActionableSection renders only cleanable groups/items.
func PrintActionableSection(r report.AuditReport, opts ViewOptions) {
	var cleanables []model.SwapItem
	for _, it := range r.Items {
		if it.Cleanable {
			cleanables = append(cleanables, it)
		}
	}

	term.PrintBoxHeader("Actionable Reclaim Candidates")
	if len(cleanables) == 0 {
		fmt.Printf("  %s\n\n", term.Dim(term.GlyphCheck()+" nothing safe to reclaim"))
		return
	}
	fmt.Printf("  Reclaimable: %s tmpfs · %s swap %s\n\n",
		term.FormatBytes(r.Reclaim.TmpfsBytes), term.FormatBytes(r.Reclaim.SwapBytes),
		term.Dim("(upper bound; pools may overlap)"))

	groupEnabled := !opts.NoGroup && !opts.ShowAll
	groups := triage.GroupItems(cleanables, groupEnabled)
	groups = triage.ExpandMatching(groups, opts.Expand)

	w := term.TermWidth()
	var cols []model.Col
	flex := 1
	if w >= 100 {
		cols = []model.Col{
			{Title: "ID", Width: 8},
			{Title: "Target / Name", Width: 10},
			{Title: "Category", Width: 15},
			{Title: "Idle", Width: 12, Right: true},
			{Title: "Action", Width: 21},
			{Title: "Reclaim", Width: 10, Right: true},
		}
	} else {
		cols = []model.Col{
			{Title: "ID", Width: 8},
			{Title: "Target / Name", Width: 10},
			{Title: "Idle", Width: 12, Right: true},
			{Title: "Action", Width: 21},
			{Title: "Reclaim", Width: 10, Right: true},
		}
	}
	cols = term.FitColumns(cols, flex, w)

	shownRows := 0
	shownItems := 0
	hiddenGroups := 0
	hiddenItems := 0
	var hiddenBytes int64
	var rows [][]string
	for _, g := range groups {
		if opts.Limit > 0 && shownRows >= opts.Limit {
			hiddenGroups++
			hiddenItems += g.Count
			hiddenBytes += g.TotalBytes
			continue
		}
		shownRows++
		shownItems += g.Count
		name := g.Key
		if g.Count > 1 {
			name = fmt.Sprintf("%s ×%d", g.Key, g.Count)
		} else {
			name = g.Representative.Name
		}
		if !opts.Wide {
			name = term.TruncateMiddle(name, cols[flex].Width)
		}
		id := displayID(g.Representative, g.Count)
		idle := idleRange(g)
		row := []string{id, name, idle, actionFor(g.Category), term.FormatBytes(g.TotalBytes)}
		if w >= 100 {
			row = []string{id, name, string(g.Category), idle, actionFor(g.Category), term.FormatBytes(g.TotalBytes)}
		}
		rows = append(rows, row)
	}
	term.RenderTable(cols, rows)

	if hiddenGroups > 0 {
		printWrapped("  ", fmt.Sprintf("... and %d more groups (%d items, %s). Run with --all to inspect the full list.",
			hiddenGroups, hiddenItems, term.FormatBytes(hiddenBytes)), term.TermWidth())
	}
	printWrapped("  ", fmt.Sprintf("Actionable population: %d cleanable · %d shown (%d rows) · %d hidden (%s) · limit %s rows",
		r.Population.Cleanable, shownItems, shownRows, hiddenItems, term.FormatBytes(hiddenBytes), limitLabel(opts.Limit)), term.TermWidth())
	fmt.Println()
}

type protectedViewData struct {
	groups           []triage.Group
	raw              int
	aggregated       int
	shown            int
	hiddenBySize     int
	hiddenByLimit    int
	hiddenLimitBytes int64
	shellCount       int
	shellRows        int
}

// buildProtectedView applies aggregation, sorting, filtering, grouping, and the
// row limit exactly once so the human table and JSON population agree.
func buildProtectedView(r report.AuditReport, opts ViewOptions) protectedViewData {
	var items []model.SwapItem
	for _, it := range r.Items {
		if !it.Cleanable {
			items = append(items, it)
		}
	}
	d := protectedViewData{raw: len(items)}
	if !opts.ShowAll {
		for _, it := range items {
			if it.PID > 0 && it.SwapBytes < 20*1024*1024 && (it.Name == "bash" || it.Name == "sh" || it.Name == "zsh") {
				d.shellCount++
			}
		}
		items = triage.AggregateShells(items)
		for _, it := range items {
			if strings.HasPrefix(it.Name, "bash (") || strings.HasPrefix(it.Name, "sh (") || strings.HasPrefix(it.Name, "zsh (") {
				d.shellRows++
			}
		}
	}
	d.aggregated = len(items)

	if opts.Sort == "age" {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Age != items[j].Age {
				return items[i].Age > items[j].Age
			}
			return items[i].Name < items[j].Name
		})
	} else {
		sort.SliceStable(items, func(i, j int) bool {
			if model.ItemSize(items[i]) != model.ItemSize(items[j]) {
				return model.ItemSize(items[i]) > model.ItemSize(items[j])
			}
			return items[i].Name < items[j].Name
		})
	}

	filtered := items[:0]
	for _, it := range items {
		if !opts.ShowAll && model.ItemSize(it) < 10*1024*1024 {
			d.hiddenBySize++
			continue
		}
		filtered = append(filtered, it)
	}
	items = filtered

	groupEnabled := !opts.NoGroup && !opts.ShowAll
	groups := triage.GroupItems(items, groupEnabled)
	groups = triage.ExpandMatching(groups, opts.Expand)

	shownRows := 0
	for _, g := range groups {
		if opts.Limit > 0 && shownRows >= opts.Limit {
			d.hiddenByLimit += g.Count
			d.hiddenLimitBytes += g.TotalBytes
			continue
		}
		d.groups = append(d.groups, g)
		shownRows++
		d.shown += g.Count
	}
	return d
}

// PrintProtectedTable renders only non-cleanable occupants.
func PrintProtectedTable(r report.AuditReport, opts ViewOptions) {
	d := buildProtectedView(r, opts)
	rawCount := d.raw
	aggregatedCount := d.aggregated
	hiddenBySize := d.hiddenBySize
	hiddenByLimit := d.hiddenByLimit
	hiddenLimitBytes := d.hiddenLimitBytes
	shownItems := d.shown
	groups := d.groups

	title := "Protected Occupants (Ranked by " + strings.ToUpper(opts.Sort) + ")"
	term.PrintBoxHeader(title)

	w := term.TermWidth()
	var cols []model.Col
	flex := 1
	ageCol := model.Col{Title: "Age", Width: 10, Right: true}
	idleCol := model.Col{Title: "Idle", Width: 12, Right: true}
	if w >= 100 {
		cols = []model.Col{
			{Title: "ID", Width: 8},
			{Title: "Target / Command", Width: 10},
			{Title: "Category", Width: 16},
			ageCol,
			idleCol,
			{Title: "Size", Width: 10, Right: true},
		}
	} else {
		flex = 0
		cols = []model.Col{
			{Title: "Target / Command", Width: 10},
			ageCol,
			idleCol,
			{Title: "Size", Width: 10, Right: true},
		}
	}
	cols = term.FitColumns(cols, flex, w)

	var rows [][]string
	for _, g := range groups {
		name := g.Representative.Name
		if g.Count > 1 {
			name = fmt.Sprintf("%s ×%d", g.Key, g.Count)
		}
		if !opts.Wide {
			name = term.TruncateMiddle(name, cols[flex].Width)
		}
		id := displayID(g.Representative, g.Count)
		age := term.FormatDuration(g.Representative.Age)
		idle := idleRange(g)
		size := term.FormatBytes(g.TotalBytes)
		if w >= 100 {
			rows = append(rows, []string{id, name, string(g.Category), age, idle, size})
		} else {
			rows = append(rows, []string{name, age, idle, size})
		}
	}
	term.RenderTable(cols, rows)

	if hiddenBySize > 0 || hiddenByLimit > 0 {
		printWrapped("  ", fmt.Sprintf("... and %d more hidden (%d below 10 MB, %d beyond --limit; %s). Run with --all to inspect the full list.",
			hiddenBySize+hiddenByLimit, hiddenBySize, hiddenByLimit, term.FormatBytes(hiddenLimitBytes)), term.TermWidth())
	}
	printWrapped("  ", fmt.Sprintf("Population: %d raw · %d after shell aggregation · %d shown · %d hidden below 10 MB · %d beyond --limit",
		rawCount, aggregatedCount, shownItems, hiddenBySize, hiddenByLimit), term.TermWidth())
	fmt.Println()
}

func displayID(it model.SwapItem, groupCount int) string {
	if groupCount > 1 {
		return ""
	}
	if it.PID > 0 {
		return term.Yellow(term.GlyphActiveIndicator() + " " + strconv.Itoa(it.PID))
	}
	if it.Path != "" {
		return model.ShortID(it)
	}
	return ""
}

func idleRange(g triage.Group) string {
	if g.Count > 1 && g.MinIdle != g.MaxIdle && g.MinIdle >= 0 && g.MaxIdle >= 0 {
		lo := strings.TrimSuffix(term.FormatDuration(g.MinIdle), "m")
		hi := strings.TrimSuffix(term.FormatDuration(g.MaxIdle), "m")
		return lo + "–" + hi
	}
	if !g.Representative.LastActiveKnown && g.Representative.Kind == model.KindProcess {
		return "unknown"
	}
	return term.FormatDuration(g.Representative.LastActive)
}

func limitLabel(limit int) string {
	if limit <= 0 {
		return "none"
	}
	return strconv.Itoa(limit)
}

func actionFor(c model.Category) string {
	switch c {
	case model.CatUntouchedCache:
		return "Prune untouched cache"
	case model.CatAbandonedRun:
		return "Prune abandoned run"
	case model.CatAbandonedAgent:
		return "End orphaned process"
	case model.CatDormantAgent:
		return "End dormant session"
	case model.CatOrphanedCache:
		return "Prune orphaned cache"
	case model.CatInUseCache:
		return "In use"
	case model.CatRecentCache:
		return "Recent"
	default:
		return ""
	}
}

// PrintCategorySummary renders the category breakdown with residual rows.
func PrintCategorySummary(r report.AuditReport) {
	term.PrintBoxHeader("Category Breakdown & Actionable Totals")

	w := term.TermWidth()
	cols := []model.Col{
		{Title: "Category", Width: 10},
		{Title: "Entities", Width: 8, Right: true},
		{Title: "Size", Width: 10, Right: true},
		{Title: "Share (attributed)", Width: 19},
		{Title: "Actionable", Width: 10, Right: true},
	}
	cols = term.FitColumns(cols, 0, w)

	var rows [][]string
	rows = append(rows, []string{term.Bold("Process swap (VmSwap) · dormant > " + term.FormatDuration(r.Thresholds.IdleThreshold)), "", "", "", ""})
	processCats := []model.Category{model.CatDormantAgent, model.CatAbandonedAgent, model.CatDesktopDaemon, model.CatActiveProcess}
	for _, c := range processCats {
		ct := r.Category(c)
		if c != model.CatAbandonedAgent && ct.Entities == 0 && ct.AttributedBytes == 0 {
			continue
		}
		rows = append(rows, categoryRow(ct))
	}
	rows = append(rows, residualRow(r.Swap))

	rows = append(rows, []string{term.Bold("/tmp allocated (tmpfs) · stale > " + term.FormatDuration(r.Thresholds.MaxAge) + " · orphan-cache > " + term.FormatDuration(r.Thresholds.OrphanCacheAge)), "", "", "", ""})
	cacheCats := []model.Category{model.CatUntouchedCache, model.CatAbandonedRun, model.CatOrphanedCache, model.CatInUseCache, model.CatRecentCache}
	for _, c := range cacheCats {
		ct := r.Category(c)
		if c != model.CatOrphanedCache && c != model.CatAbandonedRun && ct.Entities == 0 && ct.AttributedBytes == 0 {
			continue
		}
		rows = append(rows, categoryRow(ct))
	}
	rows = append(rows, residualRow(r.Tmpfs))

	term.RenderTable(cols, rows)

	printWrapped("  ", fmt.Sprintf("Population: %d entities · %d cleanable · %d protected · %s swap unattributed · %s /tmp unattributed",
		r.Population.RawItems, r.Population.Cleanable, r.Population.Protected,
		term.FormatBytes(r.Swap.UnattributedBytes), term.FormatBytes(r.Tmpfs.UnattributedBytes)), w)
	printWrapped("  ", fmt.Sprintf("Agents: %d detected · %d with swap · %d zero-swap · %d orphaned · %d dormant · %d idle-unknown · %d vanished",
		r.Population.Agents, r.Population.AgentsWithSwap, r.Population.AgentsZeroSwap,
		r.Population.AgentsOrphaned, r.Population.AgentsDormant, r.Population.AgentsIdleUnknown, r.Population.Vanished), w)
	printWrapped("  ", fmt.Sprintf("Orphan scan: %d agents · %d orphaned (evidence-gated) · %d parent-alive · %d idle-unknown · %d active",
		r.Population.Agents, r.Population.AgentsOrphaned, r.Population.AgentsParentAlive,
		r.Population.AgentsIdleUnknown, r.Population.AgentsActive), w)
	orphaned := r.Category(model.CatOrphanedCache)
	printWrapped("  ", fmt.Sprintf("Cache liveness: %d in-use · %d orphaned (%s) · %d recent · %d evidence-incomplete · index %d PIDs / %d FDs",
		r.Scan.InUseCaches, r.Scan.OrphanedCaches, term.FormatBytes(orphaned.AttributedBytes),
		r.Scan.RecentCaches, r.Scan.EvidenceIncomplete, r.Scan.OpenIndexPIDs, r.Scan.OpenIndexFDs), w)
	fmt.Println()
	if r.Population.PermissionDenied > 0 {
		fmt.Printf("  %s WARN: %d processes could not be inspected; idle/orphan evidence is incomplete\n\n",
			term.Yellow(term.GlyphWarn()), r.Population.PermissionDenied)
	}
}

func categoryRow(ct report.CategoryTotal) []string {
	pct := ct.ShareOfAttributed * 100.0
	return []string{
		"  " + string(ct.Category),
		strconv.Itoa(ct.Entities),
		term.FormatBytes(ct.AttributedBytes),
		term.RenderShareMiniBar(pct, 10),
		term.FormatBytes(ct.ReclaimableBytes),
	}
}

func residualRow(a report.Attribution) []string {
	label := "  .. [unattributed]"
	value := a.UnattributedBytes
	if a.OverlapBytes > 0 {
		label = "  .. [overlap]"
		value = a.OverlapBytes
	}
	return []string{term.Dim(label), "", term.FormatBytes(value), "", term.GlyphDash()}
}

// PrintQuickCommands renders recommended cleanup CLI commands.
func PrintQuickCommands(r report.AuditReport) {
	term.PrintBoxHeader("Quick Maintenance Commands")
	cmd := func(flagName, comment string) {
		cyanFlag := term.Cyan(flagName)
		flagVisW := term.StrWidth(flagName)
		pad := 24 - flagVisW
		if pad < 1 {
			pad = 1
		}
		fmt.Printf("  swapaudit %s%s # %s\n", cyanFlag, strings.Repeat(" ", pad), comment)
	}

	cmd("--clean-untouched", "Prune stale build caches (> "+term.FormatDuration(r.Thresholds.MaxAge)+" untouched)")
	cmd("--clean-abandoned", "Prune abandoned test suites & review runs (> "+term.FormatDuration(r.Thresholds.MaxAge)+" idle)")
	cmd("--clean-dormant", "End dormant agent sessions (> "+term.FormatDuration(r.Thresholds.IdleThreshold)+" idle)")
	cmd("--clean-orphaned", "End evidence-gated orphaned agent sessions")
	cmd("--clean-orphaned-caches", "Prune orphaned caches (> "+term.FormatDuration(r.Thresholds.OrphanCacheAge)+" idle)")
	cmd("--clean-tmp", "Clean all stale /tmp caches (untouched + abandoned)")
	cmd("--clean-all", "Run untouched + abandoned + dormant cleanup")
	cmd("--include-orphaned", "Include orphaned agents with --clean-all")
	cmd("--include-orphaned-caches", "Include orphaned caches with --clean-all")
	cmd("--all-users", "Allow signaling other users' processes")
	cmd("--dry-run", "Preview cleanup actions without touching files or processes")
	cmd("--check-updates", "Check for a newer release (one request)")
	cmd("upgrade", "Upgrade swapaudit to the latest released version")
	fmt.Println()
}

// PrintAgentEvidence prints per-agent evidence, one wrapped line per agent.
func PrintAgentEvidence(r report.AuditReport, opts ViewOptions) {
	term.PrintBoxHeader("Agent Evidence")
	shown := 0
	for _, it := range r.Items {
		if !it.Agent {
			continue
		}
		if opts.Limit > 0 && shown >= opts.Limit {
			break
		}
		shown++
		parent := fmt.Sprintf("%d/%s", it.ParentPID, it.ParentComm)
		if it.ParentAlive {
			parent += " (alive)"
		} else {
			parent += " (dead)"
		}
		decision := string(it.Category)
		if !it.Cleanable {
			decision += " (protected)"
		}
		line := fmt.Sprintf("PID %d %s · parent %s · idle %s · swap %s · tty %d · cgroup %s · reason %s · decision %s",
			it.PID, it.Name, parent, idleString(it), term.FormatBytes(it.SwapBytes), it.TTYNr,
			emptyDash(it.Cgroup), emptyDash(it.OrphanReason), decision)
		printWrapped("  ", line, term.TermWidth())
	}
	if shown == 0 {
		fmt.Printf("  %s\n", term.Dim("No agent processes detected."))
	}
	fmt.Println()
}

// PrintCacheEvidence prints per-cache liveness evidence.
func PrintCacheEvidence(r report.AuditReport, opts ViewOptions) {
	term.PrintBoxHeader("Cache Liveness Evidence")
	shown := 0
	for _, it := range r.Items {
		if it.Kind != model.KindCache {
			continue
		}
		if opts.Limit > 0 && shown >= opts.Limit {
			break
		}
		shown++
		line := fmt.Sprintf("%s · %s · size %s · %s · kind %s · handles %d · cwd %t · mapped %t · decision %s",
			it.Name, string(it.Category), term.FormatBytes(it.SizeBytes), idleString(it),
			string(it.CacheKind), it.OpenHandles, it.CwdInside, it.MappedInside, emptyDash(it.Evidence))
		printWrapped("  ", line, term.TermWidth())
	}
	if shown == 0 {
		fmt.Printf("  %s\n", term.Dim("No caches detected."))
	}
	fmt.Println()
}

func idleString(it model.SwapItem) string {
	if !it.LastActiveKnown && it.Kind == model.KindProcess {
		return "unknown"
	}
	return term.FormatDuration(it.LastActive)
}

func emptyDash(s string) string {
	if s == "" {
		return term.GlyphDash()
	}
	return s
}

func printWrapped(prefix, text string, width int) {
	if width <= 0 {
		width = 100
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		fmt.Println(prefix)
		return
	}
	line := prefix
	indent := strings.Repeat(" ", term.StrWidth(prefix))
	for _, w := range words {
		candidate := line + w
		if line != prefix {
			candidate = line + " " + w
		}
		if term.StrWidth(candidate) > width && line != prefix {
			fmt.Println(line)
			line = indent + w
		} else {
			line = candidate
		}
	}
	fmt.Println(line)
}

// PrintUsage renders the CLI help and flag usage documentation.
func PrintUsage(w io.Writer) {
	fmt.Fprintln(w, "swapaudit — Linux workstation swap, RAM, and in-memory cache auditor")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "  swapaudit [FLAGS]")
	fmt.Fprintln(w, "  swapaudit upgrade")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "COMMANDS:")
	fmt.Fprintln(w, "  upgrade                        Check GitHub releases and upgrade swapaudit in place")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "AUDIT & DISPLAY FLAGS:")
	fmt.Fprintln(w, "  --sort <size|age>              Sort column for occupant table (default: size)")
	fmt.Fprintln(w, "  --limit <N>                    Maximum rows per table (default: 12; 0 = unlimited)")
	fmt.Fprintln(w, "  --all                          Show all items; disables aggregation and the 10 MB filter")
	fmt.Fprintln(w, "  --no-group                     Expand grouped cache/process patterns")
	fmt.Fprintln(w, "  --expand <key>                 Expand one group key (prefix match)")
	fmt.Fprintln(w, "  --collapse                     Collapse groups (default)")
	fmt.Fprintln(w, "  --wide, --no-truncate          Show full names; may exceed the terminal width")
	fmt.Fprintln(w, "  --explain-agents               Print per-agent evidence")
	fmt.Fprintln(w, "  --explain-caches               Print per-cache liveness evidence")
	fmt.Fprintln(w, "  --max-age <duration>           Cutoff age for stale /tmp caches (default: 12h)")
	fmt.Fprintln(w, "  --idle-threshold <dur>         Idle threshold for dormant agents (default: 2h)")
	fmt.Fprintln(w, "  --orphan-cache-age <dur>       Idle threshold for orphaned caches (default: 1h; <= max-age)")
	fmt.Fprintln(w, "  --allow-generic-caches         Permit generic cache classification and cleanup")
	fmt.Fprintln(w, "  --tmp-pattern <pattern>        Additional /tmp directory glob pattern (repeatable)")
	fmt.Fprintln(w, "  --json                         Emit one machine-readable JSON object on stdout")
	fmt.Fprintln(w, "  --no-color                     Disable ANSI color styling")
	fmt.Fprintln(w, "  --ascii                        Use plain ASCII box borders instead of Unicode glyphs")
	fmt.Fprintln(w, "  --check-updates                Check GitHub for a newer release (one request; see PRIVACY.md)")
	fmt.Fprintln(w, "  -v, --version                  Print version metadata and exit")
	fmt.Fprintln(w, "  -h, --help                     Show this help message and exit")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "CLEANUP ACTIONS:")
	fmt.Fprintln(w, "  --clean-untouched              Prune untouched compiler & build caches (> max-age)")
	fmt.Fprintln(w, "  --clean-abandoned              Prune abandoned test runner directories (> max-age idle)")
	fmt.Fprintln(w, "  --clean-dormant                Stop dormant AI agent processes (> idle-threshold idle)")
	fmt.Fprintln(w, "  --clean-orphaned               End evidence-gated orphaned agent processes")
	fmt.Fprintln(w, "  --clean-orphaned-caches        Prune orphaned caches with no open handles (opt-in)")
	fmt.Fprintln(w, "  --clean-tmp                    Clean all stale /tmp caches (untouched + abandoned)")
	fmt.Fprintln(w, "  --clean-all                    Execute untouched + abandoned + dormant cleanup")
	fmt.Fprintln(w, "  --include-orphaned             Include orphaned agent cleanup with --clean-all")
	fmt.Fprintln(w, "  --include-orphaned-caches      Include orphaned cache cleanup with --clean-all")
	fmt.Fprintln(w, "  --allow-live-parent            Allow ending dormant agents whose parent is alive")
	fmt.Fprintln(w, "  --all-users                    Allow signaling processes owned by other UIDs")
	fmt.Fprintln(w, "  --dry-run                      Simulate cleanup actions without touching files/processes")
	fmt.Fprintln(w, "  -y, --yes                      Bypass confirmation prompts during cleanup")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "EXIT CODES:")
	fmt.Fprintln(w, "  0  success; 1  runtime failure; 2  usage error; 3  cleanup partial failure")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "NON-INTERACTIVE CLEANUP:")
	fmt.Fprintln(w, "  When stdin is not a terminal, cleanup requires --dry-run or --yes.")
	fmt.Fprintln(w, "  --json live cleanup also requires --yes or --dry-run.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "EXAMPLES:")
	fmt.Fprintln(w, "  swapaudit                               # Run complete interactive swap audit")
	fmt.Fprintln(w, "  swapaudit --sort age --limit 20         # Inspect older swap occupant processes")
	fmt.Fprintln(w, "  swapaudit --dry-run --clean-all         # Preview what would be reclaimed")
	fmt.Fprintln(w, "  swapaudit --json | jq .reclaim          # Machine-readable audit")
	fmt.Fprintln(w, "  swapaudit upgrade                       # Upgrade binary to latest release")
}
