//go:build linux

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

// Category represents the classification of a swap-consuming entity.
type Category string

const (
	CatUntouchedCache Category = "untouched-caches"
	CatAbandonedRun   Category = "abandoned-runs"
	CatDormantAgent   Category = "dormant-agents"
	CatAbandonedAgent Category = "abandoned-agents"
	CatActiveCache    Category = "active-caches"
	CatDesktopDaemon  Category = "desktop-daemons"
	CatActiveProcess  Category = "active-apps"
)

// SwapItem represents an individual process or filesystem cache occupying swap.
type SwapItem struct {
	PID          int
	StartTicks   int64
	Name         string
	Command      string
	Category     Category
	SwapBytes    int64
	SizeBytes    int64
	Age          time.Duration
	LastActive   time.Duration
	LastActiveAt time.Time
	Path         string
	Cleanable    bool
	Count        int // for aggregated rows
}

// SystemSwap represents overall system memory and swap status.
type SystemSwap struct {
	SwapTotal int64
	SwapUsed  int64
	SwapFree  int64
	MemTotal  int64
	MemUsed   int64
	MemAvail  int64
	TmpfsUsed int64
	TmpfsSize int64
}

// defaultTmpPatterns lists common prefixes/wildcards for test and temporary run directories in /tmp.
var defaultTmpPatterns = []string{
	"cs1*", "cs2*", "thermodynamic*", "Test*", "test*",
	"sila*", "manara*", "muhaffiz*", "commandcode*", "orthemology*",
	"test-bwrap*", "tsx-*", "rustup-update*",
}

// stringSliceFlag implements flag.Value to allow repeatable string flags.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(val string) error {
	*s = append(*s, val)
	return nil
}

// Col defines a column in renderTable.
type Col struct {
	Title string
	Width int
	Right bool // right-align
}

var (
	termWidth    = 100
	colorEnabled = true
	asciiMode    = false
)

func getTerminalWidth() int {
	var ws struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)))
	if err == 0 && ws.Col > 0 {
		w := int(ws.Col)
		if w < 80 {
			return 80
		}
		if w > 120 {
			return 120
		}
		return w
	}
	return 100
}

func initTerminal(noColorFlag, asciiFlag bool) {
	termWidth = getTerminalWidth()

	if asciiFlag {
		asciiMode = true
	} else {
		lang := os.Getenv("LANG")
		lcAll := os.Getenv("LC_ALL")
		if !strings.Contains(strings.ToUpper(lang), "UTF-8") && !strings.Contains(strings.ToUpper(lcAll), "UTF-8") {
			asciiMode = true
		}
	}

	if noColorFlag {
		colorEnabled = false
		return
	}
	if os.Getenv("NO_COLOR") != "" {
		colorEnabled = false
		return
	}
	if os.Getenv("TERM") == "dumb" {
		colorEnabled = false
		return
	}
	fi, err := os.Stdout.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		colorEnabled = false
		return
	}
	colorEnabled = true
}

func bold(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[1m" + s + "\x1b[0m"
}

func dim(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}

func red(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[31m" + s + "\x1b[0m"
}

func green(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[32m" + s + "\x1b[0m"
}

func yellow(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[33m" + s + "\x1b[0m"
}

func cyan(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[36m" + s + "\x1b[0m"
}

func glyphBoxTopLeft() string {
	if asciiMode {
		return "+"
	}
	return "╭"
}

func glyphBoxTopRight() string {
	if asciiMode {
		return "+"
	}
	return "╮"
}

func glyphBoxBottomLeft() string {
	if asciiMode {
		return "+"
	}
	return "╰"
}

func glyphBoxBottomRight() string {
	if asciiMode {
		return "+"
	}
	return "╯"
}

func glyphBoxHoriz() string {
	if asciiMode {
		return "-"
	}
	return "─"
}

func glyphBoxVert() string {
	if asciiMode {
		return "|"
	}
	return "│"
}

func glyphBarFilled() string {
	if asciiMode {
		return "#"
	}
	return "█"
}

func glyphBarEmpty() string {
	if asciiMode {
		return "."
	}
	return "░"
}

func glyphAccent() string {
	if asciiMode {
		return "*"
	}
	return "◆"
}

func glyphActiveIndicator() string {
	if asciiMode {
		return ">"
	}
	return "▸"
}

func glyphCheck() string {
	if asciiMode {
		return "OK"
	}
	return "✓"
}

func glyphCross() string {
	if asciiMode {
		return "FAIL"
	}
	return "✗"
}

func glyphDash() string {
	if asciiMode {
		return "-"
	}
	return "—"
}

func glyphEllipsis() string {
	if asciiMode {
		return "..."
	}
	return "…"
}

func glyphWarn() string {
	if asciiMode {
		return "!"
	}
	return "⚠"
}

func strWidth(s string) int {
	w := 0
	inEsc := false
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			inEsc = true
			i++
			continue
		}
		if inEsc {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEsc = false
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		if r == 0xFE0F {
			continue
		}
		if r >= 0x1F000 || (r >= 0x2600 && r <= 0x27BF && r != 0x2794) || (r >= 0x2B50 && r <= 0x2B55) {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

func truncate(s string, w int) string {
	if strWidth(s) <= w {
		return s
	}
	ell := glyphEllipsis()
	ellW := strWidth(ell)
	if w <= ellW {
		if w > 0 {
			r := []rune(ell)
			if w <= len(r) {
				return string(r[:w])
			}
			return ell
		}
		return ""
	}
	maxW := w - ellW
	var result []rune
	curW := 0
	for _, r := range s {
		rw := 1
		if r >= 0x1F000 || (r >= 0x2600 && r <= 0x27BF && r != 0x2794) || (r >= 0x2B50 && r <= 0x2B55) {
			rw = 2
		}
		if curW+rw > maxW {
			break
		}
		result = append(result, r)
		curW += rw
	}
	return string(result) + ell
}

func renderTable(cols []Col, rows [][]string) {
	if len(cols) == 0 {
		return
	}

	var headerParts []string
	for _, c := range cols {
		title := c.Title
		tw := strWidth(title)
		if tw > c.Width {
			title = truncate(title, c.Width)
			tw = strWidth(title)
		}
		pad := c.Width - tw
		if pad < 0 {
			pad = 0
		}
		if c.Right {
			headerParts = append(headerParts, strings.Repeat(" ", pad)+title)
		} else {
			headerParts = append(headerParts, title+strings.Repeat(" ", pad))
		}
	}
	fmt.Printf("  %s\n", strings.Join(headerParts, " | "))

	var sepParts []string
	for _, c := range cols {
		sepParts = append(sepParts, strings.Repeat("-", c.Width))
	}
	fmt.Printf("  %s\n", strings.Join(sepParts, "-+-"))

	totalWidth := 0
	for i, c := range cols {
		totalWidth += c.Width
		if i > 0 {
			totalWidth += 3
		}
	}

	for _, r := range rows {
		isSectionHeader := len(r) > 1 && r[0] != ""
		if isSectionHeader {
			for _, cell := range r[1:] {
				if cell != "" {
					isSectionHeader = false
					break
				}
			}
		}
		if isSectionHeader {
			vw := strWidth(r[0])
			pad := totalWidth - vw
			if pad < 0 {
				pad = 0
			}
			fmt.Printf("  %s%s\n", r[0], strings.Repeat(" ", pad))
			continue
		}

		var rowParts []string
		for i, c := range cols {
			val := ""
			if i < len(r) {
				val = r[i]
			}
			vw := strWidth(val)
			if vw > c.Width {
				val = truncate(val, c.Width)
				vw = strWidth(val)
			}
			pad := c.Width - vw
			if pad < 0 {
				pad = 0
			}
			if c.Right {
				rowParts = append(rowParts, strings.Repeat(" ", pad)+val)
			} else {
				rowParts = append(rowParts, val+strings.Repeat(" ", pad))
			}
		}
		fmt.Printf("  %s\n", strings.Join(rowParts, " | "))
	}
}

func printBoxHeader(title string) {
	w := termWidth
	accent := glyphAccent()
	content := fmt.Sprintf("%s  %s", accent, title)
	contentVisW := strWidth(content)

	innerWidth := w - 4
	if contentVisW > innerWidth {
		content = truncate(content, innerWidth)
		contentVisW = strWidth(content)
	}
	pad := innerWidth - contentVisW
	if pad < 0 {
		pad = 0
	}

	tl := glyphBoxTopLeft()
	tr := glyphBoxTopRight()
	bl := glyphBoxBottomLeft()
	br := glyphBoxBottomRight()
	horiz := glyphBoxHoriz()
	vert := glyphBoxVert()

	topLine := tl + strings.Repeat(horiz, w-2) + tr
	botLine := bl + strings.Repeat(horiz, w-2) + br
	boldContent := bold(content)
	midLine := fmt.Sprintf("%s %s%s %s", vert, boldContent, strings.Repeat(" ", pad), vert)

	fmt.Println(topLine)
	fmt.Println(midLine)
	fmt.Println(botLine)
}

func main() {
	var (
		sortBy         = flag.String("sort", "size", "Sort column: 'size' or 'age'")
		cleanUntouched = flag.Bool("clean-untouched", false, "Clean untouched /tmp build and compile caches")
		cleanAbandoned = flag.Bool("clean-abandoned", false, "Clean abandoned /tmp test and review runs")
		cleanDormant   = flag.Bool("clean-dormant", false, "Stop dormant agent sessions")
		cleanOrphaned  = flag.Bool("clean-orphaned", false, "End orphaned agent sessions (parent shell deceased)")
		cleanTmp       = flag.Bool("clean-tmp", false, "Clean all stale /tmp caches (untouched + abandoned)")
		cleanAll       = flag.Bool("clean-all", false, "Clean all safe cleanable groups (untouched + abandoned + dormant + orphaned)")
		dryRun         = flag.Bool("dry-run", false, "Simulate cleanup actions without deleting or stopping")
		yes            = flag.Bool("yes", false, "Automatically answer yes to confirmation prompts")
		maxAge         = flag.Duration("max-age", 12*time.Hour, "Cutoff age for stale /tmp folders")
		idleThreshold  = flag.Duration("idle-threshold", 2*time.Hour, "Idle time threshold for dormant agents")
		limit          = flag.Int("limit", 12, "Maximum rows to show in the main occupants table")
		showAll        = flag.Bool("all", false, "Show all individual processes without aggregation")
		noColor        = flag.Bool("no-color", false, "Disable colored output")
		ascii          = flag.Bool("ascii", false, "Use plain ASCII characters instead of Unicode box/bar glyphs")
	)
	var customTmpPatterns stringSliceFlag
	flag.Var(&customTmpPatterns, "tmp-pattern", "Additional /tmp pattern to identify test/temporary artifacts (repeatable)")
	flag.BoolVar(yes, "y", false, "Alias for --yes")
	flag.Parse()

	initTerminal(*noColor, *ascii)

	if *sortBy != "size" && *sortBy != "age" {
		fmt.Fprintf(os.Stderr, "error: invalid --sort value %q (must be 'size' or 'age')\n", *sortBy)
		os.Exit(2)
	}

	now := time.Now()
	uptimeSec := readUptime()
	sysSwap := readSystemSwap()

	tmpPatterns := append(append([]string{}, defaultTmpPatterns...), customTmpPatterns...)
	items := collectSwapItems(now, uptimeSec, *maxAge, *idleThreshold, tmpPatterns)

	printHeader(sysSwap)
	printActionableSection(items, *showAll)
	printMainTable(items, *sortBy, *limit, *showAll)
	printCategorySummary(items)

	// Consolidate cleanup actions
	if *cleanAll {
		*cleanUntouched = true
		*cleanAbandoned = true
		*cleanDormant = true
		*cleanOrphaned = true
	}
	if *cleanTmp {
		*cleanUntouched = true
		*cleanAbandoned = true
	}

	if *cleanUntouched || *cleanAbandoned || *cleanDormant || *cleanOrphaned {
		executeCleanup(items, *cleanUntouched, *cleanAbandoned, *cleanDormant, *cleanOrphaned, *dryRun, *yes)
	} else {
		printQuickCommands()
	}
}

func readUptime() float64 {
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

// parseMeminfo parses swap and memory statistics from /proc/meminfo.
func parseMeminfo(r io.Reader) SystemSwap {
	var s SystemSwap
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

func readSystemSwap() SystemSwap {
	var s SystemSwap

	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		s = parseMeminfo(f)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs("/tmp", &stat); err == nil {
		s.TmpfsSize = int64(stat.Blocks) * int64(stat.Bsize)
		free := int64(stat.Bfree) * int64(stat.Bsize)
		s.TmpfsUsed = s.TmpfsSize - free
	}

	return s
}

func collectSwapItems(now time.Time, uptimeSec float64, maxAge, idleThreshold time.Duration, tmpPatterns []string) []SwapItem {
	var items []SwapItem
	uptimeMs := uptimeSec * 1000.0
	bootTime := now.Add(-time.Duration(uptimeSec * float64(time.Second)))

	items = append(items, scanTmpDirectories(now, maxAge, tmpPatterns)...)

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return items
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}

		swapBytes, ppid := readProcStatus(pid)
		if swapBytes <= 0 {
			continue
		}

		comm := readComm(pid)
		argv := readCmdlineArgs(pid)
		cmd := readCmdline(pid, comm)
		age := readProcessAge(pid, bootTime)
		lastActive := readProcessLastActive(pid, uptimeMs)
		lastActiveAt := now.Add(-lastActive)

		cat, cleanable := categorizeProcess(comm, argv, pid, ppid, lastActive, idleThreshold)

		dispName := comm
		hasConv := false
		for i, arg := range argv {
			if arg == "--conversation" && i+1 < len(argv) {
				targetId := argv[i+1]
				if len(targetId) > 8 {
					targetId = targetId[:8]
				}
				dispName = fmt.Sprintf("agy [%s]", targetId)
				hasConv = true
				break
			}
		}
		if !hasConv {
			for _, arg := range argv {
				if arg == "chrome-devtools-mcp" || filepath.Base(arg) == "chrome-devtools-mcp" {
					dispName = "chrome-devtools"
					break
				}
			}
		}

		startTicks := readStartTicks(pid)
		items = append(items, SwapItem{
			PID:          pid,
			StartTicks:   startTicks,
			Name:         dispName,
			Command:      cmd,
			Category:     cat,
			SwapBytes:    swapBytes,
			Age:          age,
			LastActive:   lastActive,
			LastActiveAt: lastActiveAt,
			Cleanable:    cleanable,
			Count:        1,
		})
	}

	return items
}

func matchTmpPattern(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	if err == nil && matched {
		return true
	}
	cleanPat := strings.TrimSuffix(pattern, "*")
	if cleanPat != "" && (strings.HasPrefix(name, cleanPat) || strings.EqualFold(name, cleanPat)) {
		return true
	}
	return false
}

func scanTmpDirectories(now time.Time, maxAge time.Duration, patterns []string) []SwapItem {
	var items []SwapItem
	currentUid := uint32(os.Getuid())

	entries, err := os.ReadDir("/tmp")
	if err != nil {
		return items
	}

	knownDaemonPrefixes := []string{
		"tmux-", "codex-daemon-", "MegaLimited", "plasma-",
		"wayland-", "dbus-", "ssh-", "gnupg-", "atuin-",
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "systemd-") || strings.HasPrefix(name, "pulse-") {
			continue
		}
		isDaemon := false
		for _, p := range knownDaemonPrefixes {
			if strings.HasPrefix(name, p) {
				isDaemon = true
				break
			}
		}
		if isDaemon {
			continue
		}

		fullPath := filepath.Join("/tmp", name)
		fi, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			continue
		}

		stat_t, ok := fi.Sys().(*syscall.Stat_t)
		if !ok || stat_t.Uid != currentUid {
			continue
		}

		if fi.Mode()&os.ModeSocket != 0 || fi.Mode()&os.ModeNamedPipe != 0 {
			continue
		}

		lower := strings.ToLower(name)
		isBuildCache := strings.HasPrefix(name, "go-build") || strings.Contains(lower, "cache")
		isTestRun := false
		for _, pat := range patterns {
			if matchTmpPattern(pat, name) {
				isTestRun = true
				break
			}
		}

		if !isBuildCache && !isTestRun {
			keywords := []string{"test", "build", "review", "candidate", "probe", "profile", "repro"}
			matched := false
			for _, k := range keywords {
				if strings.Contains(lower, k) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			isTestRun = true
		}

		var size int64
		newestMtime := fi.ModTime()

		if entry.IsDir() {
			filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr == nil && d != nil {
					if info, err := d.Info(); err == nil {
						if !d.IsDir() {
							size += info.Size()
						}
						if info.ModTime().After(newestMtime) {
							newestMtime = info.ModTime()
						}
					}
				}
				return nil
			})
		} else {
			size = fi.Size()
		}

		age := now.Sub(fi.ModTime())
		lastActive := now.Sub(newestMtime)

		cleanable := lastActive > maxAge
		var cat Category
		if cleanable {
			if isBuildCache {
				cat = CatUntouchedCache
			} else {
				cat = CatAbandonedRun
			}
		} else {
			cat = CatActiveCache
		}

		items = append(items, SwapItem{
			Name:         name,
			Command:      fullPath,
			Path:         fullPath,
			Category:     cat,
			SwapBytes:    0,
			SizeBytes:    size,
			Age:          age,
			LastActive:   lastActive,
			LastActiveAt: newestMtime,
			Cleanable:    cleanable,
			Count:        1,
		})
	}

	return items
}

// parseProcStatus extracts VmSwap (in bytes) and PPid from a /proc/pid/status reader.
func parseProcStatus(r io.Reader) (int64, int) {
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

// parseVmSwap parses VmSwap bytes from an io.Reader representing /proc/pid/status.
func parseVmSwap(r io.Reader) int64 {
	swapBytes, _ := parseProcStatus(r)
	return swapBytes
}

func readProcStatus(pid int) (int64, int) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	return parseProcStatus(f)
}

func readVmSwap(pid int) int64 {
	swapBytes, _ := readProcStatus(pid)
	return swapBytes
}

func readComm(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return fmt.Sprintf("pid-%d", pid)
	}
	return strings.TrimSpace(string(data))
}

func readCmdlineArgs(pid int) []string {
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

func readCmdline(pid int, fallback string) string {
	argv := readCmdlineArgs(pid)
	if len(argv) == 0 {
		return fmt.Sprintf("[%s]", fallback)
	}
	return strings.Join(argv, " ")
}

// readStartTicks returns field 22 (starttime) from /proc/pid/stat in clock ticks.
func readStartTicks(pid int) int64 {
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

// readProcessAge calculates the age of a process from /proc/uptime and /proc/pid/stat starttime.
// Note: clock ticks are assumed to be 100 Hz (USER_HZ on standard Linux).
func readProcessAge(pid int, bootTime time.Time) time.Duration {
	startTicks := readStartTicks(pid)
	if startTicks <= 0 {
		return 0
	}
	// Note: clock ticks are assumed to be 100 Hz (USER_HZ on standard Linux).
	startSec := float64(startTicks) / 100.0
	startTime := bootTime.Add(time.Duration(startSec * float64(time.Second)))
	return time.Since(startTime)
}

func readProcessLastActive(pid int, uptimeMs float64) time.Duration {
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

func categorizeProcess(comm string, argv []string, pid, ppid int, lastActive, idleThreshold time.Duration) (Category, bool) {
	var binName string
	if len(argv) > 0 {
		binName = filepath.Base(argv[0])
	}

	desktopNames := []string{
		"plasmashell", "kwin", "kwin_wayland", "xorg", "xwayland", "kded6",
		"ksmserver", "kwalletd6", "kaccess", "polkit", "kdeconnectd",
		"xdg-desktop-portal", "systemd", "dbus-daemon", "pipewire",
	}
	for _, d := range desktopNames {
		if strings.EqualFold(comm, d) || strings.EqualFold(binName, d) {
			return CatDesktopDaemon, false
		}
	}

	isAgent := comm == "agy" || comm == "devin" || comm == "chrome-devtools-mcp" || comm == "chrome-devtools" ||
		binName == "agy" || binName == "devin" || binName == "antigravity" || binName == "chrome-devtools-mcp"

	if !isAgent {
		for _, arg := range argv {
			if arg == "--conversation" || arg == "chrome-devtools-mcp" || filepath.Base(arg) == "chrome-devtools-mcp" {
				isAgent = true
				break
			}
		}
	}

	if isAgent {
		if ppid > 1 {
			if _, err := os.Stat(fmt.Sprintf("/proc/%d", ppid)); os.IsNotExist(err) {
				return CatAbandonedAgent, true
			}
		}
		if lastActive >= 0 && lastActive > idleThreshold {
			return CatDormantAgent, true
		}
		return CatActiveProcess, false
	}

	return CatActiveProcess, false
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return glyphDash()
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		if mins > 0 {
			return fmt.Sprintf("%dh%dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dd", days)
}

func renderMeterBar(percent float64, width int) string {
	filled := int((percent / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled
	rawBar := "[" + strings.Repeat(glyphBarFilled(), filled) + strings.Repeat(glyphBarEmpty(), empty) + "]"
	if percent < 50.0 {
		return green(rawBar)
	} else if percent < 80.0 {
		return yellow(rawBar)
	}
	return red(rawBar)
}

func renderShareMiniBar(pct float64, width int) string {
	return fmt.Sprintf("%s %5.1f%%", renderMeterBar(pct, width), pct)
}

func printHeader(s SystemSwap) {
	printBoxHeader("SWAP & IN-MEMORY CACHE AUDITOR")

	barWidth := 16

	// 1. Swap
	if s.SwapTotal <= 0 {
		fmt.Printf("  %-5s  Swap not configured\n", "Swap")
	} else {
		swapPct := (float64(s.SwapUsed) / float64(s.SwapTotal)) * 100.0
		bar := renderMeterBar(swapPct, barWidth)
		fmt.Printf("  %-5s %s %5.1f%% %8s / %8s (free %s)\n",
			"Swap", bar, swapPct, formatBytes(s.SwapUsed), formatBytes(s.SwapTotal), formatBytes(s.SwapFree))
	}

	// 2. RAM
	ramPct := 0.0
	if s.MemTotal > 0 {
		ramPct = (float64(s.MemUsed) / float64(s.MemTotal)) * 100.0
	}
	ramBar := renderMeterBar(ramPct, barWidth)
	fmt.Printf("  %-5s %s %5.1f%% %8s / %8s (avail %s)\n",
		"RAM", ramBar, ramPct, formatBytes(s.MemUsed), formatBytes(s.MemTotal), formatBytes(s.MemAvail))

	// 3. /tmp
	tmpPct := 0.0
	if s.TmpfsSize > 0 {
		tmpPct = (float64(s.TmpfsUsed) / float64(s.TmpfsSize)) * 100.0
	}
	tmpBar := renderMeterBar(tmpPct, barWidth)
	tmpFree := s.TmpfsSize - s.TmpfsUsed
	fmt.Printf("  %-5s %s %5.1f%% %8s / %8s (free %s)\n\n",
		"/tmp", tmpBar, tmpPct, formatBytes(s.TmpfsUsed), formatBytes(s.TmpfsSize), formatBytes(tmpFree))
}

func itemSize(it SwapItem) int64 {
	if it.SizeBytes > 0 {
		return it.SizeBytes
	}
	return it.SwapBytes
}

func printActionableSection(items []SwapItem, showAll bool) {
	var cleanables []SwapItem
	var totalReclaimTmp int64
	var totalReclaimSwap int64

	for _, it := range items {
		if it.Cleanable {
			cleanables = append(cleanables, it)
			totalReclaimTmp += it.SizeBytes
			totalReclaimSwap += it.SwapBytes
		}
	}

	if len(cleanables) == 0 {
		fmt.Printf("  %s\n\n", dim(glyphCheck()+" nothing to reclaim"))
		return
	}

	sort.SliceStable(cleanables, func(i, j int) bool {
		si := itemSize(cleanables[i])
		sj := itemSize(cleanables[j])
		if si != sj {
			return si > sj
		}
		if cleanables[i].PID != cleanables[j].PID {
			return cleanables[i].PID < cleanables[j].PID
		}
		return cleanables[i].Name < cleanables[j].Name
	})

	boxTitle := fmt.Sprintf("Actionable Reclaim Candidates (Cleanable: %s)", formatBytes(totalReclaimTmp+totalReclaimSwap))
	printBoxHeader(boxTitle)

	var cols []Col
	var targetWidth int
	if termWidth >= 100 {
		targetWidth = termWidth - 82
		if targetWidth < 12 {
			targetWidth = 12
		}
		cols = []Col{
			{Title: "PID", Width: 9},
			{Title: "Target / Name", Width: targetWidth},
			{Title: "Category", Width: 16},
			{Title: "Size", Width: 10, Right: true},
			{Title: "Idle", Width: 8, Right: true},
			{Title: "Action", Width: 22},
		}
	} else {
		targetWidth = termWidth - 63
		if targetWidth < 12 {
			targetWidth = 12
		}
		cols = []Col{
			{Title: "PID", Width: 9},
			{Title: "Target / Name", Width: targetWidth},
			{Title: "Size", Width: 10, Right: true},
			{Title: "Idle", Width: 8, Right: true},
			{Title: "Action", Width: 22},
		}
	}

	limit := 12
	shown := 0
	var hiddenBytes int64
	var rows [][]string

	for _, it := range cleanables {
		shown++
		if !showAll && shown > limit {
			hiddenBytes += itemSize(it)
			continue
		}

		pidStr := "-"
		if it.PID > 0 {
			pidStr = strconv.Itoa(it.PID)
		}
		colPID := yellow(glyphActiveIndicator() + " " + pidStr)

		name := truncate(it.Name, targetWidth)

		action := "Prune stale cache"
		switch it.Category {
		case CatUntouchedCache:
			action = "Prune untouched cache"
		case CatAbandonedRun:
			action = "Prune abandoned run"
		case CatAbandonedAgent:
			action = "End orphaned process"
		case CatDormantAgent:
			action = "End dormant session"
		}

		if termWidth >= 100 {
			rows = append(rows, []string{
				colPID,
				name,
				string(it.Category),
				formatBytes(itemSize(it)),
				formatDuration(it.LastActive),
				action,
			})
		} else {
			rows = append(rows, []string{
				colPID,
				name,
				formatBytes(itemSize(it)),
				formatDuration(it.LastActive),
				action,
			})
		}
	}

	renderTable(cols, rows)

	if !showAll && len(cleanables) > limit {
		fmt.Printf("  ... and %d smaller candidates (%s). Run with --all to inspect full list.\n",
			len(cleanables)-limit, formatBytes(hiddenBytes))
	}
	fmt.Println()
}

func printMainTable(items []SwapItem, sortBy string, limit int, showAll bool) {
	displayItems := items

	if !showAll {
		var filtered []SwapItem
		grouped := make(map[string]*SwapItem)

		for _, it := range items {
			if it.PID > 0 && it.SwapBytes < 20*1024*1024 && (it.Name == "bash" || it.Name == "sh" || it.Name == "zsh") {
				key := it.Name
				if g, ok := grouped[key]; ok {
					g.SwapBytes += it.SwapBytes
					g.Count++
					if it.LastActive >= 0 {
						if g.LastActive < 0 || it.LastActive < g.LastActive {
							g.LastActive = it.LastActive
						}
					}
				} else {
					cp := it
					cp.Count = 1
					grouped[key] = &cp
				}
			} else {
				filtered = append(filtered, it)
			}
		}

		for _, g := range grouped {
			if g.Count == 1 {
				g.Name = fmt.Sprintf("%s (1 shell)", g.Name)
			} else {
				g.Name = fmt.Sprintf("%s (%d shells)", g.Name, g.Count)
			}
			g.PID = 0
			filtered = append(filtered, *g)
		}
		displayItems = filtered
	}

	if sortBy == "age" {
		sort.SliceStable(displayItems, func(i, j int) bool {
			if displayItems[i].Age != displayItems[j].Age {
				return displayItems[i].Age > displayItems[j].Age
			}
			if displayItems[i].PID != displayItems[j].PID {
				return displayItems[i].PID < displayItems[j].PID
			}
			return displayItems[i].Name < displayItems[j].Name
		})
	} else {
		sort.SliceStable(displayItems, func(i, j int) bool {
			si := itemSize(displayItems[i])
			sj := itemSize(displayItems[j])
			if si != sj {
				return si > sj
			}
			if displayItems[i].PID != displayItems[j].PID {
				return displayItems[i].PID < displayItems[j].PID
			}
			return displayItems[i].Name < displayItems[j].Name
		})
	}

	boxTitle := fmt.Sprintf("Top Memory & Cache Occupants (Ranked by %s)", strings.ToUpper(sortBy))
	printBoxHeader(boxTitle)

	var cols []Col
	var targetWidth int
	if termWidth >= 100 {
		targetWidth = termWidth - 68
		if targetWidth < 12 {
			targetWidth = 12
		}
		cols = []Col{
			{Title: "PID", Width: 9},
			{Title: "Target / Command", Width: targetWidth},
			{Title: "Category", Width: 16},
			{Title: "Size", Width: 10, Right: true},
			{Title: "Age", Width: 8, Right: true},
			{Title: "Idle", Width: 8, Right: true},
		}
	} else {
		targetWidth = termWidth - 38
		if targetWidth < 12 {
			targetWidth = 12
		}
		fourthCol := Col{Title: "Idle", Width: 8, Right: true}
		if sortBy == "age" {
			fourthCol = Col{Title: "Age", Width: 8, Right: true}
		}
		cols = []Col{
			{Title: "PID", Width: 9},
			{Title: "Target / Command", Width: targetWidth},
			{Title: "Size", Width: 10, Right: true},
			fourthCol,
		}
	}

	shown := 0
	hidden := 0
	var rows [][]string

	for _, it := range displayItems {
		sz := itemSize(it)
		if sz < 10*1024*1024 && !it.Cleanable {
			hidden++
			continue
		}
		if limit > 0 && shown >= limit {
			hidden++
			continue
		}
		shown++

		pidStr := "-"
		if it.PID > 0 {
			pidStr = strconv.Itoa(it.PID)
		}

		var colPID string
		if it.Cleanable {
			colPID = yellow(glyphActiveIndicator() + " " + pidStr)
		} else if it.Category == CatDesktopDaemon {
			colPID = dim("  " + pidStr)
		} else {
			colPID = "  " + pidStr
		}

		name := truncate(it.Name, targetWidth)

		if termWidth >= 100 {
			rows = append(rows, []string{
				colPID,
				name,
				string(it.Category),
				formatBytes(sz),
				formatDuration(it.Age),
				formatDuration(it.LastActive),
			})
		} else {
			fourthVal := formatDuration(it.LastActive)
			if sortBy == "age" {
				fourthVal = formatDuration(it.Age)
			}
			rows = append(rows, []string{
				colPID,
				name,
				formatBytes(sz),
				fourthVal,
			})
		}
	}

	renderTable(cols, rows)

	if hidden > 0 {
		fmt.Printf("  ... and %d more hidden. Run with --all to inspect full list.\n", hidden)
	}
	fmt.Println()
}

func printCategorySummary(items []SwapItem) {
	tmpSums := make(map[Category]int64)
	swapSums := make(map[Category]int64)
	cleanableTmpSums := make(map[Category]int64)
	cleanableSwapSums := make(map[Category]int64)
	counts := make(map[Category]int)

	var totalProcessSwap int64
	var totalTmpSize int64

	for _, it := range items {
		tmpSums[it.Category] += it.SizeBytes
		swapSums[it.Category] += it.SwapBytes
		counts[it.Category] += it.Count
		totalProcessSwap += it.SwapBytes
		totalTmpSize += it.SizeBytes
		if it.Cleanable {
			cleanableTmpSums[it.Category] += it.SizeBytes
			cleanableSwapSums[it.Category] += it.SwapBytes
		}
	}

	printBoxHeader("Category Breakdown & Actionable Totals")

	catWidth := termWidth - 61
	if catWidth < 14 {
		catWidth = 14
	}

	cols := []Col{
		{Title: "Category", Width: catWidth},
		{Title: "Entities", Width: 8, Right: true},
		{Title: "Size", Width: 10, Right: true},
		{Title: "Share", Width: 19},
		{Title: "Actionable", Width: 10, Right: true},
	}

	var rows [][]string

	// Group 1: Process Swap
	swapCats := []Category{
		CatDormantAgent,
		CatAbandonedAgent,
		CatDesktopDaemon,
		CatActiveProcess,
	}

	hasSwapRows := false
	for _, c := range swapCats {
		if counts[c] > 0 {
			hasSwapRows = true
			break
		}
	}

	if hasSwapRows {
		rows = append(rows, []string{
			bold("Process Swap (RAM)"), "", "", "", "",
		})
		for _, c := range swapCats {
			if counts[c] == 0 {
				continue
			}
			swapBytes := swapSums[c]
			sharePct := 0.0
			if totalProcessSwap > 0 {
				sharePct = (float64(swapBytes) / float64(totalProcessSwap)) * 100.0
			}
			shareBar := renderShareMiniBar(sharePct, 10)
			actStr := formatBytes(cleanableSwapSums[c])

			rows = append(rows, []string{
				"  " + string(c),
				strconv.Itoa(counts[c]),
				formatBytes(swapBytes),
				shareBar,
				actStr,
			})
		}
	}

	// Group 2: /tmp Caches
	tmpCats := []Category{
		CatUntouchedCache,
		CatAbandonedRun,
		CatActiveCache,
	}

	hasTmpRows := false
	for _, c := range tmpCats {
		if counts[c] > 0 {
			hasTmpRows = true
			break
		}
	}

	if hasTmpRows {
		rows = append(rows, []string{
			bold("/tmp Caches (tmpfs)"), "", "", "", "",
		})
		for _, c := range tmpCats {
			if counts[c] == 0 {
				continue
			}
			tmpBytes := tmpSums[c]
			sharePct := 0.0
			if totalTmpSize > 0 {
				sharePct = (float64(tmpBytes) / float64(totalTmpSize)) * 100.0
			}
			shareBar := renderShareMiniBar(sharePct, 10)
			actStr := formatBytes(cleanableTmpSums[c])

			rows = append(rows, []string{
				"  " + string(c),
				strconv.Itoa(counts[c]),
				formatBytes(tmpBytes),
				shareBar,
				actStr,
			})
		}
	}

	renderTable(cols, rows)
	fmt.Println()
}

func printQuickCommands() {
	printBoxHeader("Quick Maintenance Commands")
	cmd := func(flagName, comment string) {
		cyanFlag := cyan(flagName)
		flagVisW := strWidth(flagName)
		pad := 20 - flagVisW
		if pad < 1 {
			pad = 1
		}
		fmt.Printf("  swapaudit %s%s # %s\n", cyanFlag, strings.Repeat(" ", pad), comment)
	}

	cmd("--clean-untouched", "Prune stale build caches (> 12h untouched)")
	cmd("--clean-abandoned", "Prune abandoned test suites & review runs (> 12h idle)")
	cmd("--clean-dormant",   "End dormant agent sessions (> 2h idle)")
	cmd("--clean-orphaned",  "End orphaned agent sessions whose parent shell died")
	cmd("--clean-tmp",       "Clean all stale /tmp caches (untouched + abandoned)")
	cmd("--clean-all",       "Run all cleanup actions (untouched + abandoned + dormant + orphaned)")
	cmd("--dry-run",         "Preview cleanup actions without touching files or processes")
	fmt.Println()
}

func pruneTmpTargets(targets []SwapItem, groupTitle, prompt string, dryRun, yes bool, pathWidth int, prefix string) int64 {
	if len(targets) == 0 {
		fmt.Printf("Group: %s - %s\n", groupTitle, dim("Nothing cleanable found."))
		return 0
	}

	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].SizeBytes != targets[j].SizeBytes {
			return targets[i].SizeBytes > targets[j].SizeBytes
		}
		return targets[i].Path < targets[j].Path
	})

	var groupBytes int64
	for _, t := range targets {
		groupBytes += t.SizeBytes
	}
	fmt.Printf("Group: %s (%d targets, %s)\n", groupTitle, len(targets), formatBytes(groupBytes))
	confirm := yes
	if !confirm && !dryRun {
		confirm = askConfirmation(prompt)
		if !confirm {
			fmt.Printf("  %s\n", dim("Skipped."))
			return 0
		}
	}

	shown := 0
	limit := 10
	var hiddenBytes int64
	var reclaimed int64
	for _, t := range targets {
		if !dryRun {
			if err := os.RemoveAll(t.Path); err != nil {
				fmt.Printf("  %s%s failed to prune %s: %v\n", prefix, red(glyphCross()), t.Path, err)
				continue
			}
		}
		if shown < limit || t.SizeBytes >= 10*1024*1024 {
			tPath := truncate(t.Path, pathWidth)
			pad := pathWidth - strWidth(tPath)
			if pad < 0 {
				pad = 0
			}
			fmt.Printf("  %s%s %-6s %s%s %10s\n",
				prefix, green(glyphCheck()), "pruned", tPath, strings.Repeat(" ", pad), formatBytes(t.SizeBytes))
			shown++
		} else {
			hiddenBytes += t.SizeBytes
		}
		reclaimed += t.SizeBytes
	}
	if len(targets) > shown {
		fmt.Printf("  %s... and %d smaller targets (%s)\n", prefix, len(targets)-shown, formatBytes(hiddenBytes))
	}
	return reclaimed
}

func killProcessTargets(targets []SwapItem, groupTitle, prompt string, dryRun, yes bool, pathWidth int, prefix string) int64 {
	if len(targets) == 0 {
		fmt.Printf("Group: %s - %s\n", groupTitle, dim("Nothing cleanable found."))
		return 0
	}

	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].SwapBytes != targets[j].SwapBytes {
			return targets[i].SwapBytes > targets[j].SwapBytes
		}
		return targets[i].PID < targets[j].PID
	})

	var groupBytes int64
	for _, t := range targets {
		groupBytes += t.SwapBytes
	}
	fmt.Printf("Group: %s (%d processes, %s)\n", groupTitle, len(targets), formatBytes(groupBytes))
	confirm := yes
	if !confirm && !dryRun {
		confirm = askConfirmation(prompt)
		if !confirm {
			fmt.Printf("  %s\n", dim("Skipped."))
			return 0
		}
	}

	var reclaimed int64
	for _, p := range targets {
		if !dryRun {
			currTicks := readStartTicks(p.PID)
			if currTicks == 0 {
				fmt.Printf("  %s%s Skipped PID %d (%s): process no longer exists\n", prefix, yellow(glyphWarn()), p.PID, p.Name)
				continue
			}
			if currTicks != p.StartTicks {
				fmt.Printf("  %s%s Skipped PID %d (%s): PID was reused by another process\n", prefix, yellow(glyphWarn()), p.PID, p.Name)
				continue
			}
			if err := syscall.Kill(p.PID, syscall.SIGTERM); err != nil {
				fmt.Printf("  %s%s failed to kill PID %d (%s): %v\n", prefix, red(glyphCross()), p.PID, p.Name, err)
				continue
			}
		}
		pDesc := fmt.Sprintf("PID %d: %s", p.PID, p.Name)
		tDesc := truncate(pDesc, pathWidth)
		pad := pathWidth - strWidth(tDesc)
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("  %s%s %-6s %s%s %10s\n",
			prefix, green(glyphCheck()), "ended", tDesc, strings.Repeat(" ", pad), formatBytes(p.SwapBytes))
		reclaimed += p.SwapBytes
	}
	return reclaimed
}

func executeCleanup(items []SwapItem, cleanUntouched, cleanAbandoned, cleanDormant, cleanOrphaned, dryRun, yes bool) {
	title := "Cleanup Execution"
	if dryRun {
		title = "Cleanup Execution (Dry Run)"
	}
	printBoxHeader(title)

	prefix := ""
	if dryRun {
		prefix = "[dry-run] "
	}

	var totalReclaimedTmp int64
	var totalReclaimedSwap int64

	pathWidth := termWidth - (len(prefix) + 2 + 1 + 1 + 6 + 1 + 10 + 2)
	if pathWidth < 20 {
		pathWidth = 20
	}

	if cleanUntouched {
		var targets []SwapItem
		for _, it := range items {
			if it.Category == CatUntouchedCache && it.Cleanable {
				targets = append(targets, it)
			}
		}
		totalReclaimedTmp += pruneTmpTargets(targets, "Untouched Build Caches", "Prune stale untouched build caches?", dryRun, yes, pathWidth, prefix)
	}

	if cleanAbandoned {
		var targets []SwapItem
		for _, it := range items {
			if it.Category == CatAbandonedRun && it.Cleanable {
				targets = append(targets, it)
			}
		}
		if cleanUntouched {
			fmt.Println()
		}
		totalReclaimedTmp += pruneTmpTargets(targets, "Abandoned Test Runs & Workspaces", "Prune abandoned test workspaces and review runs?", dryRun, yes, pathWidth, prefix)
	}

	if cleanOrphaned {
		var targets []SwapItem
		for _, it := range items {
			if it.Category == CatAbandonedAgent && it.Cleanable {
				targets = append(targets, it)
			}
		}
		if cleanUntouched || cleanAbandoned {
			fmt.Println()
		}
		totalReclaimedSwap += killProcessTargets(targets, "Orphaned Agent Processes", "End orphaned agent processes whose parent shell died?", dryRun, yes, pathWidth, prefix)
	}

	if cleanDormant {
		var targets []SwapItem
		for _, it := range items {
			if it.Category == CatDormantAgent && it.Cleanable {
				targets = append(targets, it)
			}
		}
		if cleanUntouched || cleanAbandoned || cleanOrphaned {
			fmt.Println()
		}
		totalReclaimedSwap += killProcessTargets(targets, "Dormant Agent Sessions", "End dormant agent processes? (Conversations stay preserved on disk)", dryRun, yes, pathWidth, prefix)
	}

	if totalReclaimedTmp > 0 || totalReclaimedSwap > 0 {
		var parts []string
		if totalReclaimedTmp > 0 {
			parts = append(parts, fmt.Sprintf("%s tmp", formatBytes(totalReclaimedTmp)))
		}
		if totalReclaimedSwap > 0 {
			parts = append(parts, fmt.Sprintf("%s swap", formatBytes(totalReclaimedSwap)))
		}
		joined := strings.Join(parts, ", ")
		if dryRun {
			fmt.Printf("\nWould reclaim %s\n", joined)
		} else {
			fmt.Printf("\nReclaimed %s\n", joined)
		}
	} else {
		fmt.Printf("\n%s\n", dim("Nothing to reclaim."))
	}
}

func askConfirmation(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	ans, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	return ans == "y" || ans == "yes"
}

