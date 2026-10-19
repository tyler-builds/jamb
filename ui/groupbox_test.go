package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// TestGroupBoxGroup checks that a group box is a group named by its title and
// described by its description, with its children inside it, in order.
func TestGroupBoxGroup(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return GroupBox("Notifications").Description("How you hear about changes.").
			Child(Checkbox("Email", NewSignal(false))).
			Children(Checkbox("Desktop", NewSignal(true)), nil)
	})
	tr := r.tree()
	n := accesstest.Find(t, tr, access.RoleGroup, "Notifications")
	if n.Description != "How you hear about changes." {
		t.Errorf("description %q, want the group box's description", n.Description)
	}
	var boxes []string
	for _, id := range n.Children {
		if c := tr.Node(id); c != nil && c.Role == access.RoleCheckBox {
			boxes = append(boxes, c.Name)
		}
	}
	if len(boxes) != 2 || boxes[0] != "Email" || boxes[1] != "Desktop" {
		t.Errorf("the group holds the check boxes %v, want [Email Desktop]", boxes)
	}
}

// TestGroupBoxWithoutHeader checks that a group box with neither a title nor a
// description draws no header.
func TestGroupBoxWithoutHeader(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return GroupBox("").Child(Text("Body")) })
	if n := partNodes(r.rt, "GroupBox.Header"); len(n) != 0 {
		t.Errorf("%d headers drawn, want none", len(n))
	}
	partNode(t, r.rt, "GroupBox.Content", "")
}

// TestGroupBoxFill checks that the default box is outlined and has no fill, and
// that Fill fills it without a border.
func TestGroupBoxFill(t *testing.T) {
	for _, fill := range []bool{false, true} {
		r := newAccessRig(func(ctx *Context) Element {
			g := GroupBox("Sample")
			if fill {
				g.Fill()
			}
			return g
		})
		th := r.rt.theme
		n := partNode(t, r.rt, "GroupBox.GroupBox", "")
		switch {
		case !fill && (n.style.BorderWidth != 1 || !n.style.Background.IsZero()):
			t.Errorf("outlined: border %v and background %v, want 1 and none", n.style.BorderWidth, n.style.Background)
		case fill && (n.style.BorderWidth != 0 || n.style.Background != th.SurfaceHover):
			t.Errorf("filled: border %v and background %v, want 0 and %v", n.style.BorderWidth, n.style.Background, th.SurfaceHover)
		}
	}
}

// TestGroupBoxUnderHighContrast checks that an app color does not reach a group
// box under high contrast, and that a filled box is outlined there too.
func TestGroupBoxUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.GroupBox.GroupBox = Style{Background: markBG}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return GroupBox("Sample").Fill().Styles(GroupBoxStyle{GroupBox: Style{Background: markBG}})
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	n := partNode(t, r.rt, "GroupBox.GroupBox", "")
	if n.style.Background != th.Surface {
		t.Errorf("high contrast: the box is %v, want the window color %v", n.style.Background, th.Surface)
	}
	if n.style.BorderWidth != 1 || n.style.BorderColor != th.Text {
		t.Errorf("high contrast: border %v in %v, want 1 in the text color %v", n.style.BorderWidth, n.style.BorderColor, th.Text)
	}
}

func testGroupBoxParts(t *testing.T, applied map[string]bool) {
	type groupCase struct {
		part string
		set  func(*GroupBoxStyle, Style)
		text bool
	}
	cases := []groupCase{
		{part: "GroupBox.GroupBox", set: func(s *GroupBoxStyle, m Style) { s.GroupBox = m }},
		{part: "GroupBox.Header", set: func(s *GroupBoxStyle, m Style) { s.Header = m }},
		{part: "GroupBox.Title", set: func(s *GroupBoxStyle, m Style) { s.Title = m }, text: true},
		{part: "GroupBox.Description", set: func(s *GroupBoxStyle, m Style) { s.Description = m }, text: true},
		{part: "GroupBox.Content", set: func(s *GroupBoxStyle, m Style) { s.Content = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st GroupBoxStyle
			c.set(&st, Style{Background: markBG, TextColor: markFG})
			r := newAccessRig(func(ctx *Context) Element {
				return GroupBox("Notifications").Description("How you hear about changes.").Child(Text("Body")).Styles(st)
			})
			n := partNode(t, r.rt, c.part, "")
			if c.text {
				wantTextColor(t, n, c.part, markFG)
				return
			}
			wantBackground(t, n, c.part, markBG)
		})
	}
}
