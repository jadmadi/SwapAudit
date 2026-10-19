package triage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

// CategorizeProcess classifies a process into desktop daemon, agent, or active app.
// It detects orphaned agent sessions (parent PID deceased) and dormant agent sessions.
func CategorizeProcess(comm string, argv []string, pid, ppid int, lastActive, idleThreshold time.Duration) (model.Category, bool) {
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
			return model.CatDesktopDaemon, false
		}
	}

	isAgent := comm == "agy" || comm == "devin" || comm == "chrome-devtools-mcp" || comm == "chrome-devtools" ||
		binName == "agy" || binName == "devin" || binName == "antigravity" || binName == "chrome-devtools-mcp"

	if !isAgent {
		for _, arg := range argv {
			if arg == "--conversation" || arg == "chrome-devtools-mcp" || filepath.Base(arg) == "chrome-devtools-mcp" {
				isAgent = true
				break
			}
		}
	}

	if isAgent {
		if ppid > 1 {
			if _, err := os.Stat(fmt.Sprintf("/proc/%d", ppid)); os.IsNotExist(err) {
				return model.CatAbandonedAgent, true
			}
		}
		if lastActive >= 0 && lastActive > idleThreshold {
			return model.CatDormantAgent, true
		}
		return model.CatActiveProcess, false
	}

	return model.CatActiveProcess, false
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
		filtered = append(filtered, *g)
	}

	return filtered
}
