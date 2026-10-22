package views

import (
	"fmt"
	"testing"
)

// TestRunIDMatchesSprintf checks that runID writes the IDs that fmt.Sprintf writes
// for the runs, which are numbered from 1 to 100,000.
func TestRunIDMatchesSprintf(t *testing.T) {
	for _, n := range []int{1, 9, 10, 99, 1000, 12345, 99999, 100000} {
		if got, want := runID(n), fmt.Sprintf("RUN-%06d", n); got != want {
			t.Errorf("runID(%d) = %q, want %q", n, got, want)
		}
	}
}
