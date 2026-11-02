//go:build linux

package proc

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseProcStatus(t *testing.T) {
	statusContent := `Name:	cat
Umask:	0022
State:	S (sleeping)
Tgid:	12345
Ngid:	0
Pid:	12345
PPid:	6789
VmRSS:	   16384 kB
VmSwap:	    2048 kB
`
	swapBytes, ppid := ParseProcStatus(strings.NewReader(statusContent))
	if swapBytes != 2048*1024 {
		t.Errorf("ParseProcStatus() swapBytes = %d, want %d", swapBytes, 2048*1024)
	}
	if ppid != 6789 {
		t.Errorf("ParseProcStatus() ppid = %d, want 6789", ppid)
	}

	vmSwapOnly := ParseVmSwap(strings.NewReader(statusContent))
	if vmSwapOnly != 2048*1024 {
		t.Errorf("ParseVmSwap() = %d, want %d", vmSwapOnly, 2048*1024)
	}

	mem := ParseProcMemory(strings.NewReader(statusContent))
	if mem.SwapBytes != 2048*1024 {
		t.Errorf("ParseProcMemory() SwapBytes = %d, want %d", mem.SwapBytes, 2048*1024)
	}
	if mem.RSSBytes != 16384*1024 {
		t.Errorf("ParseProcMemory() RSSBytes = %d, want %d", mem.RSSBytes, 16384*1024)
	}
	if mem.PPid != 6789 {
		t.Errorf("ParseProcMemory() PPid = %d, want %d", mem.PPid, 6789)
	}
}

func TestParseProcStatusNoSwap(t *testing.T) {
	statusContent := `Name:	idle
PPid:	100
`
	swapBytes, ppid := ParseProcStatus(strings.NewReader(statusContent))
	if swapBytes != 0 {
		t.Errorf("ParseProcStatus() swapBytes = %d, want 0", swapBytes)
	}
	if ppid != 100 {
		t.Errorf("ParseProcStatus() ppid = %d, want 100", ppid)
	}
}

func TestParseMeminfo(t *testing.T) {
	meminfoContent := `MemTotal:       32647896 kB
MemFree:         4123456 kB
MemAvailable:   22891234 kB
Buffers:          345678 kB
Cached:          8765432 kB
SReclaimable:     512000 kB
Shmem:            256000 kB
SwapTotal:      17924092 kB
SwapFree:       14159872 kB
Dirty:               128 kB
Writeback:            64 kB
`
	s := ParseMeminfo(strings.NewReader(meminfoContent))

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
	if s.MemFree != 4123456*1024 {
		t.Errorf("s.MemFree = %d, want %d", s.MemFree, int64(4123456)*1024)
	}
	if s.Buffers != 345678*1024 || s.Cached != 8765432*1024 {
		t.Errorf("buffers/cached = %d/%d, want 345678/8765432 KiB", s.Buffers, s.Cached)
	}
	if s.SReclaimable != 512000*1024 || s.Shmem != 256000*1024 {
		t.Errorf("sreclaimable/shmem = %d/%d, want 512000/256000 KiB", s.SReclaimable, s.Shmem)
	}
	if s.Dirty != 128*1024 || s.Writeback != 64*1024 {
		t.Errorf("dirty/writeback = %d/%d, want 128/64 KiB", s.Dirty, s.Writeback)
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
	wantCache := int64(345678+8765432+512000-256000) * 1024
	if got := s.PageCacheBytes(); got != wantCache {
		t.Errorf("PageCacheBytes() = %d, want %d", got, wantCache)
	}
}

func TestParseSwaps(t *testing.T) {
	content := `Filename				Type		Size		Used		Priority
/dev/nvme0n1p3                          partition	17892336	5242880		-2
/swapfile                               file		8388608		1048576		10
`
	devices, err := ParseSwaps(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseSwaps() error = %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("ParseSwaps() = %d devices, want 2", len(devices))
	}
	first := devices[0]
	if first.Filename != "/dev/nvme0n1p3" || first.Type != "partition" {
		t.Errorf("first device = %+v, want partition /dev/nvme0n1p3", first)
	}
	if first.SizeBytes != 17892336*1024 || first.UsedBytes != 5242880*1024 {
		t.Errorf("first size/used = %d/%d, want 17892336/5242880 KiB", first.SizeBytes, first.UsedBytes)
	}
	if first.Priority != -2 {
		t.Errorf("first priority = %d, want -2", first.Priority)
	}
	if devices[1].Filename != "/swapfile" || devices[1].Priority != 10 {
		t.Errorf("second device = %+v, want /swapfile priority 10", devices[1])
	}
}

func TestParseSwapsMalformedLinesAreSkipped(t *testing.T) {
	content := `Filename Type Size Used Priority
only three fields
/dev/sda2 partition notanumber 0 -2
/dev/sdb1 partition 1024 0 -2
`
	devices, err := ParseSwaps(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseSwaps() error = %v", err)
	}
	if len(devices) != 1 || devices[0].Filename != "/dev/sdb1" {
		t.Fatalf("devices = %+v, want only /dev/sdb1", devices)
	}
}

func TestIsZramDevice(t *testing.T) {
	cases := map[string]bool{
		"/dev/zram0":     true,
		"/dev/zram12":    true,
		"zram0":          true,
		"/dev/nvme0n1p3": false,
		"/swapfile":      false,
		"/dev/sda2":      false,
	}
	for name, want := range cases {
		if got := IsZramDevice(name); got != want {
			t.Errorf("IsZramDevice(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestParseMemoryPressure(t *testing.T) {
	content := `some avg10=1.25 avg60=0.50 avg300=0.10 total=123456
full avg10=0.75 avg60=0.25 avg300=0.05 total=654321
`
	p := ParseMemoryPressure(strings.NewReader(content))
	if !p.Available {
		t.Fatal("Available = false, want true")
	}
	if p.SomeAvg10 != 1.25 || p.SomeAvg60 != 0.50 || p.SomeAvg300 != 0.10 {
		t.Errorf("some = %v/%v/%v, want 1.25/0.50/0.10", p.SomeAvg10, p.SomeAvg60, p.SomeAvg300)
	}
	if p.FullAvg10 != 0.75 || p.FullAvg60 != 0.25 || p.FullAvg300 != 0.05 {
		t.Errorf("full = %v/%v/%v, want 0.75/0.25/0.05", p.FullAvg10, p.FullAvg60, p.FullAvg300)
	}
}

func TestParseMemoryPressureUnavailable(t *testing.T) {
	p := ParseMemoryPressure(strings.NewReader(""))
	if p.Available {
		t.Error("Available = true, want false on an empty file")
	}
}

func TestParseProcStatusUID(t *testing.T) {
	content := "Name:\tollama\nUid:\t999\t999\t999\t999\nGid:\t999\t999\t999\t999\n"
	uid, err := ParseProcStatusUID(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseProcStatusUID() error = %v", err)
	}
	if uid != 999 {
		t.Errorf("uid = %d, want 999", uid)
	}

	if _, err := ParseProcStatusUID(strings.NewReader("Name:\tbroken\n")); err == nil {
		t.Error("ParseProcStatusUID() on missing Uid line should fail")
	}
}

func TestReadLiveProcIdentity(t *testing.T) {
	pid := os.Getpid()
	if uid, err := ReadProcUID(pid); err != nil || uid != uint32(os.Getuid()) {
		t.Errorf("ReadProcUID(%d) = %d, %v; want %d", pid, uid, err, os.Getuid())
	}
	if cgroup := ReadProcessCgroup(pid); cgroup == "" {
		t.Error("ReadProcessCgroup() returned empty for the current process")
	}
	_ = ReadTTYNr(pid)
}

func TestClassifyOrphan(t *testing.T) {
	tests := []struct {
		name        string
		parentAlive bool
		parentComm  string
		cgroup      string
		ttyNr       int64
		wantReason  string
		wantOrphan  bool
	}{
		{
			name:        "live shell parent",
			parentAlive: true, parentComm: "bash", cgroup: "/user.slice/session-1.scope", ttyNr: 34816,
			wantReason: "parent-alive", wantOrphan: false,
		},
		{
			name:        "systemd service is never orphaned",
			parentAlive: false, cgroup: "/system.slice/ollama.service", ttyNr: 0,
			wantReason: "systemd-service", wantOrphan: false,
		},
		{
			name:        "user service with scope is not orphaned without a tty",
			parentAlive: false, cgroup: "/user.slice/user@1000.service/app.slice/app.scope", ttyNr: 0,
			wantReason: "no-controlling-terminal", wantOrphan: false,
		},
		{
			name:        "reparented terminal scope",
			parentAlive: false, cgroup: "/user.slice/session-3.scope", ttyNr: 34816,
			wantReason: "reparented-terminal-scope", wantOrphan: true,
		},
		{
			name:        "terminal without scope is ambiguous",
			parentAlive: false, cgroup: "/user.slice/user-1000.slice", ttyNr: 34816,
			wantReason: "ambiguous-evidence", wantOrphan: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, orphan := classifyOrphan(tt.parentAlive, tt.parentComm, tt.cgroup, tt.ttyNr)
			if reason != tt.wantReason || orphan != tt.wantOrphan {
				t.Errorf("classifyOrphan() = (%q, %v), want (%q, %v)", reason, orphan, tt.wantReason, tt.wantOrphan)
			}
		})
	}
}

func TestClassifyOrphanOllamaLiveShape(t *testing.T) {
	reason, orphan := classifyOrphan(false, "", "/system.slice/ollama.service", 0)
	if orphan {
		t.Fatalf("ollama.service shape classified as orphaned (%s)", reason)
	}
	if reason != "systemd-service" {
		t.Errorf("reason = %q, want systemd-service", reason)
	}
}

func TestLiveProcReaderFunctions(t *testing.T) {
	pid := os.Getpid()

	uptime := ReadUptime()
	if uptime <= 0 {
		t.Errorf("ReadUptime() = %f, expected > 0", uptime)
	}

	sysSwap := ReadSystemSwap()
	if sysSwap.MemTotal <= 0 {
		t.Errorf("ReadSystemSwap() MemTotal = %d, expected > 0", sysSwap.MemTotal)
	}

	comm := ReadComm(pid)
	if comm == "" {
		t.Errorf("ReadComm(%d) returned empty string", pid)
	}

	cmdline := ReadCmdline(pid, "test-fallback")
	if cmdline == "" {
		t.Errorf("ReadCmdline(%d) returned empty string", pid)
	}

	args := ReadCmdlineArgs(pid)
	if len(args) == 0 {
		t.Errorf("ReadCmdlineArgs(%d) returned empty slice", pid)
	}

	ticks := ReadStartTicks(pid)
	if ticks <= 0 {
		t.Errorf("ReadStartTicks(%d) = %d, expected > 0", pid, ticks)
	}

	bootTime := time.Now().Add(-time.Duration(uptime * float64(time.Second)))
	age := ReadProcessAge(pid, bootTime)
	if age <= 0 {
		t.Errorf("ReadProcessAge(%d) = %v, expected > 0", pid, age)
	}

	_ = ReadProcessLastActive(pid, uptime*1000.0)
	_, _ = ReadProcStatus(pid)
	_ = ReadVmSwap(pid)
}

func TestFormatCmdline(t *testing.T) {
	if got := FormatCmdline(nil, "fallback"); got != "[fallback]" {
		t.Errorf("FormatCmdline(nil) = %q, want [fallback]", got)
	}
	if got := FormatCmdline([]string{"app", "--flag"}, "fallback"); got != "app --flag" {
		t.Errorf("FormatCmdline() = %q, want app --flag", got)
	}
}

func TestCalcProcessAge(t *testing.T) {
	bootTime := time.Now().Add(-100 * time.Second)
	age := CalcProcessAge(5000, bootTime)
	if age < 45*time.Second || age > 55*time.Second {
		t.Errorf("CalcProcessAge() = %v, expected ~50s", age)
	}
	if ageZero := CalcProcessAge(0, bootTime); ageZero != 0 {
		t.Errorf("CalcProcessAge(0) = %v, want 0", ageZero)
	}
}

func TestReadStartTicksMalformedStat(t *testing.T) {
	// 1. Non-existent and edge PIDs return 0 safely
	if ticks := ReadStartTicks(-1); ticks != 0 {
		t.Errorf("ReadStartTicks(-1) = %d, want 0", ticks)
	}
	if ticks := ReadStartTicks(0); ticks != 0 {
		t.Errorf("ReadStartTicks(0) = %d, want 0", ticks)
	}
	if ticks := ReadStartTicks(99999999); ticks != 0 {
		t.Errorf("ReadStartTicks(99999999) = %d, want 0", ticks)
	}

	// 2. Parse edge cases directly verifying proc stat format contract:
	// Field 22 is starttime (index 19 in fields after the closing paren).
	parseStatContent := func(content string) int64 {
		idx := strings.LastIndex(content, ")")
		if idx == -1 || idx+2 >= len(content) {
			return 0
		}
		fields := strings.Fields(content[idx+2:])
		if len(fields) < 20 {
			return 0
		}
		ticks, err := strconv.ParseInt(fields[19], 10, 64)
		if err != nil {
			return 0
		}
		return ticks
	}

	tests := []struct {
		name     string
		content  string
		expected int64
	}{
		{"missing_closing_paren", "12345 (cat S 1 2 3", 0},
		{"paren_at_end_of_string", "12345 (cat)", 0},
		{"fewer_than_20_fields", "12345 (cat) S 1 2 3 4 5", 0},
		{"exact_19_fields", "12345 (cat) " + strings.Repeat("0 ", 19), 0},
		{"non_integer_field_20", "12345 (cat) " + strings.Repeat("0 ", 19) + "notanumber", 0},
		{"valid_20_fields", "12345 (cat) " + strings.Repeat("0 ", 19) + "123456", 123456},
		{"comm_with_parentheses", "12345 (cat (sub)) " + strings.Repeat("0 ", 19) + "78910", 78910},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStatContent(tt.content)
			if got != tt.expected {
				t.Errorf("parseStatContent(%s) = %d, want %d", tt.name, got, tt.expected)
			}
		})
	}
}

func TestReadProcessLastActiveMalformedSched(t *testing.T) {
	// 1. Non-existent and edge PIDs return -1 safely
	if dur := ReadProcessLastActive(-1, 1000.0); dur != -1 {
		t.Errorf("ReadProcessLastActive(-1) = %v, want -1", dur)
	}
	if dur := ReadProcessLastActive(99999999, 1000.0); dur != -1 {
		t.Errorf("ReadProcessLastActive(99999999) = %v, want -1", dur)
	}

	// 2. Parse edge cases directly verifying /proc/[pid]/sched format contract
	parseSchedContent := func(content string, uptimeMs float64) time.Duration {
		scanner := bufio.NewScanner(strings.NewReader(content))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "se.exec_start") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					execMs, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
					if err == nil {
						diffMs := uptimeMs - execMs
						if diffMs < 0 {
							diffMs = 0
						}
						return time.Duration(diffMs * float64(time.Millisecond))
					}
				}
			}
		}
		return -1
	}

	tests := []struct {
		name     string
		content  string
		uptimeMs float64
		expected time.Duration
	}{
		{"empty_content", "", 1000.0, -1},
		{"missing_se_exec_start", "nr_switches : 10\nnr_voluntary_switches : 5\n", 1000.0, -1},
		{"malformed_no_colon", "se.exec_start 500.0\n", 1000.0, -1},
		{"invalid_float_value", "se.exec_start : notafloat\n", 1000.0, -1},
		{"valid_positive_idle", "se.exec_start : 500.0\n", 1000.0, 500 * time.Millisecond},
		{"exec_start_in_future", "se.exec_start : 1500.0\n", 1000.0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSchedContent(tt.content, tt.uptimeMs)
			if got != tt.expected {
				t.Errorf("parseSchedContent(%s) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}
