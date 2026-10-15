package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/text"
)

// A canvas's caret blinks from the activity time it is given: on for half the blink period, then
// off, with a wake-up at each change, and solid again after caretBlinkStop.
func TestPainterCaretBlinksFromActivity(t *testing.T) {
	r := newEditorRig(t, func(*Context) Element { return Div() })
	var wakes []time.Time
	r.rt.hostSetServices(platformServices{WakeAt: func(at time.Time) { wakes = append(wakes, at) }})
	r.rt.SetSystemSettings(SystemSettings{CaretBlink: 200 * time.Millisecond})
	base := r.now
	p := &Painter{rt: r.rt}
	at := func(d time.Duration) bool {
		r.rt.lastFrame = base.Add(d)
		return p.CaretVisible(base)
	}
	if !at(0) {
		t.Fatal("the caret is off at its activity")
	}
	if len(wakes) != 1 || !wakes[0].Equal(base.Add(200*time.Millisecond)) {
		t.Fatalf("wake-ups %v, want one at 200 ms", wakes)
	}
	if at(250 * time.Millisecond) {
		t.Fatal("the caret is on in the second half of its blink")
	}
	if wakes[len(wakes)-1] != base.Add(400*time.Millisecond) {
		t.Fatalf("the caret does not wake at its next change: last wake %v", wakes[len(wakes)-1].Sub(base))
	}
	if !at(450 * time.Millisecond) {
		t.Fatal("the caret is off in the third quarter of its blink, which is on")
	}
	wakes = nil
	if !at(caretBlinkStop + time.Second) {
		t.Fatal("the caret is off after the blink has stopped")
	}
	if len(wakes) != 0 {
		t.Fatalf("the caret scheduled %d wake-ups after it went solid, want none", len(wakes))
	}
	// Activity restarts the blink: the caret is on at once.
	r.rt.lastFrame = base.Add(caretBlinkStop + 2*time.Second)
	if !p.CaretVisible(r.rt.lastFrame) {
		t.Fatal("the caret is off at the activity that restarts it")
	}
	if !p.Now().Equal(r.rt.lastFrame) {
		t.Fatalf("Now = %v, want the frame time", p.Now())
	}
}

// A caret is drawn solid, and schedules nothing, when the system's caret blink is off, when the
// user asks for reduced motion, and in an inactive window, where it is not drawn at all.
func TestPainterCaretIsSolidOrHiddenWithoutBlinking(t *testing.T) {
	cases := []struct {
		name  string
		set   func(r *editorRig)
		drawn bool
	}{
		{"blink off", func(r *editorRig) { r.rt.SetSystemSettings(SystemSettings{CaretBlinkOff: true}) }, true},
		{"reduced motion", func(r *editorRig) { r.rt.SetSystemSettings(SystemSettings{ReducedMotion: true}) }, true},
		{"inactive window", func(r *editorRig) { r.rt.windowInactive = true }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newEditorRig(t, func(*Context) Element { return Div() })
			var wakes []time.Time
			r.rt.hostSetServices(platformServices{WakeAt: func(at time.Time) { wakes = append(wakes, at) }})
			c.set(r)
			r.rt.lastFrame = r.now.Add(250 * time.Millisecond)
			p := &Painter{rt: r.rt}
			if got := p.CaretVisible(r.now); got != c.drawn {
				t.Errorf("CaretVisible = %v, want %v", got, c.drawn)
			}
			if len(wakes) != 0 {
				t.Errorf("scheduled %d wake-ups, want none", len(wakes))
			}
		})
	}
}

// A canvas that draws a blinking caret draws again at each change of the caret, when no other
// part of the UI changed: the frame at the change repaints the canvas.
func TestCanvasCaretIsDrawnAgainAtEachBlink(t *testing.T) {
	// The rig's first frame runs on the real clock, so the activity is the real time too.
	base := time.Now()
	draws := 0
	r := newEditorRig(t, func(*Context) Element {
		return Div().Child(Canvas(func(p *Painter, w, h float32) {
			draws++
			p.CaretVisible(base)
		}).Size(Px(100), Px(20)))
	})
	before := draws
	if before == 0 {
		t.Fatal("the canvas was not drawn")
	}
	r.now = base.Add(400 * time.Millisecond)
	r.rt.Frame(r.now)
	if draws != before {
		t.Fatalf("the canvas was drawn again %d times before its caret changed, want 0", draws-before)
	}
	r.now = base.Add(530 * time.Millisecond)
	r.rt.Frame(r.now)
	if draws != before+1 {
		t.Fatalf("the canvas was drawn %d times at the caret's change, want 1", draws-before)
	}
}

// TextFormat.Italic reaches the text system's style, and the monospace family keeps it.
func TestTextStyleItalicReachesTheTextSystem(t *testing.T) {
	r := newEditorRig(t, func(*Context) Element { return Div() })
	p := &Painter{rt: r.rt}
	if l := p.layout("code", TextFormat{Italic: true, Mono: true}); !l.Style.Italic || l.Style.Family != text.FamilyMono {
		t.Errorf("italic mono style = %+v, want italic in the monospace family", l.Style)
	}
	if p.layout("code", TextFormat{}).Style.Italic {
		t.Error("text without Italic is set in the italic face")
	}
}
