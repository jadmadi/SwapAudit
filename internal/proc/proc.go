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

// ReadCmdline returns a reconstructed command line string from /proc/[pid]/cmdline.
func ReadCmdline(pid int, fallback string) string {
	argv := ReadCmdlineArgs(pid)
	if len(argv) == 0 {
		return fmt.Sprintf("[%s]", fallback)
	}
	return strings.Join(argv, " ")
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

// ReadProcessAge calculates the elapsed age of a process from boot time and /proc/[pid]/stat starttime.
// Note: clock ticks are assumed to be 100 Hz (USER_HZ on standard Linux).
func ReadProcessAge(pid int, bootTime time.Time) time.Duration {
	startTicks := ReadStartTicks(pid)
	if startTicks <= 0 {
		return 0
	}
	startSec := float64(startTicks) / 100.0
	startTime := bootTime.Add(time.Duration(startSec * float64(time.Second)))
	return time.Since(startTime)
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
