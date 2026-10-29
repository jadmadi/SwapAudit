//go:build linux

package term

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
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
		got := FormatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
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
		got := FormatDuration(tt.input)
		if got != tt.expected {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.input, got, tt.expected)
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
		{"\x1b[31mhello world\x1b[0m", 6, "\x1b[31mhello…\x1b[0m"},
		{"\x1b[38;5;208morange text here\x1b[0m", 10, "\x1b[38;5;208morange te…\x1b[0m"},
		{"\x1b[38;2;255;128;0mcustom color line\x1b[0m", 12, "\x1b[38;2;255;128;0mcustom colo…\x1b[0m"},
	}

	for _, tt := range tests {
		got := Truncate(tt.input, tt.width)
		if got != tt.expected {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.expected)
		}
		if w := StrWidth(got); w > tt.width {
			t.Errorf("StrWidth(Truncate(%q, %d)) = %d, exceeds width %d", tt.input, tt.width, w, tt.width)
		}
	}
}

func TestTruncateANSI(t *testing.T) {
	// Verify that truncated ANSI colored strings always terminate with reset
	input := "\x1b[1;33mDormant Session (Active)\x1b[0m"
	got := Truncate(input, 15)
	if !strings.HasSuffix(got, "\x1b[0m") {
		t.Errorf("Truncate(%q, 15) = %q, expected suffix \\x1b[0m", input, got)
	}
	if w := StrWidth(got); w > 15 {
		t.Errorf("StrWidth(%q) = %d, exceeds limit 15", got, w)
	}

	// Verify plain text does not introduce unnecessary reset codes
	plain := "plain text"
	gotPlain := Truncate(plain, 5)
	if strings.Contains(gotPlain, "\x1b") {
		t.Errorf("Truncate(%q, 5) = %q, should not contain ANSI escape codes", plain, gotPlain)
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
		got := StrWidth(tt.input)
		if got != tt.expected {
			t.Errorf("StrWidth(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestRenderTableWidths(t *testing.T) {
	cols := []model.Col{
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
		{"\x1b[33m▸ 999999999999999\x1b[0m", "\x1b[36mvery-long-target-name-that-definitely-exceeds-the-thirty-character-column-limit\x1b[0m", "dormant-agents", "10.0 MB", "1d", "1d"},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	RenderTable(cols, rows)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	lines := strings.Split(strings.Trim(output, "\r\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("RenderTable emitted fewer than 3 lines: %q", output)
	}

	expectedWidth := StrWidth(lines[0])
	for i, line := range lines {
		w := StrWidth(line)
		if w != expectedWidth {
			t.Errorf("line %d (%q) has visible width %d, want %d", i, line, w, expectedWidth)
		}
	}
}

func TestRenderMeterBar(t *testing.T) {
	SetColorEnabled(false)
	SetAsciiMode(true)
	defer func() {
		SetColorEnabled(true)
		SetAsciiMode(false)
	}()

	b0 := RenderMeterBar(0.0, 10)
	if b0 != "[..........]" {
		t.Errorf("RenderMeterBar(0) = %q, want [..........]", b0)
	}

	b50 := RenderMeterBar(50.0, 10)
	if b50 != "[#####.....]" {
		t.Errorf("RenderMeterBar(50) = %q, want [#####.....]", b50)
	}

	b100 := RenderMeterBar(100.0, 10)
	if b100 != "[##########]" {
		t.Errorf("RenderMeterBar(100) = %q, want [##########]", b100)
	}

	share := RenderShareMiniBar(25.5, 10)
	if !strings.Contains(share, "25.5%") {
		t.Errorf("RenderShareMiniBar = %q, missing 25.5%%", share)
	}
}

func TestColorsAndGlyphs(t *testing.T) {
	SetColorEnabled(true)
	if !strings.Contains(Bold("test"), "\x1b[1m") {
		t.Errorf("Bold missing ANSI code")
	}
	if !strings.Contains(Dim("test"), "\x1b[2m") {
		t.Errorf("Dim missing ANSI code")
	}
	if !strings.Contains(Red("test"), "\x1b[31m") {
		t.Errorf("Red missing ANSI code")
	}
	if !strings.Contains(Green("test"), "\x1b[32m") {
		t.Errorf("Green missing ANSI code")
	}
	if !strings.Contains(Yellow("test"), "\x1b[33m") {
		t.Errorf("Yellow missing ANSI code")
	}
	if !strings.Contains(Cyan("test"), "\x1b[36m") {
		t.Errorf("Cyan missing ANSI code")
	}

	SetColorEnabled(false)
	if Bold("test") != "test" {
		t.Errorf("Bold with color disabled should return raw string")
	}

	SetAsciiMode(false)
	if GlyphBoxTopLeft() != "╭" || GlyphCheck() != "✓" {
		t.Errorf("UTF-8 glyphs incorrect")
	}

	SetAsciiMode(true)
	if GlyphBoxTopLeft() != "+" || GlyphCheck() != "OK" {
		t.Errorf("ASCII glyphs incorrect")
	}
}

func TestPrintBoxHeader(t *testing.T) {
	SetTermWidth(80)
	SetAsciiMode(true)
	SetColorEnabled(false)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	PrintBoxHeader("TEST HEADER")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	lines := strings.Split(strings.Trim(output, "\r\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("PrintBoxHeader should print 3 lines, got %d", len(lines))
	}
	for i, l := range lines {
		if StrWidth(l) != 80 {
			t.Errorf("line %d visible width = %d, want 80", i, StrWidth(l))
		}
	}
}

func TestPrintUpdateNotice(t *testing.T) {
	SetTermWidth(80)
	SetAsciiMode(true)
	SetColorEnabled(false)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stdout = w

	PrintUpdateNotice("v0.1.0", "v0.2.0")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "A new release of swapaudit is available") {
		t.Errorf("expected update notice message, got %q", output)
	}
	if !strings.Contains(output, "swapaudit upgrade") {
		t.Errorf("expected upgrade instruction, got %q", output)
	}
}
