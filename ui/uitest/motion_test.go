package uitest_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ease"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

var (
	red  = ui.RGB(200, 40, 40)
	blue = ui.RGB(40, 40, 200)
)

// box is a 100 × 100 filled box with the key.
func box(key string) *ui.DivElement {
	return ui.Div().Key(key).Size(ui.Px(100), ui.Px(100)).Background(red).Shrink(0)
}

// kinds returns the kinds and elements of fs, for failure messages and
// comparisons.
func kinds(fs []uitest.Finding) string {
	var b []string
	for _, f := range fs {
		b = append(b, fmt.Sprintf("%s %s", f.Kind, f.Element))
	}
	return strings.Join(b, "; ")
}

// shown is a UI that shows the box made by mk while on is set.
func shown(t *testing.T, on *ui.Signal[bool], mk func() *ui.DivElement) *uitest.UI {
	return uitest.New(t, func(ctx *ui.Context) ui.Element {
		root := ui.Div().Child(ui.Button("Toggle").Key("toggle").OnClick(func() { on.Set(!on.Peek()) }))
		if on.Get() {
			root.Child(mk())
		}
		return root
	})
}

// An element shown and removed with no transition pops in and out; one with an
// entrance and an exit does not.
func TestMotionPops(t *testing.T) {
	for _, tc := range []struct {
		name string
		mk   func() *ui.DivElement
		want string
	}{
		{"plain", func() *ui.DivElement { return box("panel") }, "pop-in panel; pop-out panel"},
		{"fading", func() *ui.DivElement {
			return box("panel").Enter(ui.Fade(), ui.Animation{Duration: 120 * time.Millisecond}).
				Exit(ui.Fade(), ui.Animation{Duration: 80 * time.Millisecond})
		}, ""},
		{"linear fade", func() *ui.DivElement {
			// The exit's last frame is drawn a frame before its end, at the fade's last step.
			return box("panel").Enter(ui.Fade(), ui.Animation{Duration: 90 * time.Millisecond, Easing: ease.Linear}).
				Exit(ui.Fade(), ui.Animation{Duration: 90 * time.Millisecond, Easing: ease.Linear})
		}, ""},
		{"growing", func() *ui.DivElement {
			return box("panel").Enter(ui.Grow(0.9), ui.Animation{Duration: 120 * time.Millisecond})
		}, "pop-out panel"},
		{"tiny", func() *ui.DivElement { return box("panel").Size(ui.Px(12), ui.Px(12)) }, ""},
		{"faint", func() *ui.DivElement { return box("panel").Opacity(0.3) }, ""},
		{"blank holder", func() *ui.DivElement {
			// The pop is reported once, on the outermost element that came and went.
			return ui.Div().Key("holder").Child(box("a")).Child(box("b"))
		}, "pop-in holder; pop-out holder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			on := ui.NewSignal(false)
			u := shown(t, on, tc.mk)
			rec := u.Record()
			u.Click("toggle")
			u.Settle()
			u.Click("toggle")
			u.Settle()
			if got := kinds(rec.Findings()); got != tc.want {
				t.Errorf("findings %q, want %q", got, tc.want)
			}
		})
	}
}

// Display(false) hides an element as removing it does, and showing it again
// with no transition pops it in.
func TestMotionDisplayPops(t *testing.T) {
	on := ui.NewSignal(false)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Child(box("panel").Display(on.Get()))
	})
	rec := u.Record()
	on.Set(true)
	u.Frame()
	if got := kinds(rec.Findings()); got != "pop-in panel" {
		t.Errorf("findings %q, want a pop-in of the panel", got)
	}
}

// An element scrolled into view, or brought into view by the layout, is not a
// pop: it was there all along.
func TestMotionScrollIsNotAPop(t *testing.T) {
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		list := ui.Div().Key("list").Column().Height(ui.Px(200)).ScrollY()
		for i := range 20 {
			list.Child(box(fmt.Sprintf("row%d", i)))
		}
		return list
	}, uitest.Size(300, 300))
	rec := u.Record()
	u.Get("list").Scroll(0, 600)
	u.Settle()
	u.Get("list").Scroll(0, -600)
	u.Settle()
	if fs := rec.Findings(); len(fs) > 0 {
		t.Errorf("findings while scrolling: %v", fs)
	}
}

// A fade cut short is a cut exit.
func TestMotionCutExit(t *testing.T) {
	on, there := ui.NewSignal(true), ui.NewSignal(true)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		root := ui.Div()
		if there.Get() {
			op := float32(1)
			if !on.Get() {
				op = 0
			}
			root.Child(box("panel").Transition(200 * time.Millisecond).Opacity(op))
		}
		return root
	})
	rec := u.Record()
	on.Set(false)
	u.Advance(64 * time.Millisecond)
	there.Set(false)
	u.Frame()
	fs := rec.Findings()
	if got := kinds(fs); got != "cut-exit panel" {
		t.Fatalf("findings %q, want a cut exit of the panel", got)
	}
	if !strings.Contains(fs[0].String(), "removed at opacity") {
		t.Errorf("finding %q does not say the opacity it was removed at", fs[0])
	}
}

// A change of position in one frame while something animates, with no input,
// is a jump. The same change following a click is not, nor is a glide.
func TestMotionJump(t *testing.T) {
	gap := ui.NewSignal[float32](0)
	moving := ui.NewSignal[float32](0)
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Children(
			ui.Button("Move").Key("move").OnClick(func() { gap.Set(gap.Peek() + 40) }),
			ui.Div().Key("glider").Size(ui.Px(20), ui.Px(20)).Background(blue).
				Transition(300*time.Millisecond).Offset(moving.Get(), 0),
			ui.Div().Height(ui.Px(gap.Get())),
			box("panel"),
		)
	})
	rec := u.Record()
	moving.Set(200)
	u.Advance(48 * time.Millisecond)
	gap.Set(40) // data arrives while the glider moves
	u.Frame()
	u.Advance(48 * time.Millisecond)
	u.Click("move") // the user moves it
	u.Settle()
	fs := rec.Findings()
	if got := kinds(fs); got != "jump panel" {
		t.Fatalf("findings %q, want one jump of the panel", got)
	}
	if !strings.Contains(fs[0].Detail, "y ") {
		t.Errorf("the jump %q does not name the edge that moved", fs[0])
	}
}

// A spring with a bounce overshoots; one without does not, nor does a
// repeating animation going back to its start.
func TestMotionOvershootAndLoops(t *testing.T) {
	for _, tc := range []struct {
		bounce float32
		want   string
	}{{0, ""}, {0.5, "overshoot box"}} {
		x := ui.NewSignal[float32](0)
		u := uitest.New(t, func(ctx *ui.Context) ui.Element {
			return ui.Div().Child(ui.Div().Key("box").Size(ui.Px(40), ui.Px(40)).Background(red).
				Spring(ui.Spring{Duration: 300 * time.Millisecond, Bounce: tc.bounce}).Offset(x.Get(), 0))
		})
		rec := u.Record()
		x.Set(100)
		u.Settle()
		if got := kinds(rec.Findings()); got != tc.want {
			t.Errorf("bounce %v: findings %q, want %q", tc.bounce, got, tc.want)
		}
	}

	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Animate("loop", ui.Animation{Duration: 200 * time.Millisecond, Easing: ease.Linear, Repeat: true},
			func(p float32) ui.Element {
				return ui.Div().Child(ui.Div().Key("dot").Size(ui.Px(40), ui.Px(40)).Background(red).MarginLeft(300 * p))
			})
	})
	rec := u.Record()
	u.Advance(time.Second)
	if fs := rec.Findings(); len(fs) > 0 {
		t.Errorf("findings on a loop: %v", fs)
	}
}

// Motion still running long after the last input never settles.
func TestMotionNeverSettles(t *testing.T) {
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Animate("slow", ui.Animation{Duration: 5 * time.Second, Easing: ease.Linear},
			func(p float32) ui.Element {
				return ui.Div().Child(ui.Div().Key("crawler").Size(ui.Px(40), ui.Px(40)).Background(red).MarginLeft(100 * p))
			})
	})
	rec := u.Record()
	u.Advance(500 * time.Millisecond)
	if fs := rec.Findings(); len(fs) > 0 {
		t.Errorf("findings half a second in: %v", fs)
	}
	u.Advance(time.Second)
	if got := kinds(rec.Findings()); got != "never-settles crawler" {
		t.Errorf("findings %q, want never-settles naming the crawler", got)
	}
}

// checkTB records what Check reports.
type checkTB struct {
	testing.TB
	errors, logs []string
}

func (f *checkTB) Helper() {}
func (f *checkTB) Errorf(format string, a ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, a...))
}
func (f *checkTB) Logf(format string, a ...any) { f.logs = append(f.logs, fmt.Sprintf(format, a...)) }

// Check fails on findings no rule allows, on rules with no reason, and on rules
// that allow nothing; it logs what its rules allow.
func TestMotionCheck(t *testing.T) {
	on := ui.NewSignal(false)
	u := shown(t, on, func() *ui.DivElement { return box("panel") })
	rec := u.Record()
	u.Click("toggle")
	u.Settle()

	var tb checkTB
	rec.Check(&tb)
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "pop-in panel") {
		t.Errorf("Check with no rules reported %q, want the pop-in", tb.errors)
	}

	tb = checkTB{}
	rec.Check(&tb, uitest.Allow{Kind: uitest.PopIn, Element: "pan", Reason: "the panel is a test fixture"})
	if len(tb.errors) != 0 || len(tb.logs) != 1 {
		t.Errorf("Check with a rule: errors %q, logs %q; want no error and one log", tb.errors, tb.logs)
	}

	tb = checkTB{}
	rec.Check(&tb,
		uitest.Allow{Kind: uitest.PopIn, Element: "panel"},
		uitest.Allow{Kind: uitest.Jump, Element: "panel", Reason: "stale"})
	want := []string{"needs a kind and a reason", "1 motion finding", "matched no finding"}
	if len(tb.errors) != len(want) {
		t.Fatalf("Check reported %q, want %d errors", tb.errors, len(want))
	}
	for i, w := range want {
		if !strings.Contains(tb.errors[i], w) {
			t.Errorf("error %d is %q, want it to say %q", i, tb.errors[i], w)
		}
	}
}

// A recording has the frames run while it records, each with whether it
// followed an input, and none after Stop.
func TestMotionFrames(t *testing.T) {
	on := ui.NewSignal(false)
	u := shown(t, on, func() *ui.DivElement { return box("panel") })
	rec := u.Record()
	u.Click("toggle")
	u.Advance(32 * time.Millisecond)
	rec.Stop()
	u.Advance(32 * time.Millisecond)
	fs := rec.Frames()
	if len(fs) != 4 {
		t.Fatalf("%d frames, want 4: the first, the click's and two of Advance", len(fs))
	}
	if fs[0].Input || !fs[1].Input || fs[2].Input {
		t.Errorf("inputs %v %v %v, want only the click's frame", fs[0].Input, fs[1].Input, fs[2].Input)
	}
	var panel *uitest.Sample
	for i, s := range fs[3].Elements {
		if s.Key == "panel" {
			panel = &fs[3].Elements[i]
		}
	}
	if panel == nil || panel.Name != "panel" || panel.Opacity != 1 || panel.Bounds.W != 100 || panel.Background != red {
		t.Errorf("the panel in the last frame is %+v", panel)
	}
}
