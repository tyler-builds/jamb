//go:build windows

package app

import (
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
	if !sidPattern.MatchString(names.sid) {
		t.Errorf("sid = %q; want a SID", names.sid)
	}
	if !strings.HasPrefix(names.mutex, `Local\jamb-JambExamples.Notes`) {
		t.Errorf("mutex = %q", names.mutex)
	}
	if !strings.HasPrefix(names.pipe, `\\.\pipe\jamb-JambExamples.Notes-`) {
		t.Errorf("pipe = %q", names.pipe)
	}
}
