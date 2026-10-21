//go:build linux

package live

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jadmadi/swapaudit/internal/model"
)

const (
	testGiB = int64(1024 * 1024 * 1024)
	testMiB = int64(1024 * 1024)
)

func fixedSample(at time.Time) sample {
	s := emptySample(at, model.SystemSwap{
		SwapTotal: 8 * testGiB, SwapUsed: 2 * testGiB, SwapFree: 6 * testGiB,
		MemTotal: 32 * testGiB, MemUsed: 16 * testGiB, MemAvail: 16 * testGiB,
		TmpfsSize: 4 * testGiB, TmpfsUsed: testGiB,
	})
	s.MonoSeconds = 12.5
	key := processKey{PID: 1234, StartTicks: 42}
	s.Processes[key] = 200 * testMiB
	s.Names[key] = "gopls"
	s.Caches["/tmp/go-build1"] = 100 * testMiB
	s.CacheAges["/tmp/go-build1"] = 15 * 3600
	s.TmpfsSampledAt = at
	s.TmpfsCleanableBytes = 100 * testMiB
	s.TmpfsCleanableItems = 1
	return s
}

func TestRecordReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.ndjson")
	rec, err := NewRecorder(path)
	if err != nil {
		t.Fatalf("NewRecorder failed: %v", err)
	}
	at := time.Date(2026, 10, 21, 10, 0, 0, 0, time.FixedZone("EEST", 3*3600))
	if err := rec.Write(fixedSample(at)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := fi.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %o, want 600", mode)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := Replay(f)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("replayed %d samples, want 1", len(got))
	}
	s := got[0]
	if s.MonoSeconds != 12.5 || s.System.SwapUsed != 2*testGiB {
		t.Errorf("system/mono = %+v / %v", s.System, s.MonoSeconds)
	}
	if s.Processes[processKey{PID: 1234, StartTicks: 42}] != 200*testMiB {
		t.Errorf("process replay failed: %+v", s.Processes)
	}
	if s.Names[processKey{PID: 1234, StartTicks: 42}] != "gopls" {
		t.Errorf("name replay failed: %+v", s.Names)
	}
	if s.Caches["/tmp/go-build1"] != 100*testMiB || s.CacheAges["/tmp/go-build1"] != 15*3600 {
		t.Errorf("cache replay failed: %+v %+v", s.Caches, s.CacheAges)
	}
	if s.TmpfsSampledAt.IsZero() {
		t.Error("tmpfs sampled_at was not replayed")
	}
}

func TestRecordDeterministic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.ndjson")
	rec, err := NewRecorder(path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	if err := rec.Write(fixedSample(at)); err != nil {
		t.Fatal(err)
	}
	if err := rec.Write(fixedSample(at)); err != nil {
		t.Fatal(err)
	}
	rec.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || lines[0] != lines[1] {
		t.Errorf("raw lines are not deterministic: %q", data)
	}
	if !strings.Contains(lines[0], `"type":"raw_sample"`) {
		t.Errorf("line missing type discriminator: %s", lines[0])
	}
}

func TestReplayInvalid(t *testing.T) {
	if _, err := Replay(strings.NewReader("{not json")); err == nil {
		t.Error("Replay must reject malformed input")
	}
}

func TestReplayAnalysisEquivalence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.ndjson")
	rec, err := NewRecorder(path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	first := fixedSample(at)
	second := fixedSample(at.Add(time.Second))
	second.MonoSeconds = 13.5
	second.System.SwapUsed += 10 * testMiB
	second.Processes[processKey{PID: 1234, StartTicks: 42}] += 10 * testMiB
	if err := rec.Write(first); err != nil {
		t.Fatal(err)
	}
	if err := rec.Write(second); err != nil {
		t.Fatal(err)
	}
	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}

	replayed := readRecorded(t, path)
	want, err := json.Marshal(Analyze([]sample{first, second}, time.Minute, second.TmpfsSampledAt, second.At))
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(Analyze(replayed, time.Minute, replayed[1].TmpfsSampledAt, replayed[1].At))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("replayed analysis differs\nwant %s\ngot  %s", want, got)
	}
}
