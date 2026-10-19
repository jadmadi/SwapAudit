package version

import (
	"strings"
	"testing"
)

func TestVersionString(t *testing.T) {
	s := String()
	if !strings.HasPrefix(s, "swapaudit ") {
		t.Errorf("String() = %q, expected prefix 'swapaudit '", s)
	}
	if !strings.Contains(s, "commit:") {
		t.Errorf("String() = %q, missing 'commit:'", s)
	}
	if !strings.Contains(s, "built:") {
		t.Errorf("String() = %q, missing 'built:'", s)
	}
}
