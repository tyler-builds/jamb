package ui

import (
	"math"
	"testing"
	"time"
)

// Keyframes repeating out of view ask for no frames, and when they come into
// view they show the progress their time implies.
func TestOffscreenKeyframesRequestNoFrames(t *testing.T) {
	const d = 400 * time.Millisecond
	r := animRig(animColumn(func() Element {
		return Div().Key("dot").Size(Px(10), Px(10)).Background(Hex("#ffffff")).
			Play("pulse", []Keyframe{{At: 0, Motion: Fade()}, {At: 1, Motion: Motion{}.Opacity(1)}},
				Animation{Duration: d, Repeat: true, Easing: linear})
	}))
	start := r.now
	r.settle(t, 16*time.Millisecond)
	stepFrames(r, 30, 16*time.Millisecond)
	if r.rt.NeedsFrame() {
		t.Fatal("off-screen repeating keyframes ask for frames")
	}
	r.now = r.now.Add(time.Second + 100*time.Millisecond)
	r.rt.Scroll(100, 100, 0, 1500, true, 0)
	r.frame()
	if !r.rt.NeedsFrame() {
		t.Fatal("repeating keyframes scrolled into view do not ask for frames")
	}
	want := float32(math.Mod(r.now.Sub(start).Seconds()/d.Seconds(), 1))
	if s, _ := r.rt.StyleOf("dot"); math.Abs(float64(s.Opacity-want)) > 1e-3 {
		t.Errorf("opacity on resume = %v, want %v (time based, no jump)", s.Opacity, want)
	}
}

// Keyframes widen an element's ink to their farthest stop.
func TestKeyframesOverhang(t *testing.T) {
	sp := (&DivElement{}).Play(0, []Keyframe{{At: 0.3, Motion: Slide(0, 8)}, {At: 0.6, Motion: Slide(3, 4).Grow(0.7)}}, Animation{}).play
	over, grow := inkOverhang(&Style{}, nil, nil, sp)
	if over != 8 || math.Abs(float64(grow-0.3)) > 1e-6 {
		t.Errorf("overhang %v, growth %v; want 8, 0.3", over, grow)
	}
}

// A timeline's steps run one after another in a sequence, together in
// parallel, and the timeline lasts until its last step ends.
func TestTimelineProgress(t *testing.T) {
	a := Animation{Duration: 100 * time.Millisecond, Easing: linear}
	b := Animation{Duration: 200 * time.Millisecond, Easing: linear}
	seq := Sequence(a, b)
	if seq.Duration() != 300*time.Millisecond {
		t.Errorf("sequence lasts %v, want 300ms", seq.Duration())
	}
	par := Parallel(Sequence(a), Sequence(b), Sequence(a))
	if par.Duration() != 200*time.Millisecond {
		t.Errorf("parallel lasts %v, want 200ms (the longest)", par.Duration())
	}
	then := seq.Then(par)
	if then.Duration() != 500*time.Millisecond || len(then.steps) != 5 {
		t.Errorf("then lasts %v with %d steps, want 500ms and 5", then.Duration(), len(then.steps))
	}
	tests := []struct {
		tl   Timeline
		at   time.Duration
		want []float32
	}{
		{seq, 0, []float32{0, 0}},
		{seq, 50 * time.Millisecond, []float32{0.5, 0}},
		{seq, 100 * time.Millisecond, []float32{1, 0}},
		{seq, 200 * time.Millisecond, []float32{1, 0.5}},
		{seq, 300 * time.Millisecond, []float32{1, 1}},
		{par, 100 * time.Millisecond, []float32{1, 0.5, 1}},
		{then, 400 * time.Millisecond, []float32{1, 1, 1, 0.5, 1}},
		{Sequence(Animation{}, a), 0, []float32{1, 0}},
	}
	for i, tc := range tests {
		p := make([]float32, len(tc.tl.steps))
		tc.tl.progress(tc.at, p)
		for j := range p {
			if math.Abs(float64(p[j]-tc.want[j])) > 1e-4 {
				t.Errorf("case %d at %v: progress %v, want %v", i, tc.at, p, tc.want)
				break
			}
		}
	}
}
