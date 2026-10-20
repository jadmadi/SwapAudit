package triage

import (
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/proc"
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

func TestIsAgent(t *testing.T) {
	tests := []struct {
		comm string
		argv []string
		want bool
	}{
		{"agy", []string{"agy"}, true},
		{"devin", []string{"devin"}, true},
		{"node", []string{"node", "--conversation", "abc"}, true},
		{"chrome-devtools", []string{"chrome-devtools-mcp"}, true},
		{"node", []string{"node", "/usr/bin/chrome-devtools-mcp"}, true},
		{"bash", []string{"bash"}, false},
		{"node", []string{"node", "server.js"}, false},
	}
	for _, tt := range tests {
		if got := IsAgent(tt.comm, tt.argv); got != tt.want {
			t.Errorf("IsAgent(%q, %v) = %v, want %v", tt.comm, tt.argv, got, tt.want)
		}
	}
}

func TestClassifyAgentReasons(t *testing.T) {
	idle := 2 * time.Hour
	tests := []struct {
		name      string
		p         proc.ProcessSnapshot
		wantCat   model.Category
		cleanable bool
		wantWhy   string
	}{
		{
			name:    "orphaned agent",
			p:       proc.ProcessSnapshot{Comm: "agy", Orphaned: true, Orphan: proc.OrphanEvidence{Reason: "reparented-terminal-scope"}, IdleKnown: true, Idle: 10 * time.Minute},
			wantCat: model.CatAbandonedAgent, cleanable: true, wantWhy: "reparented-terminal-scope",
		},
		{
			name:    "dormant agent",
			p:       proc.ProcessSnapshot{Comm: "agy", IdleKnown: true, Idle: 3 * time.Hour},
			wantCat: model.CatDormantAgent, cleanable: true, wantWhy: "idle-threshold",
		},
		{
			name:    "idle unknown agent",
			p:       proc.ProcessSnapshot{Comm: "agy", IdleKnown: false, Idle: -1},
			wantCat: model.CatActiveProcess, cleanable: false, wantWhy: "idle-unknown",
		},
		{
			name:    "active agent",
			p:       proc.ProcessSnapshot{Comm: "agy", IdleKnown: true, Idle: 10 * time.Minute},
			wantCat: model.CatActiveProcess, cleanable: false, wantWhy: "active",
		},
		{
			name:    "desktop daemon",
			p:       proc.ProcessSnapshot{Comm: "plasmashell"},
			wantCat: model.CatDesktopDaemon, cleanable: false, wantWhy: "desktop-daemon",
		},
		{
			name:    "not an agent",
			p:       proc.ProcessSnapshot{Comm: "bash"},
			wantCat: model.CatActiveProcess, cleanable: false, wantWhy: "not-agent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, cleanable, why := ClassifyAgent(tt.p, idle)
			if cat != tt.wantCat || cleanable != tt.cleanable || why != tt.wantWhy {
				t.Errorf("ClassifyAgent() = (%v, %v, %q), want (%v, %v, %q)", cat, cleanable, why, tt.wantCat, tt.cleanable, tt.wantWhy)
			}
		})
	}
}

func TestAgentPopulation(t *testing.T) {
	result := proc.ProcessScanResult{
		Total:      10,
		Accessible: 9,
		Vanished:   1,
		Processes: []proc.ProcessSnapshot{
			{PID: 1, Comm: "agy", VmSwap: 10, IdleKnown: true, Idle: 3 * time.Hour, ParentAlive: true},
			{PID: 2, Comm: "agy", VmSwap: 0, Orphaned: true, Orphan: proc.OrphanEvidence{Reason: "reparented-terminal-scope"}, IdleKnown: true, Idle: time.Minute},
			{PID: 3, Comm: "agy", VmSwap: 0, IdleKnown: false, Idle: -1},
			{PID: 4, Comm: "bash", VmSwap: 5, IdleKnown: true, Idle: 10 * time.Minute},
		},
	}
	pop := BuildAgentPopulation(result, 2*time.Hour)
	if pop.Agents != 3 || pop.AgentsWithSwap != 1 || pop.AgentsZeroSwap != 2 {
		t.Errorf("agent counts = %+v, want 3 total, 1 with swap, 2 zero-swap", pop)
	}
	if pop.DormantAgents != 1 || pop.OrphanedAgents != 1 || pop.IdleUnknownAgents != 1 || pop.ActiveAgents != 1 {
		t.Errorf("classification counts = %+v", pop)
	}
	if pop.ParentAliveAgents != 1 {
		t.Errorf("parent alive = %d, want 1", pop.ParentAliveAgents)
	}
}

func TestNormalizeGroupKey(t *testing.T) {
	tests := map[string]string{
		"go-build3514075730":                     "go-build*",
		"TestGoalsStatsFlagConflicts11102814295": "TestGoalsStatsFlagConflicts*",
		"sila_mcp_test_426715724":                "sila_mcp_test*",
		"agy":                                    "agy",
		"chrome-devtools":                        "chrome-devtools",
		"test-foo":                               "test-foo",
	}
	for in, want := range tests {
		if got := NormalizeGroupKey(in); got != want {
			t.Errorf("NormalizeGroupKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGroupItems(t *testing.T) {
	items := []model.SwapItem{
		{Name: "go-build1234", Category: model.CatUntouchedCache, CacheKind: model.CacheBuild, AllocatedBytes: 100, SizeBytes: 100, LastActive: time.Hour, LastActiveKnown: true, Cleanable: true},
		{Name: "go-build5678", Category: model.CatUntouchedCache, CacheKind: model.CacheBuild, AllocatedBytes: 200, SizeBytes: 200, LastActive: 2 * time.Hour, LastActiveKnown: true, Cleanable: true},
		{Name: "unique", Category: model.CatUntouchedCache, CacheKind: model.CacheRun, AllocatedBytes: 50, SizeBytes: 50, LastActive: time.Minute, LastActiveKnown: true, Cleanable: true},
	}
	groups := GroupItems(items, true)
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	byKey := map[string]Group{}
	for _, g := range groups {
		byKey[g.Key] = g
	}
	g := byKey["go-build*"]
	if g.Count != 2 || g.TotalBytes != 300 {
		t.Errorf("go-build group = %+v, want count 2 total 300", g)
	}
	if g.MinIdle != time.Hour || g.MaxIdle != 2*time.Hour {
		t.Errorf("idle range = %v–%v, want 1h–2h", g.MinIdle, g.MaxIdle)
	}
	if len(ExpandGroup(g)) != 2 {
		t.Errorf("ExpandGroup returned %d members, want 2", len(ExpandGroup(g)))
	}
	singles := GroupItems(items, false)
	if len(singles) != 3 {
		t.Errorf("disabled grouping returned %d groups, want 3", len(singles))
	}
}

func TestGroupCategoryTies(t *testing.T) {
	items := []model.SwapItem{
		{Name: "go-build1234", Category: model.CatUntouchedCache, AllocatedBytes: 100, SizeBytes: 100},
		{Name: "go-build5678", Category: model.CatUntouchedCache, AllocatedBytes: 200, SizeBytes: 200},
		{Name: "TestA1234", Category: model.CatAbandonedRun, AllocatedBytes: 50, SizeBytes: 50},
		{Name: "agent", Category: model.CatDormantAgent, SwapBytes: 300},
	}
	groups := GroupItems(items, true)
	byCategory := map[model.Category]int64{}
	var total int64
	for _, g := range groups {
		byCategory[g.Category] += g.TotalBytes
		total += g.TotalBytes
	}
	want := map[model.Category]int64{model.CatUntouchedCache: 300, model.CatAbandonedRun: 50, model.CatDormantAgent: 300}
	for cat, v := range want {
		if byCategory[cat] != v {
			t.Errorf("category %s group sum = %d, want %d", cat, byCategory[cat], v)
		}
	}
	if total != 650 {
		t.Errorf("total group bytes = %d, want 650", total)
	}
}
