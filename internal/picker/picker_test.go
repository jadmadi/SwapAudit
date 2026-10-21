//go:build linux

package picker

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
	"github.com/jadmadi/swapaudit/internal/term"
)

func TestMain(m *testing.M) {
	term.SetColorEnabled(false)
	term.SetAsciiMode(false)
	os.Exit(m.Run())
}

func fixtureItems() []model.SwapItem {
	return []model.SwapItem{
		{PID: 100, StartTicks: 1, Name: "agent-a", Category: model.CatDormantAgent, SwapBytes: 300, Age: 5 * time.Hour, LastActive: 3 * time.Hour, Cleanable: true},
		{PID: 200, StartTicks: 2, Name: "agent-b", Category: model.CatDormantAgent, SwapBytes: 500, Age: 5 * time.Hour, LastActive: 4 * time.Hour, Cleanable: true},
		{PID: 300, StartTicks: 3, Name: "orphan", Category: model.CatAbandonedAgent, SwapBytes: 100, Age: 6 * time.Hour, LastActive: 5 * time.Hour, Cleanable: true},
		{Name: "go-build1", Path: "/tmp/go-build1", Category: model.CatUntouchedCache, SizeBytes: 2048, Age: 14 * time.Hour, LastActive: 13 * time.Hour, Cleanable: true},
		{Name: "go-build2", Path: "/tmp/go-build2", Category: model.CatUntouchedCache, SizeBytes: 1024, Age: 14 * time.Hour, LastActive: 13 * time.Hour, Cleanable: true},
		{Name: "run1", Path: "/tmp/run1", Category: model.CatAbandonedRun, SizeBytes: 4096, Age: 14 * time.Hour, LastActive: 13 * time.Hour, Cleanable: true},
		{PID: 999, StartTicks: 9, Name: "active", Category: model.CatActiveProcess, SwapBytes: 50, Age: time.Hour, LastActive: time.Minute, Cleanable: false},
	}
}

type execCall struct {
	untouched, abandoned, dormant, orphaned, dryRun, yes bool
}

type spy struct {
	calls []execCall
	scans int
	snaps int
}

func harness(items []model.SwapItem, input string, dryRun bool, width int) (int, string, *spy) {
	s := &spy{}
	var out bytes.Buffer
	cfg := Config{
		In:     bufio.NewReader(strings.NewReader(input)),
		Out:    &out,
		Items:  items,
		DryRun: dryRun,
		Width:  width,
		Scan:   func() []model.SwapItem { s.scans++; return items },
		Snapshot: func() model.SystemSwap {
			s.snaps++
			used := int64(500)
			if s.snaps > 1 {
				used = 400
			}
			return model.SystemSwap{SwapTotal: 1000, SwapUsed: used, SwapFree: 1000 - used, TmpfsSize: 1000, TmpfsUsed: used}
		},
		Execute: func(_ []model.SwapItem, u, a, d, o, dr, yes bool) {
			s.calls = append(s.calls, execCall{u, a, d, o, dr, yes})
		},
	}
	return Run(cfg), out.String(), s
}

func TestNilGuards(t *testing.T) {
	base := func() Config {
		return Config{
			In:       bufio.NewReader(strings.NewReader("q\n")),
			Out:      &bytes.Buffer{},
			Scan:     func() []model.SwapItem { return nil },
			Snapshot: func() model.SystemSwap { return model.SystemSwap{} },
			Execute:  func([]model.SwapItem, bool, bool, bool, bool, bool, bool) {},
		}
	}
	cases := []struct {
		name string
		cfg  Config
	}{
		{"nil out", Config{}},
		{"nil in", func() Config { c := base(); c.In = nil; return c }()},
		{"nil scan", func() Config { c := base(); c.Scan = nil; return c }()},
		{"nil snapshot", func() Config { c := base(); c.Snapshot = nil; return c }()},
		{"nil execute", func() Config { c := base(); c.Execute = nil; return c }()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code := Run(tc.cfg); code != 1 {
				t.Fatalf("Run() = %d, want 1", code)
			}
		})
	}
}

func TestExitPaths(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"eof", ""},
		{"empty line", "\n"},
		{"lower q", "q\n"},
		{"upper q", "Q\n"},
		{"eof at token", "1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, s := harness(fixtureItems(), tc.input, false, 100)
			if code != 0 {
				t.Fatalf("Run() = %d, want 0", code)
			}
			if len(s.calls) != 0 {
				t.Fatal("exit paths must not execute")
			}
		})
	}
}

func TestZeroTargetTier(t *testing.T) {
	items := []model.SwapItem{{PID: 1, Name: "active", Category: model.CatActiveProcess, SwapBytes: 10}}
	code, out, s := harness(items, "1\nq\n", false, 100)
	if code != 0 {
		t.Fatalf("Run() = %d, want 0", code)
	}
	if !strings.Contains(out, "tier 1: nothing to clean") {
		t.Errorf("missing empty-tier message:\n%s", out)
	}
	if len(s.calls) != 0 {
		t.Fatal("empty tier must not execute")
	}
}

func TestMismatchLimit(t *testing.T) {
	code, out, s := harness(fixtureItems(), "1\nx\ny\nz\nq\n", false, 100)
	if code != 0 {
		t.Fatalf("Run() = %d, want 0", code)
	}
	if got := strings.Count(out, "mismatch: expected 2 — nothing was changed."); got != 3 {
		t.Errorf("mismatch count = %d, want 3\n%s", got, out)
	}
	if !strings.Contains(out, "cancelled after 3 attempts — nothing was changed.") {
		t.Errorf("missing cancellation message:\n%s", out)
	}
	if len(s.calls) != 0 {
		t.Fatal("mismatches must not execute")
	}
}

func TestExecutorMatrix(t *testing.T) {
	cases := []struct {
		key   string
		token string
		want  [4]bool
	}{
		{"1", "2", [4]bool{true, false, false, false}},
		{"2", "1", [4]bool{false, true, false, false}},
		{"3", "200", [4]bool{false, false, true, false}},
		{"4", "300", [4]bool{false, false, false, true}},
	}
	for _, tc := range cases {
		t.Run("tier "+tc.key, func(t *testing.T) {
			code, out, s := harness(fixtureItems(), tc.key+"\n"+tc.token+"\nq\n", false, 100)
			if code != 0 {
				t.Fatalf("Run() = %d, want 0", code)
			}
			if len(s.calls) != 1 {
				t.Fatalf("executor calls = %d, want 1", len(s.calls))
			}
			c := s.calls[0]
			got := [4]bool{c.untouched, c.abandoned, c.dormant, c.orphaned}
			if got != tc.want {
				t.Errorf("tier booleans = %v, want %v", got, tc.want)
			}
			if c.dryRun || !c.yes {
				t.Errorf("dryRun/yes = %v/%v, want false/true", c.dryRun, c.yes)
			}
			if s.scans != 1 || s.snaps != 2 {
				t.Errorf("scans/snaps = %d/%d, want 1/2", s.scans, s.snaps)
			}
			if !strings.Contains(out, "delta swap -100 B") {
				t.Errorf("missing measured line:\n%s", out)
			}
		})
	}
}

func TestDryRun(t *testing.T) {
	code, out, s := harness(fixtureItems(), "1\n2\nq\n", true, 100)
	if code != 0 {
		t.Fatalf("Run() = %d, want 0", code)
	}
	if len(s.calls) != 1 || !s.calls[0].dryRun || !s.calls[0].yes {
		t.Fatalf("dry-run call = %+v", s.calls)
	}
	if s.scans != 0 || s.snaps != 1 {
		t.Errorf("dry run must skip rescan: scans=%d snaps=%d", s.scans, s.snaps)
	}
	if !strings.Contains(out, "ACTION PICKER (dry run)") {
		t.Errorf("missing dry-run menu label:\n%s", out)
	}
}

func TestTokenFor(t *testing.T) {
	fs := tokensForTier(t, 1)
	if fs != "2" {
		t.Errorf("filesystem token = %q, want 2", fs)
	}
	proc := tokensForTier(t, 3)
	if proc != "200" {
		t.Errorf("process token = %q, want 200", proc)
	}
	tie := []model.SwapItem{
		{PID: 7, SwapBytes: 100, Cleanable: true},
		{PID: 3, SwapBytes: 100, Cleanable: true},
	}
	if got := tokenFor(tiers[2], tie); got != "3" {
		t.Errorf("tie token = %q, want 3", got)
	}
}

func tokensForTier(t *testing.T, key int) string {
	t.Helper()
	targets := targetsFor(fixtureItems(), tiers[key-1].category)
	if len(targets) == 0 {
		t.Fatalf("no targets for tier %d", key)
	}
	return tokenFor(tiers[key-1], targets)
}

func TestMeasuredLine(t *testing.T) {
	before := model.SystemSwap{SwapTotal: 1000, SwapUsed: 500, TmpfsSize: 1000, TmpfsUsed: 600}
	after := model.SystemSwap{SwapTotal: 1000, SwapUsed: 400, TmpfsSize: 1000, TmpfsUsed: 500}
	want := "swap 500 B/1000 B (50.0%) -> 400 B/1000 B (40.0%) · tmpfs 600 B/1000 B -> 500 B/1000 B · delta swap -100 B · delta tmpfs -100 B"
	if got := measuredLine(before, after); got != want {
		t.Errorf("measuredLine() =\n%q\nwant\n%q", got, want)
	}
}

func TestPickerGolden(t *testing.T) {
	for _, width := range []int{80, 100, 120} {
		t.Run("width "+strconv.Itoa(width), func(t *testing.T) {
			code, out, _ := harness(fixtureItems(), "3\n\nq\n", false, width)
			if code != 0 {
				t.Fatalf("Run() = %d, want 0", code)
			}
			for _, line := range strings.Split(out, "\n") {
				if term.StrWidth(line) > width {
					t.Errorf("line exceeds width %d: %q (%d cells)", width, line, term.StrWidth(line))
				}
			}
			if strings.Contains(out, "\x1b") {
				t.Error("color is disabled but output contains ANSI escapes")
			}
			for _, header := range []string{"KEY", "TIER", "ITEMS", "SIZE", "RESOURCE", "SAFETY", "REVIEW tier 3 of 4", "FREES 800 B swap"} {
				if !strings.Contains(out, header) {
					t.Errorf("golden output missing %q", header)
				}
			}
		})
	}
}

func TestReviewTruncation(t *testing.T) {
	var items []model.SwapItem
	for i := 0; i < 12; i++ {
		items = append(items, model.SwapItem{
			Name: "go-build", Path: "/tmp/go-build", Category: model.CatUntouchedCache,
			SizeBytes: int64(i+1) * 1024, LastActive: 13 * time.Hour, Age: 14 * time.Hour, Cleanable: true,
		})
	}
	code, out, _ := harness(items, "1\n\nq\n", false, 100)
	if code != 0 {
		t.Fatalf("Run() = %d, want 0", code)
	}
	if !strings.Contains(out, "… and 2 more targets") {
		t.Errorf("missing truncation line:\n%s", out)
	}
}

func TestInvalidSelections(t *testing.T) {
	code, out, s := harness(fixtureItems(), "0\n5\n12\nx\nq\n", false, 100)
	if code != 0 {
		t.Fatalf("Run() = %d, want 0", code)
	}
	if got := strings.Count(out, "invalid selection:"); got != 4 {
		t.Errorf("invalid selection count = %d, want 4\n%s", got, out)
	}
	if len(s.calls) != 0 {
		t.Fatal("invalid selections must not execute")
	}
}

func TestSizeCeilings(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("caller unavailable")
	}
	dir := filepath.Dir(file)
	prod, err := os.ReadFile(filepath.Join(dir, "picker.go"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(prod), "\n"); lines > 350 {
		t.Errorf("picker.go has %d lines, ceiling is 350", lines)
	}
	test, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(string(test), "\n"); lines > 400 {
		t.Errorf("picker_test.go has %d lines, ceiling is 400", lines)
	}
}
