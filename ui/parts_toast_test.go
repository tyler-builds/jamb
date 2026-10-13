package ui

import (
	"testing"
	"time"
)

func testToastParts(t *testing.T, applied map[string]bool) {
	type toastCase struct {
		part   string
		set    func(*ToastStyle, Style)
		action bool
		text   bool
	}
	cases := []toastCase{
		{part: "Toast.Container", set: func(s *ToastStyle, m Style) { s.Container = m }},
		{part: "Toast.Icon", set: func(s *ToastStyle, m Style) { s.Icon = m }},
		{part: "Toast.Text", set: func(s *ToastStyle, m Style) { s.Text = m }},
		{part: "Toast.Action", set: func(s *ToastStyle, m Style) { s.Action = m }, action: true},
		{part: "Toast.Close", set: func(s *ToastStyle, m Style) { s.Close = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st ToastStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return Div().Column().Child(Toaster()) })
			toast := r.rt.toast(ToastInfo, "Saved").Styles(st)
			if c.action {
				toast.Action("Undo", func() {})
			}
			r.frame()
			n := partNode(t, r.rt, c.part, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

// testTooltipParts checks the tooltip's panel and its text, which the runtime
// builds from the theme: the style is set on the theme and the tip is shown by
// a hover, after its delay.
func testTooltipParts(t *testing.T, applied map[string]bool) {
	cases := []struct {
		part string
		set  func(*TooltipStyle, Style)
		text bool
	}{
		{part: "Tooltip.Panel", set: func(s *TooltipStyle, m Style) { s.Panel = m }},
		{part: "Tooltip.Text", set: func(s *TooltipStyle, m Style) { s.Text = m }, text: true},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			theme := *DarkTheme()
			c.set(&theme.Tooltip, Style{Background: markBG, TextColor: markFG})
			r := newThemedRig(&theme, func(ctx *Context) Element {
				return Div().Column().Child(Div().Key("t").Size(Px(100), Px(30)).Tooltip("Hello"))
			})
			rc := r.rt.nodeRect(keyNode(t, r.rt, "t"))
			r.rt.PointerMove(rc.X+rc.W/2, rc.Y+rc.H/2, 0)
			r.now = r.now.Add(600 * time.Millisecond)
			r.frame()
			n := partNode(t, r.rt, c.part, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}

// TestToastUnderHighContrast checks that a toast's border is the text color under
// high contrast, so the box is outlined, and that an app color does not reach it.
func TestToastUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Toast = ToastStyle{Container: Style{Background: markBG}}
	r := newThemedRig(&theme, func(ctx *Context) Element { return Div().Column().Child(Toaster()) })
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.rt.toast(ToastError, "Failed")
	r.frame()
	th := r.rt.theme
	n := partNode(t, r.rt, "Toast.Container", "")
	if n.style.Background == markBG {
		t.Errorf("high contrast: an app color reached the toast")
	}
	if n.style.BorderColor != th.Text || n.style.BorderWidth != 2 {
		t.Errorf("high contrast: the toast border is %v %v, want 2 px of %v", n.style.BorderWidth, n.style.BorderColor, th.Text)
	}
}
