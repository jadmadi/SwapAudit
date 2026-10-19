//go:build linux

package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1048576 * 3 / 2, "1.5 MB"},
		{1073741824, "1.0 GB"},
		{1073741824 * 3, "3.0 GB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{-1, "—"},
		{-5 * time.Second, "—"},
		{0, "0s"},
		{30 * time.Second, "30s"},
		{45 * time.Second, "45s"},
		{59 * time.Second, "59s"},
		{1 * time.Minute, "1m"},
		{12 * time.Minute, "12m"},
		{45 * time.Minute, "45m"},
		{1 * time.Hour, "1h"},
		{2*time.Hour + 14*time.Minute, "2h14m"},
		{3*time.Hour + 30*time.Minute, "3h30m"},
		{24 * time.Hour, "1d"},
		{3*24*time.Hour + 4*time.Hour, "3d4h"},
		{2*24*time.Hour + 5*time.Minute, "2d"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.input)
		if got != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 6, "hello…"},
		{"chrome-devtools-mcp", 15, "chrome-devtool…"},
		{"short", 1, "…"},
		{"short", 0, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.width)
		if got != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.expected)
		}
		if w := strWidth(got); w > tt.width {
			t.Errorf("strWidth(truncate(%q, %d)) = %d, exceeds width %d", tt.input, tt.width, w, tt.width)
		}
	}
}

func TestStrWidth(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 5},
		{"abc 123", 7},
		{"\x1b[31mhello\x1b[0m", 5},
		{"\x1b[1;32mTest\x1b[0m", 4},
		{"\x1b[33m◆\x1b[0m", 1},
		{"\x1b[33m▸\x1b[0m", 1},
		{"🔄", 2},
		{"📊", 2},
		{"📈", 2},
		{"⚡", 2},
	}

	for _, tt := range tests {
		got := strWidth(tt.input)
		if got != tt.expected {
			t.Errorf("strWidth(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestRenderTableWidths(t *testing.T) {
	cols := []Col{
		{Title: "PID", Width: 9},
		{Title: "Target / Command", Width: 30},
		{Title: "Category", Width: 16},
		{Title: "Swap Size", Width: 10, Right: true},
		{Title: "Age", Width: 8, Right: true},
		{Title: "Idle", Width: 8, Right: true},
	}

	rows := [][]string{
		{"▸ 47627", "chrome-devtools", "dormant-agents", "38.9 MB", "2d7h", "2d7h"},
		{"  2699690", "chrome-devtools", "dormant-agents", "36.7 MB", "1d4h", "1d4h"},
		{"  -", "bash (45 shells)", "active-apps", "296.8 MB", "2d7h", "2s"},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	renderTable(cols, rows)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	lines := strings.Split(strings.Trim(output, "\r\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("renderTable emitted fewer than 3 lines: %q", output)
	}

	expectedWidth := strWidth(lines[0])
	for i, line := range lines {
		w := strWidth(line)
		if w != expectedWidth {
			t.Errorf("line %d (%q) has visible width %d, want %d", i, line, w, expectedWidth)
		}
	}
}

func TestCategorizeProcess(t *testing.T) {
	idleThreshold := 2 * time.Hour

	tests := []struct {
		name          string
		comm          string
		argv          []string
		pid           int
		ppid          int
		lastActive    time.Duration
		wantCategory  Category
		wantCleanable bool
	}{
		{
			name:          "desktop daemon plasmashell",
			comm:          "plasmashell",
			argv:          []string{"plasmashell"},
			pid:           100,
			ppid:          1,
			lastActive:    10 * time.Minute,
			wantCategory:  CatDesktopDaemon,
			wantCleanable: false,
		},
		{
			name:          "desktop daemon kwin_wayland",
			comm:          "kwin_wayland",
			argv:          []string{"/usr/bin/kwin_wayland"},
			pid:           101,
			ppid:          1,
			lastActive:    0,
			wantCategory:  CatDesktopDaemon,
			wantCleanable: false,
		},
		{
			name:          "active agent session",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           102,
			ppid:          1,
			lastActive:    15 * time.Minute,
			wantCategory:  CatActiveProcess,
			wantCleanable: false,
		},
		{
			name:          "dormant agent session exceeding threshold",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           103,
			ppid:          1,
			lastActive:    3 * time.Hour,
			wantCategory:  CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "agent session with unknown last active",
			comm:          "agy",
			argv:          []string{"agy"},
			pid:           104,
			ppid:          1,
			lastActive:    -1,
			wantCategory:  CatActiveProcess,
			wantCleanable: false,
		},
		{
			name:          "dormant chrome devtools mcp",
			comm:          "chrome-devtools",
			argv:          []string{"chrome-devtools-mcp"},
			pid:           105,
			ppid:          1,
			lastActive:    4 * time.Hour,
			wantCategory:  CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "agent identified by discrete conversation flag",
			comm:          "node",
			argv:          []string{"node", "--conversation", "deadbeef123"},
			pid:           106,
			ppid:          1,
			lastActive:    5 * time.Hour,
			wantCategory:  CatDormantAgent,
			wantCleanable: true,
		},
		{
			name:          "regular shell not agent",
			comm:          "bash",
			argv:          []string{"bash"},
			pid:           107,
			ppid:          1,
			lastActive:    10 * time.Hour,
			wantCategory:  CatActiveProcess,
			wantCleanable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, cleanable := categorizeProcess(tt.comm, tt.argv, tt.pid, tt.ppid, tt.lastActive, idleThreshold)
			if cat != tt.wantCategory || cleanable != tt.wantCleanable {
				t.Errorf("categorizeProcess() = (%v, %v), want (%v, %v)",
					cat, cleanable, tt.wantCategory, tt.wantCleanable)
			}
		})
	}
}

func TestParseProcStatus(t *testing.T) {
	statusContent := `Name:	cat
Umask:	0022
State:	S (sleeping)
Tgid:	12345
Ngid:	0
Pid:	12345
PPid:	6789
VmSwap:	    2048 kB
`
	swapBytes, ppid := parseProcStatus(strings.NewReader(statusContent))
	if swapBytes != 2048*1024 {
		t.Errorf("parseProcStatus() swapBytes = %d, want %d", swapBytes, 2048*1024)
	}
	if ppid != 6789 {
		t.Errorf("parseProcStatus() ppid = %d, want %d", ppid, 6789)
	}

	vmSwapOnly := parseVmSwap(strings.NewReader(statusContent))
	if vmSwapOnly != 2048*1024 {
		t.Errorf("parseVmSwap() = %d, want %d", vmSwapOnly, 2048*1024)
	}
}

func TestParseProcStatusNoSwap(t *testing.T) {
	statusContent := `Name:	idle
PPid:	100
`
	swapBytes, ppid := parseProcStatus(strings.NewReader(statusContent))
	if swapBytes != 0 {
		t.Errorf("parseProcStatus() swapBytes = %d, want 0", swapBytes)
	}
	if ppid != 100 {
		t.Errorf("parseProcStatus() ppid = %d, want 100", ppid)
	}
}

func TestParseMeminfo(t *testing.T) {
	meminfoContent := `MemTotal:       32647896 kB
MemFree:         4123456 kB
MemAvailable:   22891234 kB
Buffers:          345678 kB
Cached:          8765432 kB
SwapTotal:      17924092 kB
SwapFree:       14159872 kB
Dirty:               128 kB
`
	s := parseMeminfo(strings.NewReader(meminfoContent))

	expectedMemTotal := int64(32647896) * 1024
	expectedMemAvail := int64(22891234) * 1024
	expectedSwapTotal := int64(17924092) * 1024
	expectedSwapFree := int64(14159872) * 1024
	expectedSwapUsed := expectedSwapTotal - expectedSwapFree
	expectedMemUsed := expectedMemTotal - expectedMemAvail

	if s.MemTotal != expectedMemTotal {
		t.Errorf("s.MemTotal = %d, want %d", s.MemTotal, expectedMemTotal)
	}
	if s.MemAvail != expectedMemAvail {
		t.Errorf("s.MemAvail = %d, want %d", s.MemAvail, expectedMemAvail)
	}
	if s.MemUsed != expectedMemUsed {
		t.Errorf("s.MemUsed = %d, want %d", s.MemUsed, expectedMemUsed)
	}
	if s.SwapTotal != expectedSwapTotal {
		t.Errorf("s.SwapTotal = %d, want %d", s.SwapTotal, expectedSwapTotal)
	}
	if s.SwapFree != expectedSwapFree {
		t.Errorf("s.SwapFree = %d, want %d", s.SwapFree, expectedSwapFree)
	}
	if s.SwapUsed != expectedSwapUsed {
		t.Errorf("s.SwapUsed = %d, want %d", s.SwapUsed, expectedSwapUsed)
	}
}

func TestMatchTmpPattern(t *testing.T) {
	patterns := []string{
		"cs1*", "sila*", "manara*", "test-bwrap*", "tsx-*", "custom-run",
	}

	tests := []struct {
		name     string
		expected bool
	}{
		{"sila-cli-test-profile", true},
		{"manara-backend", true},
		{"test-bwrap-instance", true},
		{"tsx-1234", true},
		{"custom-run-567", true},
		{"unrelated-file", false},
		{"chrome-cache", false},
	}

	for _, tt := range tests {
		matched := false
		for _, pat := range patterns {
			if matchTmpPattern(pat, tt.name) {
				matched = true
				break
			}
		}
		if matched != tt.expected {
			t.Errorf("matchTmpPattern against %q = %v, want %v", tt.name, matched, tt.expected)
		}
	}
}
