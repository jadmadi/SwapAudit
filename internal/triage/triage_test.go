package triage

import (
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

func TestCategorizeProcess(t *testing.T) {
	idleThreshold := 2 * time.Hour

	tests := []struct {
		name          string
		comm          string
		argv          []string
		pid           int
		ppid          int
		isOrphaned    bool
		lastActive    time.Duration
		wantCategory  model.Category
		wantCleanable bool
	}{
		{
			name:          "desktop daemon plasmashell",
			comm:          "plasmashell",
			argv:          []string{"plasmashell"},
			pid:           100,
			ppid:          1,
			isOrphaned:    false,
			lastActive:    10 * time.Minute,
			wantCategory:  model.CatDesktopDaemon,
			wantCleanable: false,
		},
		{
			name:          "desktop daemon kwin_wayland",
			comm:          "kwin_wayland",
			argv:          []string{"/usr/bin/kwin_wayland"},
			pid:           101,
			ppid:          1,
			isOrphaned:    false,
			lastActive:    0,
			wantCategory:  model.CatDesktopDaemon,
			wantCleanable: false,
		},
		{
			name:          "active agent session",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           102,
			ppid:          1000,
			isOrphaned:    false,
			lastActive:    15 * time.Minute,
			wantCategory:  model.CatActiveProcess,
			wantCleanable: false,
		},
		{
			name:          "orphaned agent session whose parent died",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           103,
			ppid:          1,
			isOrphaned:    true,
			lastActive:    30 * time.Minute,
			wantCategory:  model.CatAbandonedAgent,
			wantCleanable: true,
		},
		{
			name:          "dormant agent session exceeding threshold",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           104,
			ppid:          1000,
			isOrphaned:    false,
			lastActive:    3 * time.Hour,
			wantCategory:  model.CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "agent session with unknown last active",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           105,
			ppid:          1000,
			isOrphaned:    false,
			lastActive:    -1,
			wantCategory:  model.CatActiveProcess,
			wantCleanable: false,
		},
		{
			name:          "dormant chrome devtools mcp",
			comm:          "chrome-devtools",
			argv:          []string{"chrome-devtools-mcp"},
			pid:           106,
			ppid:          1000,
			isOrphaned:    false,
			lastActive:    4 * time.Hour,
			wantCategory:  model.CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "abandoned chrome devtools mcp orphaned",
			comm:          "chrome-devtools",
			argv:          []string{"chrome-devtools-mcp"},
			pid:           107,
			ppid:          1,
			isOrphaned:    true,
			lastActive:    10 * time.Minute,
			wantCategory:  model.CatAbandonedAgent,
			wantCleanable: true,
		},
		{
			name:          "agent identified by discrete conversation flag",
			comm:          "node",
			argv:          []string{"node", "--conversation", "deadbeef123"},
			pid:           108,
			ppid:          1000,
			isOrphaned:    false,
			lastActive:    5 * time.Hour,
			wantCategory:  model.CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "regular shell not agent",
			comm:          "bash",
			argv:          []string{"bash"},
			pid:           109,
			ppid:          1,
			isOrphaned:    false,
			lastActive:    10 * time.Hour,
			wantCategory:  model.CatActiveProcess,
			wantCleanable: false,
		},
		{
			name:          "orphaned muse upgrade binary",
			comm:          "muse",
			argv:          []string{"/home/jad/.local/bin/muse-bin-1.4.14-R7309.1", "upgrade"},
			pid:           110,
			ppid:          1,
			isOrphaned:    true,
			lastActive:    5 * time.Minute,
			wantCategory:  model.CatAbandonedAgent,
			wantCleanable: true,
		},
		{
			name:          "orphaned mcp stdio server",
			comm:          "node",
			argv:          []string{"node", "/path/to/react-grab-mcp", "--stdio"},
			pid:           111,
			ppid:          1,
			isOrphaned:    true,
			lastActive:    10 * time.Minute,
			wantCategory:  model.CatAbandonedAgent,
			wantCleanable: true,
		},
		{
			name:          "active mcp server with alive parent",
			comm:          "node",
			argv:          []string{"node", "/path/to/react-grab-mcp", "--stdio"},
			pid:           112,
			ppid:          1000,
			isOrphaned:    false,
			lastActive:    10 * time.Minute,
			wantCategory:  model.CatActiveProcess,
			wantCleanable: false,
		},
		{
			name:          "orphaned opencode service",
			comm:          "opencode",
			argv:          []string{"/home/jad/.opencode/bin/opencode", "serve", "--service"},
			pid:           113,
			ppid:          1,
			isOrphaned:    true,
			lastActive:    15 * time.Minute,
			wantCategory:  model.CatAbandonedAgent,
			wantCleanable: true,
		},
		{
			name:          "orphaned commandcode node runner",
			comm:          "node",
			argv:          []string{"node", "/home/jad/.npm/command-code/bin/cmd.js"},
			pid:           114,
			ppid:          1,
			isOrphaned:    true,
			lastActive:    10 * time.Minute,
			wantCategory:  model.CatAbandonedAgent,
			wantCleanable: true,
		},
		{
			name:          "dormant cmd binary",
			comm:          "cmd",
			argv:          []string{"/home/jad/.local/bin/cmd"},
			pid:           115,
			ppid:          1000,
			isOrphaned:    false,
			lastActive:    3 * time.Hour,
			wantCategory:  model.CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "ollama serve is not an agent",
			comm:          "ollama",
			argv:          []string{"/usr/local/bin/ollama", "serve"},
			pid:           116,
			ppid:          1,
			isOrphaned:    true,
			lastActive:    10 * time.Minute,
			wantCategory:  model.CatActiveProcess,
			wantCleanable: false,
		},
		{
			name:          "generic serve is not an agent",
			comm:          "python3",
			argv:          []string{"python3", "-m", "http.server", "serve"},
			pid:           117,
			ppid:          1,
			isOrphaned:    true,
			lastActive:    3 * time.Hour,
			wantCategory:  model.CatActiveProcess,
			wantCleanable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, cleanable := CategorizeProcess(tt.comm, tt.argv, tt.pid, tt.ppid, tt.isOrphaned, tt.lastActive, idleThreshold)
			if cat != tt.wantCategory || cleanable != tt.wantCleanable {
				t.Errorf("CategorizeProcess() = (%v, %v), want (%v, %v)",
					cat, cleanable, tt.wantCategory, tt.wantCleanable)
			}
		})
	}
}

func TestAggregateShells(t *testing.T) {
	items := []model.SwapItem{
		{
			PID:        1001,
			Name:       "bash",
			SwapBytes:  5 * 1024 * 1024,
			LastActive: 10 * time.Minute,
		},
		{
			PID:        1002,
			Name:       "bash",
			SwapBytes:  8 * 1024 * 1024,
			LastActive: 2 * time.Minute,
		},
		{
			PID:        2001,
			Name:       "zsh",
			SwapBytes:  6 * 1024 * 1024,
			LastActive: 5 * time.Minute,
		},
		{
			PID:        3001,
			Name:       "node",
			SwapBytes:  50 * 1024 * 1024,
			LastActive: 1 * time.Minute,
		},
	}

	aggregated := AggregateShells(items)

	var bashItem, zshItem, nodeItem *model.SwapItem
	for i := range aggregated {
		if aggregated[i].Name == "bash (2 shells)" {
			bashItem = &aggregated[i]
		} else if aggregated[i].Name == "zsh (1 shell)" {
			zshItem = &aggregated[i]
		} else if aggregated[i].Name == "node" {
			nodeItem = &aggregated[i]
		}
	}

	if bashItem == nil {
		t.Fatalf("expected aggregated bash item 'bash (2 shells)', got none")
	}
	if bashItem.SwapBytes != 13*1024*1024 {
		t.Errorf("bash aggregated swapBytes = %d, want %d", bashItem.SwapBytes, 13*1024*1024)
	}
	if bashItem.LastActive != 2*time.Minute {
		t.Errorf("bash aggregated lastActive = %v, want 2m", bashItem.LastActive)
	}
	if bashItem.PID != 0 {
		t.Errorf("bash aggregated PID = %d, want 0", bashItem.PID)
	}

	if zshItem == nil {
		t.Fatalf("expected aggregated zsh item 'zsh (1 shell)', got none")
	}
	if zshItem.Count != 1 {
		t.Errorf("zsh count = %d, want 1", zshItem.Count)
	}

	if nodeItem == nil {
		t.Fatalf("expected untouched node item, got none")
	}
	if nodeItem.PID != 3001 {
		t.Errorf("node PID = %d, want 3001", nodeItem.PID)
	}
}
