//go:build linux

package watch

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jadmadi/swapaudit/internal/live"
	"github.com/jadmadi/swapaudit/internal/term"
)

const (
	topNameCells = 40
	maxBodyBytes = 512
)

// FormatTitle builds a plain-text notification title.
func FormatTitle(resource, severity string, eta *int64, pct float64) string {
	if eta != nil {
		return fmt.Sprintf("swapaudit: %s full in %s", resource, formatETA(*eta))
	}
	return fmt.Sprintf("swapaudit: %s %.1f%% and rising", resource, pct)
}

// FormatBody builds the three-line notification body.
func FormatBody(resource string, stats live.WindowStats, top *live.Mover, reclaimable int64, age time.Duration) string {
	series, used, total := metrics(stats, resource)
	pct := percent(used, total)
	line1 := fmt.Sprintf("%s/%s (%.1f%%) · %s/min",
		term.FormatBytes(used), term.FormatBytes(total), pct, signedPerMin(int64(series.RateBytesPerSec*60)))
	if stats.SustainedSeconds > 0 {
		line1 += fmt.Sprintf(" · sustained %ds", stats.SustainedSeconds)
	}

	line2 := "top grower: unaccounted (shared/kernel)"
	if top != nil {
		line2 = "top grower: " + formatTop(top, stats.Seconds)
	}

	line3 := "run 'swapaudit' to review reclaimable tiers"
	if resource == "tmpfs" {
		line3 = term.FormatBytes(reclaimable) + " reclaimable"
		if age > 0 {
			line3 += fmt.Sprintf(" (sampled %ds ago)", int(age/time.Second))
		}
		line3 += " · run 'swapaudit' to review"
	}
	return clampBody(line1 + "\n" + line2 + "\n" + line3)
}

// FormatStatus renders one plain-text status line.
func FormatStatus(resource string, stats live.WindowStats, top *live.Mover, pressureThreshold float64) string {
	at := stats.SampledAt
	if at.IsZero() {
		at = time.Now()
	}
	parts := []string{at.Format("15:04:05")}
	resources := []string{"swap", "tmpfs"}
	switch resource {
	case "swap":
		resources = []string{"swap"}
	case "tmpfs":
		resources = []string{"tmpfs"}
	}
	for _, res := range resources {
		series, used, total := metrics(stats, res)
		pct := percent(used, total)
		if series.Trend == live.TrendInsufficient && pct < pressureThreshold {
			continue
		}
		eta := "--"
		if series.ETASeconds != nil {
			eta = formatETA(*series.ETASeconds)
		}
		parts = append(parts, fmt.Sprintf("%s %.1f%% %s/min eta %s", res, pct, signedPerMin(int64(series.RateBytesPerSec*60)), eta))
	}
	if top != nil {
		perMin := top.DeltaBytes
		if stats.Seconds > 0 {
			perMin = top.DeltaBytes * 60 / int64(stats.Seconds)
		}
		parts = append(parts, "top "+term.Truncate(top.Name, topNameCells)+" "+signedPerMin(perMin))
	}
	return strings.Join(parts, " | ")
}

func formatTop(m *live.Mover, windowSeconds int) string {
	perMin := m.DeltaBytes
	if windowSeconds > 0 {
		perMin = m.DeltaBytes * 60 / int64(windowSeconds)
	}
	name := term.Truncate(m.Name, topNameCells)
	who := name
	if m.Kind != "cache" {
		pid := 0
		if m.PID != nil {
			pid = *m.PID
		}
		who = fmt.Sprintf("%s (PID %d)", name, pid)
	}
	return fmt.Sprintf("%s %s/min, now %s", who, signedPerMin(perMin), term.FormatBytes(m.CurrentBytes))
}

func signedPerMin(v int64) string {
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

func formatETA(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	if d >= time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func clampBody(s string) string {
	if len(s) <= maxBodyBytes {
		return s
	}
	cut := maxBodyBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
