//go:build windows

package app

import (
	"regexp"
	"strings"
	"testing"
)

// TestCurrentInstanceNames reads the user's SID and the session only. It creates no
// mutex, pipe or shell object.
func TestCurrentInstanceNames(t *testing.T) {
	names, err := currentInstanceNames("JambExamples.Notes")
	if err != nil {
		t.Fatalf("currentInstanceNames: %v", err)
	}
	if !isSID(names.sid) {
		t.Errorf("sid = %q; want a SID", names.sid)
	}
	if !strings.HasPrefix(names.mutex, `Local\jamb-JambExamples.Notes`) {
		t.Errorf("mutex = %q", names.mutex)
	}
	if !strings.HasPrefix(names.pipe, `\\.\pipe\jamb-JambExamples.Notes-`) {
		t.Errorf("pipe = %q", names.pipe)
	}
}

// isSID accepts exactly what the pattern ^S-1-[0-9]+(-[0-9]+)+$ matches.
func TestIsSIDMatchesPattern(t *testing.T) {
	pattern := regexp.MustCompile(`^S-1-[0-9]+(-[0-9]+)+$`)
	for _, s := range []string{
		"S-1-5-21-1", "S-1-5-21-3623811015-3361044348-30300820-1013", "S-1-5-18", "S-1-1-0",
		"", "S-1-", "S-1-5", "S-1-5-", "S-1--5", "S-1-5--21", "S-1-5-21-", "-S-1-5-21",
		"S-2-5-21", "s-1-5-21", "S-1-5-21a", "S-1-5-2 1", "S-1-5-21\n", "S-1-5-21)(A;;GA;;;WD",
		"S-1-0x5-21", "S-1-5-21-1-", "S-1-+5-21", "S-1-５-21", "S-1-5-21-1-2-3-4-5-6-7",
	} {
		if got, want := isSID(s), pattern.MatchString(s); got != want {
			t.Errorf("isSID(%q) = %t; the pattern says %t", s, got, want)
		}
	}
}
