package triage

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
)

// IsAgent reports whether a process belongs to the supported agent set.
func IsAgent(comm string, argv []string) bool {
	var binName string
	if len(argv) > 0 {
		binName = filepath.Base(argv[0])
	}
	if comm == "agy" || comm == "devin" || comm == "chrome-devtools-mcp" || comm == "chrome-devtools" ||
		binName == "agy" || binName == "devin" || binName == "antigravity" || binName == "chrome-devtools-mcp" {
		return true
	}
	for _, arg := range argv {
		if arg == "--conversation" || arg == "chrome-devtools-mcp" || filepath.Base(arg) == "chrome-devtools-mcp" {
			return true
		}
	}
	return false
}

// AgentDisplayName returns the compact display name for an agent process.
func AgentDisplayName(comm string, argv []string) string {
	for i, arg := range argv {
		if arg == "--conversation" && i+1 < len(argv) {
			targetID := argv[i+1]
			if runes := []rune(targetID); len(runes) > 8 {
				targetID = string(runes[:8])
			}
			return fmt.Sprintf("agy [%s]", targetID)
		}
	}
	for _, arg := range argv {
		if arg == "chrome-devtools-mcp" || filepath.Base(arg) == "chrome-devtools-mcp" {
			return "chrome-devtools"
		}
	}
	return comm
}

// IsDesktopDaemon reports whether a process is a known desktop/session daemon.
func IsDesktopDaemon(comm string, argv []string) bool {
	var binName string
	if len(argv) > 0 {
		binName = filepath.Base(argv[0])
	}
	desktopNames := []string{
		"plasmashell", "kwin", "kwin_wayland", "xorg", "xwayland", "kded6",
		"ksmserver", "kwalletd6", "kaccess", "polkit", "kdeconnectd",
		"xdg-desktop-portal", "systemd", "dbus-daemon", "pipewire",
	}
	for _, d := range desktopNames {
		if strings.EqualFold(comm, d) || strings.EqualFold(binName, d) {
			return true
		}
	}
	return false
}

// ClassifyAgent classifies a process and returns the evidence reason.
// Reasons: orphan-evidence, idle-threshold, idle-unknown, active, not-agent, desktop-daemon.
func ClassifyAgent(p proc.ProcessSnapshot, idleThreshold time.Duration) (model.Category, bool, string) {
	if IsDesktopDaemon(p.Comm, p.Argv) {
		return model.CatDesktopDaemon, false, "desktop-daemon"
	}
	if !IsAgent(p.Comm, p.Argv) {
		return model.CatActiveProcess, false, "not-agent"
	}
	if p.Orphaned {
		return model.CatAbandonedAgent, true, p.Orphan.Reason
	}
	if !p.IdleKnown {
		return model.CatActiveProcess, false, "idle-unknown"
	}
	if p.Idle > idleThreshold {
		return model.CatDormantAgent, true, "idle-threshold"
	}
	return model.CatActiveProcess, false, "active"
}

// CategorizeProcess is a compatibility wrapper that preserves the old signature.
func CategorizeProcess(comm string, argv []string, pid, ppid int, isOrphaned bool, lastActive, idleThreshold time.Duration) (model.Category, bool) {
	if IsDesktopDaemon(comm, argv) {
		return model.CatDesktopDaemon, false
	}
	if !IsAgent(comm, argv) {
		return model.CatActiveProcess, false
	}
	if isOrphaned {
		return model.CatAbandonedAgent, true
	}
	if lastActive >= 0 && lastActive > idleThreshold {
		return model.CatDormantAgent, true
	}
	return model.CatActiveProcess, false
}

// AgentPopulation counts the agent scan population.
type AgentPopulation struct {
	TotalProcesses      int
	AccessibleProcesses int
	Agents              int
	AgentsWithSwap      int
	AgentsZeroSwap      int
	DormantAgents       int
	OrphanedAgents      int
	ParentAliveAgents   int
	IdleUnknownAgents   int
	ActiveAgents        int
	Vanished            int
	PermissionDenied    int
}

// BuildAgentPopulation derives population counts from a full process scan.
func BuildAgentPopulation(result proc.ProcessScanResult, idleThreshold time.Duration) AgentPopulation {
	pop := AgentPopulation{
		TotalProcesses:      result.Total,
		AccessibleProcesses: result.Accessible,
		Vanished:            result.Vanished,
		PermissionDenied:    result.PermissionDenied,
	}
	for _, p := range result.Processes {
		if !IsAgent(p.Comm, p.Argv) {
			continue
		}
		pop.Agents++
		if p.VmSwap > 0 {
			pop.AgentsWithSwap++
		} else {
			pop.AgentsZeroSwap++
		}
		if p.ParentAlive {
			pop.ParentAliveAgents++
		}
		if !p.IdleKnown {
			pop.IdleUnknownAgents++
		}
		cat, _, _ := ClassifyAgent(p, idleThreshold)
		switch cat {
		case model.CatAbandonedAgent:
			pop.OrphanedAgents++
		case model.CatDormantAgent:
			pop.DormantAgents++
		default:
			pop.ActiveAgents++
		}
	}
	return pop
}

// AggregateShells groups small (<20MB) idle shell processes (bash, sh, zsh) to avoid display clutter.
func AggregateShells(items []model.SwapItem) []model.SwapItem {
	var filtered []model.SwapItem
	grouped := make(map[string]*model.SwapItem)

	for _, it := range items {
		if it.PID > 0 && it.SwapBytes < 20*1024*1024 && (it.Name == "bash" || it.Name == "sh" || it.Name == "zsh") {
			key := it.Name
			if g, ok := grouped[key]; ok {
				g.SwapBytes += it.SwapBytes
				g.SizeBytes += it.SizeBytes
				g.Count++
				if it.LastActive >= 0 {
					if g.LastActive < 0 || it.LastActive < g.LastActive {
						g.LastActive = it.LastActive
					}
				}
			} else {
				cp := it
				cp.Count = 1
				grouped[key] = &cp
			}
		} else {
			filtered = append(filtered, it)
		}
	}

	for _, g := range grouped {
		if g.Count == 1 {
			g.Name = fmt.Sprintf("%s (1 shell)", g.Name)
		} else {
			g.Name = fmt.Sprintf("%s (%d shells)", g.Name, g.Count)
		}
		g.PID = 0
		g.Kind = model.KindProcess
		filtered = append(filtered, *g)
	}

	return filtered
}
