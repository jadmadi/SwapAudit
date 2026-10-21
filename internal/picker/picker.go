//go:build linux

// Package picker renders the post-report tiered action menu and delegates all mutations.
package picker

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/term"
)

// Config injects every input, output, and side effect used by the picker.
type Config struct {
	In       *bufio.Reader
	Out      io.Writer
	Items    []model.SwapItem
	DryRun   bool
	Width    int
	Scan     func() []model.SwapItem
	Snapshot func() model.SystemSwap
	Execute  func(items []model.SwapItem, untouched, abandoned, dormant, orphaned, dryRun, yes bool)
}

type tier struct {
	key              int
	label, threshold string
	category         model.Category
	resource, safety string
	process          bool
}

var tiers = []tier{
	{1, "untouched caches (>12h)", "threshold: newest file activity older than 12h", model.CatUntouchedCache, "RAM", "rebuildable", false},
	{2, "abandoned runs (>12h idle)", "threshold: no file activity for more than 12h", model.CatAbandonedRun, "RAM", "rebuildable", false},
	{3, "dormant agents (>2h idle)", "threshold: process idle for more than 2h", model.CatDormantAgent, "swap", "irreversible", true},
	{4, "orphaned agents (parent gone)", "threshold: parent process gone (reparented to init/systemd)", model.CatAbandonedAgent, "swap", "irreversible", true},
}

const maxReviewRows, maxAttempts = 10, 3

type printer struct {
	w   io.Writer
	err error
}

func (p *printer) printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, args...)
}

func (p *printer) code() int {
	if p.err != nil {
		return 1
	}
	return 0
}

// Run drives the menu until quit or EOF and returns a process exit code.
func Run(cfg Config) int {
	if cfg.Out == nil {
		return 1
	}
	p := &printer{w: cfg.Out}
	if cfg.In == nil || cfg.Scan == nil || cfg.Snapshot == nil || cfg.Execute == nil {
		p.printf("%s\n", "error: picker requires an input reader, scan, snapshot, and executor")
		return 1
	}

	width := cfg.Width
	if width < 80 {
		width = 80
	}
	if width > 120 {
		width = 120
	}
	items := cfg.Items
	dryRun := cfg.DryRun

	renderMenu(p, items, dryRun, width)
	for {
		if p.err != nil {
			return 1
		}
		p.printf("Select a tier [1-4], [d] dry-run, [q] quit: ")
		line, err := cfg.In.ReadString('\n')
		if err != nil && line == "" {
			return p.code()
		}
		choice := strings.TrimSpace(line)

		switch strings.ToLower(choice) {
		case "":
			return p.code()
		case "q":
			return p.code()
		case "d":
			dryRun = !dryRun
			state := "off"
			if dryRun {
				state = "on"
			}
			p.printf("%s\n", "dry run: "+state)
			renderMenu(p, items, dryRun, width)
		case "1", "2", "3", "4":
			key := int(choice[0] - '0')
			var quit bool
			items, dryRun, quit = runTier(p, cfg, items, dryRun, width, key)
			if quit {
				return p.code()
			}
		default:
			p.printf("invalid selection: %s\n", choice)
		}
		if p.err != nil {
			return 1
		}
	}
}

func runTier(p *printer, cfg Config, items []model.SwapItem, dryRun bool, width, key int) ([]model.SwapItem, bool, bool) {
	t := tiers[key-1]
	targets := targetsFor(items, t.category)
	if len(targets) == 0 {
		p.printf("tier %d: nothing to clean\n\n", key)
		return items, dryRun, false
	}

	renderReview(p, t, targets, width)
	token := tokenFor(t, targets)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		p.printf("Type %s to confirm, or press Enter to cancel: ", token)
		line, err := cfg.In.ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			return items, dryRun, true
		}
		answer := strings.TrimSpace(line)
		if answer == "" {
			p.printf("%s\n", "cancelled.")
			return items, dryRun, false
		}
		if strings.EqualFold(answer, "q") {
			return items, dryRun, true
		}
		if answer != token {
			p.printf("mismatch: expected %s — nothing was changed.\n", token)
			continue
		}

		before := cfg.Snapshot()
		cfg.Execute(items, key == 1, key == 2, key == 3, key == 4, dryRun, true)
		if dryRun {
			renderMenu(p, items, dryRun, width)
			return items, dryRun, false
		}

		after := cfg.Snapshot()
		fresh := cfg.Scan()
		p.printf("%s\n", measuredLine(before, after))
		renderMenu(p, fresh, dryRun, width)
		return fresh, dryRun, false
	}

	p.printf("%s\n", "cancelled after 3 attempts — nothing was changed.")
	return items, dryRun, false
}

func targetsFor(items []model.SwapItem, category model.Category) []model.SwapItem {
	var out []model.SwapItem
	for _, it := range items {
		if it.Category == category && it.Cleanable {
			out = append(out, it)
		}
	}
	return out
}

func tokenFor(t tier, targets []model.SwapItem) string {
	if !t.process {
		return strconv.Itoa(len(targets))
	}
	best := targets[0]
	for _, it := range targets[1:] {
		if it.SwapBytes > best.SwapBytes || (it.SwapBytes == best.SwapBytes && it.PID < best.PID) {
			best = it
		}
	}
	return strconv.Itoa(best.PID)
}

func renderMenu(p *printer, items []model.SwapItem, dryRun bool, width int) {
	title := "ACTION PICKER"
	if dryRun {
		title += " (dry run)"
	}
	p.printf("\n%s\n", title)

	tierWidth := width - 54
	if tierWidth < 12 {
		tierWidth = 12
	}
	widths := []int{3, tierWidth, 5, 9, 8, 12}
	right := []bool{false, false, true, true, false, false}
	p.printf("%s\n", formatRow([]string{"KEY", "TIER", "ITEMS", "SIZE", "RESOURCE", "SAFETY"}, widths, right))
	p.printf("%s\n", strings.Repeat("-", width))

	for _, t := range tiers {
		targets := targetsFor(items, t.category)
		size := "—"
		if len(targets) > 0 {
			size = term.FormatBytes(totalBytes(targets))
		}
		cells := []string{strconv.Itoa(t.key), t.label, strconv.Itoa(len(targets)), size, t.resource, t.safety}
		p.printf("%s\n", formatRow(cells, widths, right))
	}
	p.printf("%s\n", "")
}

func totalBytes(items []model.SwapItem) (total int64) {
	for _, it := range items {
		total += model.ItemSize(it)
	}
	return
}

func renderReview(p *printer, t tier, targets []model.SwapItem, width int) {
	p.printf("\nREVIEW tier %d of 4 — %s\n", t.key, t.label)
	p.printf("%s\n", term.Truncate(t.threshold, width))

	sorted := append([]model.SwapItem(nil), targets...)
	sort.SliceStable(sorted, func(i, j int) bool {
		si, sj := model.ItemSize(sorted[i]), model.ItemSize(sorted[j])
		if si != sj {
			return si > sj
		}
		if sorted[i].Name != sorted[j].Name {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].PID < sorted[j].PID
	})

	headers, right := []string{"TARGET", "SIZE", "IDLE", "AGE"}, []bool{false, true, true, true}
	widths := []int{0, 9, 8, 8}
	if t.process {
		headers, right, widths = []string{"PID", "TARGET", "SIZE", "IDLE"}, []bool{false, false, true, true}, []int{8, 0, 9, 8}
	}
	targetWidth := width - 2 - 25 - (len(headers)-1)*3
	if targetWidth < 12 {
		targetWidth = 12
	}
	if t.process {
		widths[1] = targetWidth
	} else {
		widths[0] = targetWidth
	}

	p.printf("%s\n", formatRow(headers, widths, right))
	p.printf("%s\n", strings.Repeat("-", width))

	shown := 0
	var hiddenBytes int64
	for _, it := range sorted {
		if shown >= maxReviewRows {
			hiddenBytes += model.ItemSize(it)
			continue
		}
		shown++
		pid := "-"
		if it.PID > 0 {
			pid = strconv.Itoa(it.PID)
		}
		size := term.FormatBytes(model.ItemSize(it))
		idle := term.FormatDuration(it.LastActive)
		if t.process {
			p.printf("%s\n", formatRow([]string{pid, it.Name, size, idle}, widths, right))
		} else {
			p.printf("%s\n", formatRow([]string{it.Name, size, idle, term.FormatDuration(it.Age)}, widths, right))
		}
	}
	if len(sorted) > shown {
		p.printf("%s\n", term.Truncate(fmt.Sprintf("… and %d more targets (%s)", len(sorted)-shown, term.FormatBytes(hiddenBytes)), width))
	}

	total := term.FormatBytes(totalBytes(targets))
	frees, risk := "FREES "+total+" RAM · swap unchanged", "RISK rebuildable: caches are regenerated on demand"
	if t.process {
		frees, risk = "FREES "+total+" swap · RAM unchanged", "RISK irreversible: running processes will be terminated"
	}
	p.printf("%s\n", term.Truncate(frees, width))
	p.printf("%s\n", term.Truncate(risk, width))
	p.printf("%s\n", "")
}

func formatRow(cells []string, widths []int, right []bool) string {
	parts := make([]string, len(cells))
	for i := range cells {
		val := term.Truncate(cells[i], widths[i])
		pad := widths[i] - term.StrWidth(val)
		if pad < 0 {
			pad = 0
		}
		if right[i] {
			parts[i] = strings.Repeat(" ", pad) + val
		} else {
			parts[i] = val + strings.Repeat(" ", pad)
		}
	}
	return "  " + strings.Join(parts, " | ")
}

func measuredLine(before, after model.SystemSwap) string {
	return fmt.Sprintf(
		"swap %s/%s (%s) -> %s/%s (%s) · tmpfs %s/%s -> %s/%s · delta swap %s · delta tmpfs %s",
		term.FormatBytes(before.SwapUsed), term.FormatBytes(before.SwapTotal), pct(before.SwapUsed, before.SwapTotal),
		term.FormatBytes(after.SwapUsed), term.FormatBytes(after.SwapTotal), pct(after.SwapUsed, after.SwapTotal),
		term.FormatBytes(before.TmpfsUsed), term.FormatBytes(before.TmpfsSize),
		term.FormatBytes(after.TmpfsUsed), term.FormatBytes(after.TmpfsSize),
		signedBytes(after.SwapUsed-before.SwapUsed),
		signedBytes(after.TmpfsUsed-before.TmpfsUsed),
	)
}

func pct(used, total int64) string {
	if total <= 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", float64(used)/float64(total)*100)
}

func signedBytes(v int64) string {
	if v == 0 {
		return "0 B"
	}
	sign := "+"
	if v < 0 {
		sign = "-"
		v = -v
	}
	return sign + term.FormatBytes(v)
}
