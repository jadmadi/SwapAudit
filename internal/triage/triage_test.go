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
			lastActive:    0,
			wantCategory:  model.CatDesktopDaemon,
			wantCleanable: false,
		},
		{
			name:          "active agent session",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           102,
			ppid:          1,
			lastActive:    15 * time.Minute,
			wantCategory:  model.CatActiveProcess,
			wantCleanable: false,
		},
		{
			name:          "dormant agent session exceeding threshold",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           103,
			ppid:          1,
			lastActive:    3 * time.Hour,
			wantCategory:  model.CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "agent session with unknown last active",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           104,
			ppid:          1,
			lastActive:    -1,
			wantCategory:  model.CatActiveProcess,
			wantCleanable: false,
		},
		{
			name:          "dormant chrome devtools mcp",
			comm:          "chrome-devtools",
			argv:          []string{"chrome-devtools-mcp"},
			pid:           105,
			ppid:          1,
			lastActive:    4 * time.Hour,
			wantCategory:  model.CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "agent identified by discrete conversation flag",
			comm:          "node",
			argv:          []string{"node", "--conversation", "deadbeef123"},
			pid:           106,
			ppid:          1,
			lastActive:    5 * time.Hour,
			wantCategory:  model.CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "regular shell not agent",
			comm:          "bash",
			argv:          []string{"bash"},
			pid:           107,
			ppid:          1,
			lastActive:    10 * time.Hour,
			wantCategory:  model.CatActiveProcess,
			wantCleanable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, cleanable := CategorizeProcess(tt.comm, tt.argv, tt.pid, tt.ppid, tt.lastActive, idleThreshold)
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
