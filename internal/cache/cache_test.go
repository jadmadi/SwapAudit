//go:build linux

package cache

import (
	"testing"
)

func TestMatchTmpPattern(t *testing.T) {
	patterns := []string{
		"cs1*", "sila*", "manara*", "test-bwrap*", "tsx-*", "custom-run",
	}

	tests := []struct {
		name     string
		expected bool
	}{
		{"sila-cli-test-profile", true},
		{"manara-backend", true},
		{"test-bwrap-instance", true},
		{"tsx-1234", true},
		{"custom-run-567", true},
		{"unrelated-file", false},
		{"chrome-cache", false},
	}

	for _, tt := range tests {
		matched := false
		for _, pat := range patterns {
			if MatchTmpPattern(pat, tt.name) {
				matched = true
				break
			}
		}
		if matched != tt.expected {
			t.Errorf("MatchTmpPattern against %q = %v, want %v", tt.name, matched, tt.expected)
		}
	}
}
