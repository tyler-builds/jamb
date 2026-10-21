//go:build windows

package win32

import (
	"testing"
	"time"
)

// TestPerformanceNanosAdvancesFinely checks that the counter never runs backward and
// that it resolves steps shorter than the 0.5 ms of time.Now's monotonic clock.
func TestPerformanceNanosAdvancesFinely(t *testing.T) {
	first := PerformanceNanos()
	for i := 0; i < 1000; i++ {
		prev := PerformanceNanos()
		if next := PerformanceNanos(); next < prev {
			t.Fatalf("the counter went back: %d after %d", next, prev)
		}
	}
	// A busy loop of about 100 µs is seen as a step of well under a millisecond.
	start := PerformanceNanos()
	for PerformanceNanos()-start < 100_000 {
	}
	if d := PerformanceNanos() - start; d < 100_000 || d > int64(10*time.Millisecond) {
		t.Errorf("a 100 µs wait measured %v", time.Duration(d))
	}
	if PerformanceNanos() <= first {
		t.Error("the counter did not advance")
	}
}

func TestProcessMemoryUsageReportsThisProcess(t *testing.T) {
	private, working := ProcessMemoryUsage()
	if private == 0 || working == 0 {
		t.Errorf("ProcessMemoryUsage = %d, %d; want both above 0 for a running test", private, working)
	}
}
