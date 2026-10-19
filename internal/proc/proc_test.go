//go:build linux

package proc

import (
	"strings"
	"testing"
)

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
	swapBytes, ppid := ParseProcStatus(strings.NewReader(statusContent))
	if swapBytes != 2048*1024 {
		t.Errorf("ParseProcStatus() swapBytes = %d, want %d", swapBytes, 2048*1024)
	}
	if ppid != 6789 {
		t.Errorf("ParseProcStatus() ppid = %d, want %d", ppid, 6789)
	}

	vmSwapOnly := ParseVmSwap(strings.NewReader(statusContent))
	if vmSwapOnly != 2048*1024 {
		t.Errorf("ParseVmSwap() = %d, want %d", vmSwapOnly, 2048*1024)
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
SwapTotal:      17924092 kB
SwapFree:       14159872 kB
Dirty:               128 kB
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
