//go:build linux

package proc

import (
	"bufio"
	"fmt"
	"io"
	"os"
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
		case "MemAvailable:":
			s.MemAvail = bytes
		}
	}
	s.SwapUsed = s.SwapTotal - s.SwapFree
	s.MemUsed = s.MemTotal - s.MemAvail
	return s
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

// ParseProcStatus extracts VmSwap (in bytes) and PPid from a /proc/[pid]/status reader.
func ParseProcStatus(r io.Reader) (int64, int) {
	var swapBytes int64
	var ppid int

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmSwap:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, _ := strconv.ParseInt(fields[1], 10, 64)
				swapBytes = val * 1024
			}
		} else if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ppid, _ = strconv.Atoi(fields[1])
			}
		}
	}
	return swapBytes, ppid
}

// ParseVmSwap parses VmSwap bytes from an io.Reader representing /proc/[pid]/status.
func ParseVmSwap(r io.Reader) int64 {
	swapBytes, _ := ParseProcStatus(r)
	return swapBytes
}

// ReadProcStatus reads VmSwap bytes and parent PID for a given PID.
func ReadProcStatus(pid int) (int64, int) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	return ParseProcStatus(f)
}

// ReadVmSwap returns the VmSwap bytes for a given PID.
func ReadVmSwap(pid int) int64 {
	swapBytes, _ := ReadProcStatus(pid)
	return swapBytes
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
	// Fields after comm start at field 3 (state); tty_nr is field 7.
	if len(fields) < 5 {
		return 0
	}
	tty, err := strconv.ParseInt(fields[4], 10, 64)
	if err != nil {
		return 0
	}
	return tty
}

// ReadProcessCgroup returns the unified cgroup path from /proc/[pid]/cgroup, or "" if unavailable.
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
	PPID       int
	ParentComm string
	Cgroup     string
	TTYNr      int64
	Reason     string
}

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
// terminal-launched agent whose parent shell has died. Ambiguous evidence
// always returns false.
func ClassifyOrphan(pid, ppid int) (OrphanEvidence, bool) {
	ev := OrphanEvidence{
		PPID:   ppid,
		Cgroup: ReadProcessCgroup(pid),
		TTYNr:  ReadTTYNr(pid),
	}

	parentAlive := false
	if ppid > 1 {
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", ppid)); err == nil {
			parentAlive = true
			ev.ParentComm = ReadComm(ppid)
		}
	}

	reason, orphaned := classifyOrphan(parentAlive, ev.ParentComm, ev.Cgroup, ev.TTYNr)
	ev.Reason = reason
	return ev, orphaned
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
