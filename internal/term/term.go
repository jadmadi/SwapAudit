//go:build linux

package term

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/jadmadi/swapaudit/internal/model"
)

var (
	termWidth    = 100
	colorEnabled = true
	asciiMode    = false
)

// GetTerminalWidth queries stdout for the current window width and clamps it between 80 and 120.
func GetTerminalWidth() int {
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

// InitTerminal configures terminal width, color activation, and ASCII fallback.
func InitTerminal(noColorFlag, asciiFlag bool) {
	termWidth = GetTerminalWidth()

	if asciiFlag {
		asciiMode = true
	} else {
		lang := os.Getenv("LANG")
		lcAll := os.Getenv("LC_ALL")
		if !strings.Contains(strings.ToUpper(lang), "UTF-8") && !strings.Contains(strings.ToUpper(lcAll), "UTF-8") {
			asciiMode = true
		}
	}

	if noColorFlag || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
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

// SetTermWidth overrides the terminal width (intended for tests).
func SetTermWidth(w int) {
	termWidth = w
}

// TermWidth returns the active terminal width.
func TermWidth() int {
	return termWidth
}

// SetAsciiMode forces ASCII mode (intended for tests).
func SetAsciiMode(enabled bool) {
	asciiMode = enabled
}

// SetColorEnabled forces color on/off (intended for tests).
func SetColorEnabled(enabled bool) {
	colorEnabled = enabled
}

// Bold formats string with ANSI bold.
func Bold(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[1m" + s + "\x1b[0m"
}

// Dim formats string with ANSI dim.
func Dim(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}

// Red formats string with ANSI red.
func Red(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[31m" + s + "\x1b[0m"
}

// Green formats string with ANSI green.
func Green(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[32m" + s + "\x1b[0m"
}

// Yellow formats string with ANSI yellow.
func Yellow(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[33m" + s + "\x1b[0m"
}

// Cyan formats string with ANSI cyan.
func Cyan(s string) string {
	if !colorEnabled {
		return s
	}
	return "\x1b[36m" + s + "\x1b[0m"
}

// GlyphBoxTopLeft returns the top-left box border character.
func GlyphBoxTopLeft() string {
	if asciiMode {
		return "+"
	}
	return "╭"
}

// GlyphBoxTopRight returns the top-right box border character.
func GlyphBoxTopRight() string {
	if asciiMode {
		return "+"
	}
	return "╮"
}

// GlyphBoxBottomLeft returns the bottom-left box border character.
func GlyphBoxBottomLeft() string {
	if asciiMode {
		return "+"
	}
	return "╰"
}

// GlyphBoxBottomRight returns the bottom-right box border character.
func GlyphBoxBottomRight() string {
	if asciiMode {
		return "+"
	}
	return "╯"
}

// GlyphBoxHoriz returns the horizontal box border character.
func GlyphBoxHoriz() string {
	if asciiMode {
		return "-"
	}
	return "─"
}

// GlyphBoxVert returns the vertical box border character.
func GlyphBoxVert() string {
	if asciiMode {
		return "|"
	}
	return "│"
}

// GlyphBarFilled returns the progress bar filled segment.
func GlyphBarFilled() string {
	if asciiMode {
		return "#"
	}
	return "█"
}

// GlyphBarEmpty returns the progress bar empty segment.
func GlyphBarEmpty() string {
	if asciiMode {
		return "."
	}
	return "░"
}

// GlyphAccent returns the section accent icon.
func GlyphAccent() string {
	if asciiMode {
		return "*"
	}
	return "◆"
}

// GlyphActiveIndicator returns the indicator for actionable rows.
func GlyphActiveIndicator() string {
	if asciiMode {
		return ">"
	}
	return "▸"
}

// GlyphCheck returns the success check icon.
func GlyphCheck() string {
	if asciiMode {
		return "OK"
	}
	return "✓"
}

// GlyphCross returns the error cross icon.
func GlyphCross() string {
	if asciiMode {
		return "FAIL"
	}
	return "✗"
}

// GlyphDash returns a dash or em-dash substitute.
func GlyphDash() string {
	if asciiMode {
		return "-"
	}
	return "—"
}

// GlyphEllipsis returns ellipsis glyphs.
func GlyphEllipsis() string {
	if asciiMode {
		return "..."
	}
	return "…"
}

// GlyphWarn returns a warning indicator.
func GlyphWarn() string {
	if asciiMode {
		return "!"
	}
	return "⚠"
}

// StrWidth calculates the visible display width of a string, ignoring ANSI escape codes.
func StrWidth(s string) int {
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

// Truncate truncates a string to fit within w visible cells, ending with an ellipsis.
func Truncate(s string, w int) string {
	if StrWidth(s) <= w {
		return s
	}
	ell := GlyphEllipsis()
	ellW := StrWidth(ell)
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

// FormatBytes formats a byte count into a human-readable string.
func FormatBytes(b int64) string {
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

// FormatDuration formats a time.Duration into a concise 1-2 unit string.
func FormatDuration(d time.Duration) string {
	if d < 0 {
		return GlyphDash()
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

// RenderMeterBar draws a progress meter bar with green/yellow/red color thresholds.
func RenderMeterBar(percent float64, width int) string {
	filled := int((percent / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled
	rawBar := "[" + strings.Repeat(GlyphBarFilled(), filled) + strings.Repeat(GlyphBarEmpty(), empty) + "]"
	if percent < 50.0 {
		return Green(rawBar)
	} else if percent < 80.0 {
		return Yellow(rawBar)
	}
	return Red(rawBar)
}

// RenderShareMiniBar draws a compact meter bar followed by percentage text.
func RenderShareMiniBar(pct float64, width int) string {
	return fmt.Sprintf("%s %5.1f%%", RenderMeterBar(pct, width), pct)
}

// RenderTable outputs a formatted ASCII or Unicode table matching column width specifications.
func RenderTable(cols []model.Col, rows [][]string) {
	if len(cols) == 0 {
		return
	}

	var headerParts []string
	for _, c := range cols {
		title := c.Title
		tw := StrWidth(title)
		if tw > c.Width {
			title = Truncate(title, c.Width)
			tw = StrWidth(title)
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
			vw := StrWidth(r[0])
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
			vw := StrWidth(val)
			if vw > c.Width {
				val = Truncate(val, c.Width)
				vw = StrWidth(val)
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

// PrintBoxHeader prints an enclosed banner header aligned to the terminal width.
func PrintBoxHeader(title string) {
	w := termWidth
	accent := GlyphAccent()
	content := fmt.Sprintf("%s  %s", accent, title)
	contentVisW := StrWidth(content)

	innerWidth := w - 4
	if contentVisW > innerWidth {
		content = Truncate(content, innerWidth)
		contentVisW = StrWidth(content)
	}
	pad := innerWidth - contentVisW
	if pad < 0 {
		pad = 0
	}

	tl := GlyphBoxTopLeft()
	tr := GlyphBoxTopRight()
	bl := GlyphBoxBottomLeft()
	br := GlyphBoxBottomRight()
	horiz := GlyphBoxHoriz()
	vert := GlyphBoxVert()

	topLine := tl + strings.Repeat(horiz, w-2) + tr
	botLine := bl + strings.Repeat(horiz, w-2) + br
	boldContent := Bold(content)
	midLine := fmt.Sprintf("%s %s%s %s", vert, boldContent, strings.Repeat(" ", pad), vert)

	fmt.Println(topLine)
	fmt.Println(midLine)
	fmt.Println(botLine)
}

// PrintUpdateNotice prints an advisory box notifying that a newer release is available.
func PrintUpdateNotice(current, latest string) {
	w := termWidth
	tl := GlyphBoxTopLeft()
	tr := GlyphBoxTopRight()
	bl := GlyphBoxBottomLeft()
	br := GlyphBoxBottomRight()
	horiz := GlyphBoxHoriz()
	vert := GlyphBoxVert()

	topLine := Cyan(tl + strings.Repeat(horiz, w-2) + tr)
	botLine := Cyan(bl + strings.Repeat(horiz, w-2) + br)

	innerWidth := w - 4
	line1 := fmt.Sprintf("A new release of swapaudit is available: %s -> %s", current, latest)
	line2 := "Run 'swapaudit upgrade' to update to the latest version."

	pad1 := innerWidth - StrWidth(line1)
	if pad1 < 0 {
		pad1 = 0
	}
	pad2 := innerWidth - StrWidth(line2)
	if pad2 < 0 {
		pad2 = 0
	}

	mid1 := fmt.Sprintf("%s %s%s %s", Cyan(vert), Yellow(line1), strings.Repeat(" ", pad1), Cyan(vert))
	mid2 := fmt.Sprintf("%s %s%s %s", Cyan(vert), Bold(line2), strings.Repeat(" ", pad2), Cyan(vert))

	fmt.Println()
	fmt.Println(topLine)
	fmt.Println(mid1)
	fmt.Println(mid2)
	fmt.Println(botLine)
}
