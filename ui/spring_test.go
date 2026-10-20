package ui

import (
	"math"
	"testing"
	"time"
)

// A critically damped spring released from rest never passes its target,
// and every spring loses energy as it moves, whatever the step.
func TestSpringStep(t *testing.T) {
	tests := []struct {
		name   string
		spring Spring
		dt     float32
	}{
		{"critical 16ms", Spring{Duration: 300 * time.Millisecond}, 0.016},
		{"critical 100ms", Spring{Duration: 300 * time.Millisecond}, 0.1},
		{"bouncy 16ms", Spring{Duration: 300 * time.Millisecond, Bounce: 0.5}, 0.016},
		{"bouncy 7ms", Spring{Duration: 500 * time.Millisecond, Bounce: 0.9}, 0.007},
		{"bounce clamped", Spring{Duration: 200 * time.Millisecond, Bounce: 3}, 0.016},
	}
	for _, tc := range tests {
		omega, zeta := tc.spring.params()
		energy := func(x, v float32) float64 {
			y := float64(x - 100)
			return float64(v)*float64(v) + omega*omega*y*y
		}
		x, v := float32(0), float32(0)
		e := energy(x, v)
		var peak float32
		for i := 0; i < 2000; i++ {
			x, v = springStep(x, v, 100, tc.dt, omega, zeta)
			peak = max(peak, x)
			if ne := energy(x, v); ne > e*(1+1e-5)+1e-4 {
				t.Fatalf("%s: energy rose from %v to %v at step %d", tc.name, e, ne, i)
			} else {
				e = ne
			}
		}
		if zeta >= 1 && peak > 100+1e-3 {
			t.Errorf("%s: a critically damped spring overshot to %v", tc.name, peak)
		}
		if tc.spring.Bounce > 0 && peak <= 100.5 {
			t.Errorf("%s: a spring with a bounce peaked at %v, want past 100", tc.name, peak)
		}
		if math.Abs(float64(x-100)) > 1e-2 {
			t.Errorf("%s: the spring ended at %v, want 100", tc.name, x)
		}
	}
	// One long step lands where many short ones do: the step is exact.
	omega, zeta := Spring{Duration: 300 * time.Millisecond, Bounce: 0.3}.params()
	x1, v1 := springStep(0, 0, 100, 0.08, omega, zeta)
	x2, v2 := float32(0), float32(0)
	for i := 0; i < 8; i++ {
		x2, v2 = springStep(x2, v2, 100, 0.01, omega, zeta)
	}
	if math.Abs(float64(x1-x2)) > 1e-3 || math.Abs(float64(v1-v2)) > 1e-2 {
		t.Errorf("one 80ms step gives (%v, %v), eight 10ms steps (%v, %v)", x1, v1, x2, v2)
	}
}

// The scale an element is drawn at, its spring included, is in its
// DrawnStyle, and a scale below 1 widens its ink as much as one above.
func TestDrawnScaleAndGrowth(t *testing.T) {
	r := animRig(func(ctx *Context) Element {
		return Div().Row().Child(Div().Key("box").Size(Px(40), Px(40)).Scale(0.8))
	})
	d, _ := r.rt.StyleOf("box")
	if d.Scale != 0.8 {
		t.Errorf("drawn scale %v, want 0.8", d.Scale)
	}
	if g := growth(0.8); math.Abs(float64(g-0.2)) > 1e-6 {
		t.Errorf("growth(0.8) = %v, want 0.2", g)
	}
	if g := growth(0); g != 0 {
		t.Errorf("growth of an unset scale = %v, want 0", g)
	}
}
