package ease

import (
	"math"
	"testing"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-5 }

func TestEndpointsAndMidpoints(t *testing.T) {
	tests := []struct {
		name string
		f    func(float32) float32
		mid  float32 // value at 0.5
	}{
		{"Linear", Linear, 0.5},
		{"In", In, 0.25},
		{"Out", Out, 0.75},
		{"InOut", InOut, 0.5},
		{"InCubic", InCubic, 0.125},
		{"OutCubic", OutCubic, 0.875},
		{"InOutCubic", InOutCubic, 0.5},
		{"Reverse(Out)", Reverse(Out), 0.25},
		{"Reverse(OutCubic)", Reverse(OutCubic), 0.125},
	}
	for _, tc := range tests {
		if v := tc.f(0); !near(v, 0) {
			t.Errorf("%s(0) = %v, want 0", tc.name, v)
		}
		if v := tc.f(1); !near(v, 1) {
			t.Errorf("%s(1) = %v, want 1", tc.name, v)
		}
		if v := tc.f(0.5); !near(v, tc.mid) {
			t.Errorf("%s(0.5) = %v, want %v", tc.name, v, tc.mid)
		}
	}
}

// The non-bounce functions never decrease, so an animation never runs back.
func TestMonotonic(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    func(float32) float32
	}{
		{"Linear", Linear}, {"In", In}, {"Out", Out}, {"InOut", InOut},
		{"InCubic", InCubic}, {"OutCubic", OutCubic}, {"InOutCubic", InOutCubic},
		{"Reverse(Out)", Reverse(Out)},
	} {
		prev := tc.f(0)
		for i := 1; i <= 100; i++ {
			v := tc.f(float32(i) / 100)
			if v < prev-1e-6 {
				t.Errorf("%s decreases at t=%v: %v after %v", tc.name, float32(i)/100, v, prev)
				break
			}
			prev = v
		}
	}
}

// Bounce goes out and back: it peaks at the midpoint and returns to 0.
func TestBounce(t *testing.T) {
	b := Bounce(Out)
	if v := b(0); !near(v, 0) {
		t.Errorf("Bounce(Out)(0) = %v, want 0", v)
	}
	if v := b(0.5); !near(v, 1) {
		t.Errorf("Bounce(Out)(0.5) = %v, want the peak 1", v)
	}
	if v := b(1); !near(v, 0) {
		t.Errorf("Bounce(Out)(1) = %v, want 0 (back where it started)", v)
	}
	if v := b(0.25); !near(v, Out(0.5)) {
		t.Errorf("Bounce(Out)(0.25) = %v, want Out(0.5) = %v", v, Out(0.5))
	}
	if v := b(0.75); !near(v, Out(0.5)) {
		t.Errorf("Bounce(Out)(0.75) = %v, want Out(0.5) = %v", v, Out(0.5))
	}
}

// Reverse(f) mirrors f, so Reverse(Out) is In.
func TestReverseMirrors(t *testing.T) {
	for i := 0; i <= 10; i++ {
		x := float32(i) / 10
		if v := Reverse(Out)(x); !near(v, In(x)) {
			t.Errorf("Reverse(Out)(%v) = %v, want In(%v) = %v", x, v, x, In(x))
		}
	}
}

// A critically damped spring rises without overshooting; one with a bounce
// overshoots 1 and comes back. Both start at 0 and end exactly at 1.
func TestSpring(t *testing.T) {
	for _, b := range []float32{0, 0.3, 0.9, -1, 2} {
		f := Spring(b)
		if v := f(0); v != 0 {
			t.Errorf("Spring(%v)(0) = %v, want 0", b, v)
		}
		if v := f(1); v != 1 {
			t.Errorf("Spring(%v)(1) = %v, want 1", b, v)
		}
	}
	critical := Spring(0)
	prev := float32(0)
	for i := 1; i <= 1000; i++ {
		x := float32(i) / 1000
		v := critical(x)
		if v < prev {
			t.Fatalf("Spring(0) decreases at %v: %v < %v", x, v, prev)
		}
		if v > 1 {
			t.Fatalf("Spring(0) overshoots at %v: %v", x, v)
		}
		prev = v
	}
	bouncy := Spring(0.3)
	var peak float32
	for i := 0; i <= 1000; i++ {
		peak = max(peak, bouncy(float32(i)/1000))
	}
	if peak <= 1.01 {
		t.Errorf("Spring(0.3) peaks at %v, want an overshoot past 1", peak)
	}
}
