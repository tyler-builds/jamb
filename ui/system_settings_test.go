package ui

import (
	"math"
	"testing"
	"time"
)

// The high contrast theme takes its colors from the system colors, and the
// runtime switches to it and back with the setting.
func TestHighContrastThemeFromSystemColors(t *testing.T) {
	c := HighContrastBlack()
	th := HighContrastTheme(c, DarkTheme())
	checks := []struct {
		name      string
		got, want Color
	}{
		{"Background", th.Background, c.Window},
		{"Surface", th.Surface, c.Window},
		{"Text", th.Text, c.WindowText},
		{"TextMuted", th.TextMuted, c.WindowText},
		{"TextSubtle", th.TextSubtle, c.WindowText},
		{"Accent", th.Accent, c.Highlight},
		{"AccentText", th.AccentText, c.HighlightText},
		{"SurfaceActive", th.SurfaceActive, c.Highlight},
		{"FocusRing", th.FocusRing, c.Highlight},
		{"Border", th.Border, c.WindowText},
		{"Scrollbar", th.Scrollbar, c.WindowText},
	}
	for _, ch := range checks {
		if ch.got != ch.want {
			t.Errorf("%s = %v, want the system color %v", ch.name, ch.got, ch.want)
		}
	}
	if !th.Dark || th.Name != "high contrast" {
		t.Errorf("a black window is a dark high contrast theme (dark %v, name %q)", th.Dark, th.Name)
	}
	if th.FontSize != DarkTheme().FontSize || th.Radius != DarkTheme().Radius {
		t.Error("the high contrast theme does not keep the app's font size and radius")
	}
	if HighContrastTheme(HighContrastWhite(), LightTheme()).Dark {
		t.Error("a white window is a light high contrast theme")
	}
}

func TestSystemSettingsSwitchTheme(t *testing.T) {
	var hc bool
	r := newOverlayRig(func(ctx *Context) Element {
		hc = ctx.HighContrast()
		return Div().Child(Text("x"))
	}, 300, 200)
	if r.rt.Theme().Name != "Dark" {
		t.Fatal("the app's theme is not the default")
	}
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastWhite()})
	r.frame()
	if r.rt.Theme().Name != "high contrast" || r.rt.Theme().Dark {
		t.Fatalf("high contrast on: theme %q, dark %v", r.rt.Theme().Name, r.rt.Theme().Dark)
	}
	if !hc || !r.rt.HighContrast() {
		t.Error("the component does not see high contrast")
	}
	// A theme the app sets stays in place under high contrast, and comes back
	// when high contrast is off.
	light := LightTheme()
	r.rt.SetTheme(light)
	if r.rt.Theme().Name != "high contrast" {
		t.Error("SetTheme replaced the high contrast theme")
	}
	r.rt.SetSystemSettings(SystemSettings{})
	r.frame()
	if r.rt.Theme() != light || hc {
		t.Errorf("high contrast off: theme %q, the component sees %v", r.rt.Theme().Name, hc)
	}
}

// A zero Colors under high contrast uses the representative black scheme.
func TestHighContrastWithoutColorsUsesBlack(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element { return Div() }, 200, 100)
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true})
	if got := r.rt.Theme().Background; got != HighContrastBlack().Window {
		t.Errorf("background %v, want the black scheme's window color", got)
	}
}

// High contrast draws solid colors: a blurred shadow and a gradient are not
// painted, while a crisp ring (a focus ring) is.
func TestHighContrastDropsShadowsAndGradients(t *testing.T) {
	grad := LinearGradient(135, Hex("#ff0000"), Hex("#0000ff"))
	build := func(hc bool) *Runtime {
		r := newOverlayRig(func(ctx *Context) Element {
			return Div().Padding(20).Child(
				Div().Key("fancy").Size(Px(80), Px(40)).Background(Hex("#222222")).BackgroundGradient(grad).
					Shadow(ShadowLg).
					Child(Div().Key("ring").Size(Px(20), Px(20)).
						Style(Style{Shadows: []Shadow{{Spread: 2, Color: Hex("#00ff00")}}})))
		}, 300, 200)
		r.rt.SetSystemSettings(SystemSettings{HighContrast: hc, Colors: HighContrastBlack()})
		r.frame()
		return r.rt
	}
	countGradients := func(rt *Runtime) (grads, blurred int) {
		for _, q := range rt.scene.Quads {
			if q.Gradient {
				grads++
			}
			if q.Blur > 0 {
				blurred++
			}
		}
		return grads, blurred
	}
	if g, b := countGradients(build(false)); g == 0 || b == 0 {
		t.Fatalf("without high contrast the gradient and the shadow are drawn (gradients %d, blurred %d)", g, b)
	}
	rt := build(true)
	if g, b := countGradients(rt); g != 0 || b != 0 {
		t.Errorf("high contrast draws a gradient (%d) or a blurred shadow (%d)", g, b)
	}
	if _, ok := rt.Find("ring"); !ok {
		t.Fatal("the ring element is missing")
	}
	found := false
	for _, q := range rt.scene.Quads {
		if q.BorderWidth > 0 && q.BorderColor == Hex("#00ff00").scene() {
			found = true
		}
	}
	if !found {
		t.Error("high contrast dropped the crisp ring")
	}
}

// The Text size setting scales the font, and with it the text's box, and not
// the zoom or the layout around it.
func TestTextScaleScalesTextOnly(t *testing.T) {
	heightOf := func(scale int, zoom float32) float32 {
		r := newOverlayRig(func(ctx *Context) Element {
			return Div().Padding(10).Child(Text("Scaled text").Key("t"))
		}, 400, 300)
		r.rt.SetZoom(zoom)
		r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
		r.frame()
		return r.find(t, "t").H
	}
	base := heightOf(100, 1)
	if got := heightOf(225, 1) / base; math.Abs(float64(got)-2.25) > 0.01 {
		t.Errorf("text height at 225%% is %.3f times the height at 100%%, want 2.25", got)
	}
	// The zoom scales the whole UI in logical pixels: the text's logical height
	// is the same at any zoom, for the same text scale.
	if got, want := heightOf(150, 1.5), heightOf(150, 1); math.Abs(float64(got-want)) > 0.01 {
		t.Errorf("text height at 150%% text and 150%% zoom is %.3f, want %.3f (the zoom must not change it)", got, want)
	}
}

// With reduced motion a transition jumps to its target, in one frame.
func TestReducedMotionJumpsTransitions(t *testing.T) {
	hot := NewSignal(false)
	build := func(reduced bool) *Runtime {
		r := newOverlayRig(func(ctx *Context) Element {
			bg := Hex("#111111")
			if hot.Get() {
				bg = Hex("#eeeeee")
			}
			return Div().Key("box").Size(Px(40), Px(40)).Background(bg).Transition(400 * time.Millisecond)
		}, 200, 100)
		r.rt.SetSystemSettings(SystemSettings{ReducedMotion: reduced})
		r.frame()
		hot.Set(true)
		r.now = r.now.Add(16 * time.Millisecond)
		r.frame()
		return r.rt
	}
	shown := func(rt *Runtime) Color { return rt.findKey(rt.tree, "box").st.shown.bg }
	if got := shown(build(false)); got == Hex("#eeeeee") {
		t.Fatal("without reduced motion the transition is not running after one frame")
	}
	if got := shown(build(true)); got != Hex("#eeeeee") {
		t.Errorf("with reduced motion the transition shows %v after one frame, want the target", got)
	}
	hot.Set(false)
}

// A finite Animate jumps to its end, and a repeating one holds its first frame.
// Neither asks for a frame.
func TestReducedMotionAnimate(t *testing.T) {
	var finite, repeat []float32
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().
			Child(Animate("finite", Animation{Duration: time.Second, Easing: linear}, func(p float32) Element {
				finite = append(finite, p)
				return Div()
			})).
			Child(Animate("repeat", Animation{Duration: time.Second, Easing: linear, Repeat: true}, func(p float32) Element {
				repeat = append(repeat, p)
				return Div()
			}))
	}, 200, 100)
	r.rt.SetSystemSettings(SystemSettings{ReducedMotion: true})
	r.now = r.now.Add(300 * time.Millisecond)
	r.frame()
	if len(finite) == 0 || finite[len(finite)-1] != 1 {
		t.Errorf("a finite animation rendered %v, want its end (1)", finite)
	}
	if len(repeat) == 0 || repeat[len(repeat)-1] != 0 {
		t.Errorf("a repeating animation rendered %v, want its first frame (0)", repeat)
	}
	if r.rt.NeedsFrame() {
		t.Error("reduced motion left an animation asking for frames")
	}
}

// Enter and exit motions do not run with reduced motion: an element that
// appears is at its place at once, and nothing is left to animate.
func TestReducedMotionSkipsEnterAndExit(t *testing.T) {
	show := NewSignal(false)
	r := newOverlayRig(func(ctx *Context) Element {
		if !show.Get() {
			return Div()
		}
		return Div().Key("panel").Size(Px(40), Px(40)).
			Enter(Fade(), Animation{Duration: 300 * time.Millisecond}).
			Exit(Fade(), Animation{Duration: 300 * time.Millisecond})
	}, 200, 100)
	r.rt.SetSystemSettings(SystemSettings{ReducedMotion: true})
	show.Set(true)
	r.frame()
	if r.rt.NeedsFrame() {
		t.Error("an entering element asks for frames under reduced motion")
	}
	show.Set(false)
	r.frame()
	if len(r.rt.ghosts) != 0 {
		t.Error("an exiting element is kept on screen under reduced motion")
	}
}

// Scrolling jumps to its target under reduced motion.
func TestReducedMotionScrollJumps(t *testing.T) {
	r := newTestRuntime(scrollList(20), 300, 400, 1)
	r.SetSystemSettings(SystemSettings{ReducedMotion: true})
	r.Scroll(150, 100, 0, 300, false, 0)
	frame(r)
	if n := r.findKey(r.tree, "list"); n == nil || n.st.scrollY != n.st.targetY || n.st.targetY == 0 {
		t.Errorf("the scroll is at %v, want its target", n.st.scrollY)
	}
	if r.NeedsFrame() {
		t.Error("a scroll under reduced motion asks for frames")
	}
}

// The caret blinks at the system's blink time, or stays solid when the system's
// blink is off; its width is the system's width times the default 1.5 px.
func TestCaretBlinkAndWidthSettings(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element { return Div() }, 200, 100)
	es := &editState{activity: r.now}
	at := func(d time.Duration) { r.rt.lastFrame = r.now.Add(d) }

	r.rt.SetSystemSettings(SystemSettings{CaretBlink: 200 * time.Millisecond})
	at(100 * time.Millisecond)
	if !r.rt.caretVisible(es) {
		t.Error("the caret is off in the first half of its blink")
	}
	at(300 * time.Millisecond)
	if r.rt.caretVisible(es) {
		t.Error("the caret is on in the second half of a 200 ms blink")
	}

	r.rt.SetSystemSettings(SystemSettings{CaretBlinkOff: true, CaretBlink: 200 * time.Millisecond})
	at(300 * time.Millisecond)
	if !r.rt.caretVisible(es) {
		t.Error("the caret blinks although the system's blink is off")
	}

	r.rt.SetSystemSettings(SystemSettings{})
	if r.rt.caretWidth != 1.5 {
		t.Errorf("the default caret is %v px wide, want 1.5", r.rt.caretWidth)
	}
	r.rt.SetSystemSettings(SystemSettings{CaretWidth: 2})
	if r.rt.caretWidth != 3 {
		t.Errorf("a caret width of 2 gives %v px, want 3", r.rt.caretWidth)
	}
	if got := r.rt.SystemSettings(); got.CaretBlink != 530*time.Millisecond || got.TextScale != 100 || got.CaretWidth != 2 {
		t.Errorf("normalized settings %+v", got)
	}
}

// Text scale is clamped to 100 to 225 percent.
func TestTextScaleClamped(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element { return Div() }, 200, 100)
	for in, want := range map[int]int{0: 100, 80: 100, 150: 150, 300: 225} {
		r.rt.SetSystemSettings(SystemSettings{TextScale: in})
		if got := r.rt.TextScale(); got != want {
			t.Errorf("TextScale %d gives %d, want %d", in, got, want)
		}
	}
}

// The Text size setting is independent of the display's scale: at 150% device
// scale the text is still 2.25 times its size at 225%, in logical pixels.
func TestTextScaleIndependentOfDPI(t *testing.T) {
	heightOf := func(scale int, dpi float32) float32 {
		r := newOverlayRig(func(ctx *Context) Element {
			return Div().Padding(10).Child(Text("DPI").Key("t"))
		}, 400, 300)
		r.rt.SetViewport(400*dpi, 300*dpi, dpi)
		r.rt.SetSystemSettings(SystemSettings{TextScale: scale})
		r.frame()
		return r.find(t, "t").H
	}
	base := heightOf(100, 1.5)
	if got := heightOf(225, 1.5) / base; math.Abs(float64(got)-2.25) > 0.01 {
		t.Errorf("text at 225%% and 150%% DPI is %.3f times its 100%% size, want 2.25", got)
	}
	if got, want := heightOf(100, 1.5), heightOf(100, 1); math.Abs(float64(got-want)) > 0.01 {
		t.Errorf("DPI changes the logical text height: %.3f at 150%% DPI, %.3f at 100%%", got, want)
	}
}

// Under high contrast disabled controls are drawn in GrayText, not dimmed, and
// that GrayText still reads on the window in both schemes. A disabled control
// is exempt from the ratio (WCAG 1.4.3), but the text is kept readable.
func TestHighContrastDisabledIsGrayText(t *testing.T) {
	for _, c := range []SystemColors{HighContrastBlack(), HighContrastWhite()} {
		th := HighContrastTheme(c, DarkTheme())
		if th.Disabled != c.GrayText {
			t.Errorf("disabled color %v, want GrayText %v", th.Disabled, c.GrayText)
		}
		if r := contrastRatio(th.Disabled, th.Surface); r < 4.5 {
			t.Errorf("GrayText on the window is %.2f:1, want 4.5 or more", r)
		}
	}
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Child(Button("Save").Key("save").Disabled(true))
	}, 200, 100)
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	n := r.rt.findKey(r.rt.tree, "save")
	if n == nil || n.style.Opacity != 0 && n.style.Opacity != 1 {
		t.Fatal("the disabled button is not found")
	}
	if n.style.Opacity == 0.5 {
		t.Error("a disabled button is dimmed with opacity under high contrast")
	}
}
