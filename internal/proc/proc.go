//go:build linux

package proc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

// ReadUptime reads system uptime in seconds from /proc/uptime.
func ReadUptime() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		val, _ := strconv.ParseFloat(fields[0], 64)
		return val
	}
	return 0
}

// ParseMeminfo parses swap and memory statistics from an io.Reader representing /proc/meminfo.
func ParseMeminfo(r io.Reader) model.SystemSwap {
	var s model.SystemSwap
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(fields[1], 10, 64)
		bytes := val * 1024
		switch fields[0] {
		case "SwapTotal:":
			s.SwapTotal = bytes
		case "SwapFree:":
			s.SwapFree = bytes
		case "MemTotal:":
			s.MemTotal = bytes
		case "MemFree:":
			s.MemFree = bytes
		case "MemAvailable:":
			s.MemAvail = bytes
		case "Buffers:":
			s.Buffers = bytes
		case "Cached:":
			s.Cached = bytes
		case "SReclaimable:":
			s.SReclaimable = bytes
		case "Shmem:":
			s.Shmem = bytes
		case "Dirty:":
			s.Dirty = bytes
		case "Writeback:":
			s.Writeback = bytes
		}
	}
	s.SwapUsed = s.SwapTotal - s.SwapFree
	s.MemUsed = s.MemTotal - s.MemAvail
	return s
}

// SwapDevice is one active swap area from /proc/swaps.
type SwapDevice struct {
	Filename  string
	Type      string
	SizeBytes int64
	UsedBytes int64
	Priority  int
}

// ParseSwaps parses /proc/swaps. Sizes are reported in KiB by the kernel and
// are converted to bytes.
func ParseSwaps(r io.Reader) ([]SwapDevice, error) {
	var devices []SwapDevice
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Filename") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		sizeKib, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			continue
		}
		usedKib, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			continue
		}
		priority, err := strconv.Atoi(fields[4])
		if err != nil {
			continue
		}
		devices = append(devices, SwapDevice{
			Filename:  fields[0],
			Type:      fields[1],
			SizeBytes: sizeKib * 1024,
			UsedBytes: usedKib * 1024,
			Priority:  priority,
		})
	}
	if err := scanner.Err(); err != nil {
		return devices, err
	}
	return devices, nil
}

// ReadSwapDevices reads the active swap areas from /proc/swaps.
func ReadSwapDevices() ([]SwapDevice, error) {
	f, err := os.Open("/proc/swaps")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseSwaps(f)
}

// IsZramDevice reports whether a swap area is a zram block device. zram stores
// compressed pages in RAM, so draining it decompresses pages into RAM.
func IsZramDevice(filename string) bool {
	return strings.HasPrefix(filepath.Base(filename), "zram")
}

// MemoryPressure is the /proc/pressure/memory snapshot. Available is false on
// kernels without CONFIG_PSI.
type MemoryPressure struct {
	SomeAvg10  float64
	SomeAvg60  float64
	SomeAvg300 float64
	FullAvg10  float64
	FullAvg60  float64
	FullAvg300 float64
	Available  bool
}

// ParseMemoryPressure parses the two lines in /proc/pressure/memory.
func ParseMemoryPressure(r io.Reader) MemoryPressure {
	var p MemoryPressure
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		kind := fields[0]
		if kind != "some" && kind != "full" {
			continue
		}
		var a10, a60, a300 float64
		for _, f := range fields[1:] {
			parts := strings.SplitN(f, "=", 2)
			if len(parts) != 2 {
				continue
			}
			v, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				continue
			}
			switch parts[0] {
			case "avg10":
				a10 = v
			case "avg60":
				a60 = v
			case "avg300":
				a300 = v
			}
		}
		if kind == "some" {
			p.SomeAvg10, p.SomeAvg60, p.SomeAvg300 = a10, a60, a300
			p.Available = true
		} else {
			p.FullAvg10, p.FullAvg60, p.FullAvg300 = a10, a60, a300
			p.Available = true
		}
	}
	return p
}

// ReadMemoryPressure reads /proc/pressure/memory.
func ReadMemoryPressure() MemoryPressure {
	f, err := os.Open("/proc/pressure/memory")
	if err != nil {
		return MemoryPressure{}
	}
	defer f.Close()
	return ParseMemoryPressure(f)
}

// ReadSystemSwap queries /proc/meminfo and /tmp tmpfs statfs.
func ReadSystemSwap() model.SystemSwap {
	var s model.SystemSwap

	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		s = ParseMeminfo(f)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs("/tmp", &stat); err == nil {
		s.TmpfsSize = int64(stat.Blocks) * int64(stat.Bsize)
		free := int64(stat.Bfree) * int64(stat.Bsize)
		s.TmpfsUsed = s.TmpfsSize - free
	}

	return s
}

// Memory captures swap, resident physical memory, and parent PID from /proc/[pid]/status.
type Memory struct {
	SwapBytes int64
	RSSBytes  int64
	PPid      int
}

// ParseProcMemory extracts VmSwap, VmRSS (in bytes), and PPid from a /proc/[pid]/status reader.
func ParseProcMemory(r io.Reader) Memory {
	var m Memory
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmSwap:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				m.SwapBytes = val * 1024
			}
		} else if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				m.RSSBytes = val * 1024
			}
		} else if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				m.PPid, _ = strconv.Atoi(fields[1])
			}
		}
	}
	return m
}

// ParseProcStatus extracts VmSwap (in bytes) and PPid from a /proc/[pid]/status reader.
func ParseProcStatus(r io.Reader) (int64, int) {
	m := ParseProcMemory(r)
	return m.SwapBytes, m.PPid
}

// ParseVmSwap parses VmSwap bytes from an io.Reader representing /proc/[pid]/status.
func ParseVmSwap(r io.Reader) int64 {
	m := ParseProcMemory(r)
	return m.SwapBytes
}

// ReadProcMemory reads VmSwap, VmRSS, and PPid for a given PID.
func ReadProcMemory(pid int) Memory {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return Memory{}
	}
	defer f.Close()
	return ParseProcMemory(f)
}

// ReadProcStatus reads VmSwap bytes and parent PID for a given PID.
func ReadProcStatus(pid int) (int64, int) {
	m := ReadProcMemory(pid)
	return m.SwapBytes, m.PPid
}

// ReadVmSwap returns the VmSwap bytes for a given PID.
func ReadVmSwap(pid int) int64 {
	swapBytes, _ := ReadProcStatus(pid)
	return swapBytes
}

// ReadComm returns the process command name from /proc/[pid]/comm.
func ReadComm(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return fmt.Sprintf("pid-%d", pid)
	}
	return strings.TrimSpace(string(data))
}

// ReadCmdlineArgs returns parsed argv tokens from /proc/[pid]/cmdline.
func ReadCmdlineArgs(pid int) []string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || len(data) == 0 {
		return nil
	}
	raw := strings.Split(string(data), "\x00")
	var argv []string
	for _, a := range raw {
		if a != "" {
			argv = append(argv, a)
		}
	}
	return argv
}

// FormatCmdline formats argv tokens or falls back to bracketed comm name.
func FormatCmdline(argv []string, fallback string) string {
	if len(argv) == 0 {
		return fmt.Sprintf("[%s]", fallback)
	}
	return strings.Join(argv, " ")
}

// ReadCmdline returns a reconstructed command line string from /proc/[pid]/cmdline.
func ReadCmdline(pid int, fallback string) string {
	return FormatCmdline(ReadCmdlineArgs(pid), fallback)
}

// ReadStartTicks returns field 22 (starttime) from /proc/[pid]/stat in clock ticks.
func ReadStartTicks(pid int) int64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	content := string(data)
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

// CalcProcessAge calculates elapsed age from start ticks and boot time.
func CalcProcessAge(startTicks int64, bootTime time.Time) time.Duration {
	if startTicks <= 0 {
		return 0
	}
	startSec := float64(startTicks) / 100.0
	startTime := bootTime.Add(time.Duration(startSec * float64(time.Second)))
	return time.Since(startTime)
}

// ReadProcessAge calculates the elapsed age of a process from boot time and /proc/[pid]/stat starttime.
// Note: clock ticks are assumed to be 100 Hz (USER_HZ on standard Linux).
func ReadProcessAge(pid int, bootTime time.Time) time.Duration {
	return CalcProcessAge(ReadStartTicks(pid), bootTime)
}

// ParseProcStatusUID returns the real UID from the Uid: line in /proc/[pid]/status.
func ParseProcStatusUID(r io.Reader) (uint32, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("malformed Uid line")
		}
		val, err := strconv.ParseUint(fields[1], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("parse Uid: %w", err)
		}
		return uint32(val), nil
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("Uid line not found")
}

// ReadProcUID returns the real UID of pid.
func ReadProcUID(pid int) (uint32, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return ParseProcStatusUID(f)
}

// ReadTTYNr returns field 7 (tty_nr) from /proc/[pid]/stat, or 0 if unavailable.
func ReadTTYNr(pid int) int64 {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	content := string(data)
	idx := strings.LastIndex(content, ")")
	if idx == -1 || idx+2 >= len(content) {
		return 0
	}
	fields := strings.Fields(content[idx+2:])
	if len(fields) < 5 {
		return 0
	}
	tty, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return 0
	}
	return tty
}

// ReadProcessCgroup returns the unified cgroup path from /proc/[pid]/cgroup.
func ReadProcessCgroup(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return ""
	}
	var fallback string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		if parts[0] == "0" && parts[1] == "" {
			return parts[2]
		}
		if fallback == "" {
			fallback = parts[2]
		}
	}
	return fallback
}

// OrphanEvidence captures the evidence used to classify a reparented process.
type OrphanEvidence struct {
	PPID        int
	ParentAlive bool
	ParentComm  string
	Cgroup      string
	TTYNr       int64
	Reason      string
}

// classifyOrphan is deliberately conservative. A process is orphaned only when
// it looks like a terminal-launched process whose parent is gone. systemd
// services, processes without a controlling terminal, and ambiguous evidence
// are never orphaned.
func classifyOrphan(parentAlive bool, parentComm, cgroup string, ttyNr int64) (string, bool) {
	if parentAlive && parentComm != "systemd" && parentComm != "init" {
		return "parent-alive", false
	}
	if strings.Contains(cgroup, ".service") && !strings.Contains(cgroup, ".scope") {
		return "systemd-service", false
	}
	if ttyNr == 0 {
		return "no-controlling-terminal", false
	}
	if strings.Contains(cgroup, ".scope") || strings.Contains(cgroup, "session-") {
		return "reparented-terminal-scope", true
	}
	return "ambiguous-evidence", false
}

// ClassifyOrphan returns true only when the process is a plausible
// terminal-launched process whose parent shell died. Ambiguous evidence
// always returns false.
func ClassifyOrphan(pid, ppid int) (OrphanEvidence, bool) {
	ev := OrphanEvidence{
		PPID:   ppid,
		Cgroup: ReadProcessCgroup(pid),
		TTYNr:  ReadTTYNr(pid),
	}
	if ppid > 1 {
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", ppid)); err == nil {
			ev.ParentAlive = true
			ev.ParentComm = ReadComm(ppid)
		}
	}
	reason, orphaned := classifyOrphan(ev.ParentAlive, ev.ParentComm, ev.Cgroup, ev.TTYNr)
	ev.Reason = reason
	return ev, orphaned
}

// ReadProcessLastActive parses se.exec_start from /proc/[pid]/sched to determine process idle time.
// Returns -1 if schedule information is unavailable.
func ReadProcessLastActive(pid int, uptimeMs float64) time.Duration {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/sched", pid))
	if err != nil {
		return -1
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
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
