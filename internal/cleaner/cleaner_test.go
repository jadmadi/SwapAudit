//go:build linux

package cleaner

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jadmadi/swapaudit/internal/model"
)

func TestPruneTmpTargetsDryRun(t *testing.T) {
	targets := []model.SwapItem{
		{
			Path:      "/tmp/fake-test-run-1",
			SizeBytes: 1024 * 1024 * 10,
		},
		{
			Path:      "/tmp/fake-test-run-2",
			SizeBytes: 1024 * 1024 * 5,
		},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	reclaimed := PruneTmpTargets(targets, "Test Targets", "Prompt?", true, false, 40, "[dry-run] ")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if reclaimed != 1024*1024*15 {
		t.Errorf("PruneTmpTargets() reclaimed = %d, want %d", reclaimed, 1024*1024*15)
	}
	if !strings.Contains(output, "[dry-run]") {
		t.Errorf("PruneTmpTargets() output missing [dry-run] prefix: %s", output)
	}
	if !strings.Contains(output, "fake-test-run-1") {
		t.Errorf("PruneTmpTargets() output missing target: %s", output)
	}
}

func TestKillProcessTargetsDryRun(t *testing.T) {
	targets := []model.SwapItem{
		{
			PID:        99999,
			Name:       "dummy-agent",
			SwapBytes:  1024 * 1024 * 20,
			StartTicks: 12345,
		},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	reclaimed := KillProcessTargets(targets, "Agent Processes", "Kill?", true, false, 40, "[dry-run] ")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if reclaimed != 1024*1024*20 {
		t.Errorf("KillProcessTargets() reclaimed = %d, want %d", reclaimed, 1024*1024*20)
	}
	if !strings.Contains(output, "[dry-run]") {
		t.Errorf("KillProcessTargets() output missing [dry-run] prefix: %s", output)
	}
	if !strings.Contains(output, "dummy-agent") {
		t.Errorf("KillProcessTargets() output missing process name: %s", output)
	}
}

func TestExecuteCleanupDryRun(t *testing.T) {
	items := []model.SwapItem{
		{
			Category:  model.CatUntouchedCache,
			Cleanable: true,
			Path:      "/tmp/fake-cache",
			SizeBytes: 10 * 1024 * 1024,
		},
		{
			Category:  model.CatAbandonedRun,
			Cleanable: true,
			Path:      "/tmp/fake-run",
			SizeBytes: 5 * 1024 * 1024,
		},
		{
			Category:   model.CatAbandonedAgent,
			Cleanable:  true,
			PID:        88888,
			Name:       "dead-parent-agent",
			SwapBytes:  25 * 1024 * 1024,
			StartTicks: 999,
		},
		{
			Category:   model.CatDormantAgent,
			Cleanable:  true,
			PID:        88889,
			Name:       "dormant-agent",
			SwapBytes:  15 * 1024 * 1024,
			StartTicks: 1000,
		},
	}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	os.Stdout = w

	ExecuteCleanup(items, true, true, true, true, true, true)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "Would reclaim 15.0 MB tmp, 40.0 MB swap") {
		t.Errorf("ExecuteCleanup output missing expected reclaim summary: %s", output)
	}
}
