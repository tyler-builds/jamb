package ui

import (
	"testing"
)

// reducedRig is a rig with reduced motion on, so that the ring's dots and the
// indeterminate segment have a fixed place to test.
func reducedRig(root func(*Context) Element) *accessRig {
	r := newAccessRig(root)
	r.rt.SetSystemSettings(SystemSettings{ReducedMotion: true})
	r.frame()
	return r
}

func testProgressParts(t *testing.T, applied map[string]bool) {
	type progCase struct {
		part  string
		set   func(*ProgressBarStyle, Style)
		drawn string
		text  bool
	}
	cases := []progCase{
		{part: "ProgressBar.Track", set: func(s *ProgressBarStyle, m Style) { s.Track = m }, drawn: "ProgressBar.Track"},
		{part: "ProgressBar.Fill", set: func(s *ProgressBarStyle, m Style) { s.Fill = m }, drawn: "ProgressBar.Fill"},
		{part: "ProgressBar.Label", set: func(s *ProgressBarStyle, m Style) { s.Label = m }, drawn: "ProgressBar.Label", text: true},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st ProgressBarStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := reducedRig(func(ctx *Context) Element {
				return Div().Column().Child(ProgressBar(0.4).Label("Upload").Styles(st))
			})
			n := partNode(t, r.rt, c.drawn, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
	// The indeterminate bar's segment is the Fill part, drawn in the middle with
	// reduced motion. The track is the same part as in the determinate bar.
	applied["IndeterminateProgressBar.Fill"] = true
	t.Run("IndeterminateProgressBar.Fill", func(t *testing.T) {
		var st ProgressBarStyle
		st.Fill = Style{Background: markBG}
		r := reducedRig(func(ctx *Context) Element {
			return Div().Column().Child(IndeterminateProgressBar().Label("Working").Styles(st))
		})
		wantBackground(t, partNode(t, r.rt, "ProgressBar.Fill", ""), "IndeterminateProgressBar.Fill", markBG)
	})
	applied["IndeterminateProgressBar.Track"] = true
	t.Run("IndeterminateProgressBar.Track", func(t *testing.T) {
		var st ProgressBarStyle
		st.Track = Style{Background: markBG}
		r := reducedRig(func(ctx *Context) Element {
			return Div().Column().Child(IndeterminateProgressBar().Label("Working").Styles(st))
		})
		wantBackground(t, partNode(t, r.rt, "ProgressBar.Track", ""), "IndeterminateProgressBar.Track", markBG)
	})

	applied["ProgressRing.Ring"] = true
	t.Run("ProgressRing.Ring", func(t *testing.T) {
		var st ProgressRingStyle
		st.Ring = Style{Background: markBG}
		r := reducedRig(func(ctx *Context) Element {
			return Div().Column().Child(ProgressRing().Label("Sending").Size(RingSmall).Styles(st))
		})
		wantBackground(t, partNode(t, r.rt, "ProgressRing.Ring", ""), "ProgressRing.Ring", markBG)
	})
	applied["ProgressRing.Dot"] = true
	t.Run("ProgressRing.Dot", func(t *testing.T) {
		var st ProgressRingStyle
		st.Dot = Style{Background: markBG}
		r := reducedRig(func(ctx *Context) Element {
			return Div().Column().Child(ProgressRing().Label("Sending").Styles(st))
		})
		dots := partNodes(r.rt, "ProgressRing.Dot")
		if len(dots) != ringDots {
			t.Fatalf("the ring draws %d dots, want %d", len(dots), ringDots)
		}
		for _, d := range dots {
			wantBackground(t, d, "ProgressRing.Dot", markBG)
		}
	})
}

// TestProgressUnderHighContrast checks the guardrails for the bar: the groove is
// the window color with an outline, the fill is Highlight, and an app color is
// not used for the fill.
func TestProgressUnderHighContrast(t *testing.T) {
	theme := *DarkTheme
	theme.ProgressBar = ProgressBarStyle{Fill: Style{Background: markBG}}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Div().Column().Child(ProgressBar(0.5).Label("Upload").Color(Hex("#123456")))
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	if n := partNode(t, r.rt, "ProgressBar.Fill", ""); n.style.Background != th.Accent {
		t.Errorf("high contrast: the fill is %v, want Highlight %v", n.style.Background, th.Accent)
	}
	if n := partNode(t, r.rt, "ProgressBar.Track", ""); n.style.Background != th.Surface {
		t.Errorf("high contrast: the track is %v, want the window color %v", n.style.Background, th.Surface)
	}
}
