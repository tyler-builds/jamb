package ui

import (
	"testing"
)

func testBadgeParts(t *testing.T, applied map[string]bool) {
	type badgeCase struct {
		part  string
		set   func(*BadgeStyle, Style)
		build func() *BadgeElement
		drawn string
		text  bool
	}
	cases := []badgeCase{
		{part: "Badge.Badge", set: func(s *BadgeStyle, m Style) { s.Badge = m }, build: func() *BadgeElement { return Badge(5) }, drawn: "Badge.Badge"},
		{part: "Badge.Label", set: func(s *BadgeStyle, m Style) { s.Label = m }, build: func() *BadgeElement { return Badge(5) }, drawn: "Badge.Label", text: true},
		{part: "Badge.Dot", set: func(s *BadgeStyle, m Style) { s.Dot = m }, build: func() *BadgeElement { return BadgeText("New").StatusDot() }, drawn: "Badge.Dot"},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st BadgeStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element { return c.build().Styles(st) })
			n := partNode(t, r.rt, c.drawn, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
	// A dot that is the whole badge is drawn by the Dot part too.
	applied["Badge.Dot"] = true
	t.Run("Badge.Dot alone", func(t *testing.T) {
		var st BadgeStyle
		st.Dot = Style{Background: markBG}
		r := newAccessRig(func(ctx *Context) Element { return BadgeDot().Styles(st) })
		wantBackground(t, partNode(t, r.rt, "Badge.Dot", ""), "Badge.Dot", markBG)
	})
}

// TestBadgeVariantStyles checks the theme's Badge, which applies to every
// variant, and its Badges, which applies to one variant with the instance over
// both.
func TestBadgeVariantStyles(t *testing.T) {
	all, success, inst := Hex("#111213"), Hex("#121314"), Hex("#141516")
	bg := func(th *Theme, v BadgeVariant, st BadgeStyle) Color {
		r := newThemedRig(th, func(ctx *Context) Element { return Badge(3).Variant(v).Styles(st) })
		return partNode(t, r.rt, "Badge.Badge", "").style.Background
	}
	th := *DarkTheme()
	th.Badge.Badge.Background = all
	th.Badges.Success.Badge.Background = success
	if got := bg(&th, BadgeNeutral, BadgeStyle{}); got != all {
		t.Errorf("neutral: background %v, want the theme's Badge %v", got, all)
	}
	if got := bg(&th, BadgeSuccess, BadgeStyle{}); got != success {
		t.Errorf("success: background %v, want the variant's %v", got, success)
	}
	if got := bg(&th, BadgeSuccess, BadgeStyle{Badge: Style{Background: inst}}); got != inst {
		t.Errorf("success with an instance: background %v, want %v", got, inst)
	}
}

// TestBadgeUnderHighContrast checks that an app color does not reach a badge
// under high contrast: it is the window color with an outline.
func TestBadgeUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Badge.Badge = Style{Background: markBG}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Badge(4).Variant(BadgeDanger).Styles(BadgeStyle{Badge: Style{Background: markBG}})
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	n := partNode(t, r.rt, "Badge.Badge", "")
	if n.style.Background == markBG {
		t.Errorf("high contrast: an app color reached the badge")
	}
	if n.style.Background != th.Surface {
		t.Errorf("high contrast: the badge is %v, want the window color %v", n.style.Background, th.Surface)
	}
}
