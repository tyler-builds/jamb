package ui

import (
	"math"
	"testing"
)

// The segment of an indeterminate bar slides in from the left end and out at the right
// end: it is wholly outside at the start and end of a cycle, so the repeat does not jump,
// it is whole in the middle, and it moves without jumps between nearby phases.
func TestIndeterminateSegmentSlidesInAndOut(t *testing.T) {
	for _, p := range []float32{0, 1} {
		if _, w := segmentSpan(p); w != 0 {
			t.Errorf("at phase %v the segment shows %v of the track, want none", p, w)
		}
	}
	if s, w := segmentSpan(0.5); math.Abs(float64(w-progressSegment)) > 1e-6 || s <= 0 || s+w >= 1 {
		t.Errorf("at phase 0.5 the segment is at %v, %v wide; want it whole and inside", s, w)
	}
	// Entering: it is drawn from the left end, and grows.
	if s, w := segmentSpan(0.1); s != 0 || w <= 0 || w >= progressSegment {
		t.Errorf("entering, the segment is at %v, %v wide; want at the left end, partly shown", s, w)
	}
	// Leaving: it reaches the right end, and shrinks.
	if s, w := segmentSpan(0.95); math.Abs(float64(s+w-1)) > 1e-6 || w <= 0 || w >= progressSegment {
		t.Errorf("leaving, the segment is at %v, %v wide; want at the right end, partly shown", s, w)
	}
	// Both edges move continuously while the segment shows; a hidden segment has no place.
	const step = 0.001
	for p := float32(0); p+step <= 1; p += step {
		s0, w0 := segmentSpan(p)
		s1, w1 := segmentSpan(p + step)
		if w0 == 0 || w1 == 0 {
			if w0+w1 > 0.01 {
				t.Fatalf("the segment appears or vanishes at more than a sliver between phases %v and %v", p, p+step)
			}
			continue
		}
		if math.Abs(float64(s1-s0)) > 0.01 || math.Abs(float64((s1+w1)-(s0+w0))) > 0.01 {
			t.Fatalf("the segment jumps between phases %v and %v: [%v, %v] to [%v, %v]", p, p+step, s0, s0+w0, s1, s1+w1)
		}
	}
}
