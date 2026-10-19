package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// TestStatusBarAccess checks that a status bar is a status bar that holds its
// items, the start ones before the end ones, and that it has no name unless it
// is given one.
func TestStatusBarAccess(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(
			StatusBar().Start(Text("Projects"), nil).End(Text("6 items")).Key("status"))
	})
	tr := r.tree()
	var bar *access.Node
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleStatusBar {
			bar = n
		}
		return true
	})
	if bar == nil {
		t.Fatal("no status bar in the tree")
	}
	if bar.Name != "" {
		t.Errorf("name %q, want none", bar.Name)
	}
	var texts []string
	tr.Walk(func(n *access.Node) bool {
		for p := n; p != nil; p = tr.Node(p.Parent) {
			if p == bar && n.Role == access.RoleStaticText {
				texts = append(texts, n.Name)
				break
			}
		}
		return true
	})
	if len(texts) != 2 || texts[0] != "Projects" || texts[1] != "6 items" {
		t.Errorf("the status bar holds %v, want [Projects 6 items]", texts)
	}

	r2 := newAccessRig(func(ctx *Context) Element { return StatusBar().Label("Editor status").Start(Text("Ln 1")) })
	accesstest.Find(t, r2.tree(), access.RoleStatusBar, "Editor status")
}

// TestStatusBarLayout checks that the start items sit at the left, the end items
// at the right, and that the bar is compact and fills the width.
func TestStatusBarLayout(t *testing.T) {
	r := newOverlayRig(func(ctx *Context) Element {
		return Div().Column().Size(Full, Full).
			Child(Div().Grow(1)).
			Child(StatusBar().Start(Text("Left").Key("left")).End(Text("Right").Key("right")).Key("bar"))
	}, 600, 400)
	bar, left, right := r.find(t, "bar"), r.find(t, "left"), r.find(t, "right")
	if bar.W != 600 || bar.Y+bar.H != 400 {
		t.Errorf("bar %+v, want the full width at the bottom", bar)
	}
	if bar.H > 32 {
		t.Errorf("bar height %v, want a compact bar", bar.H)
	}
	if left.X > bar.X+20 || right.X+right.W < bar.X+bar.W-20 {
		t.Errorf("items at %v and %v, want the start at the left and the end at the right", left, right)
	}
}

// TestStatusBarLook checks the default look: a top border only, small muted text,
// and, under high contrast, the text color without the app's colors.
func TestStatusBarLook(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return StatusBar().Start(Text("Ready")) })
	th := r.rt.theme
	n := partNode(t, r.rt, "StatusBar.Bar", "")
	if n.style.BorderEdges != (Edges{Top: 1}) || n.style.TextColor != th.TextMuted || n.style.FontSize != 12 {
		t.Errorf("bar border %v, text %v at %v, want a top border and 12 px muted text",
			n.style.BorderEdges, n.style.TextColor, n.style.FontSize)
	}

	theme := *DarkTheme()
	theme.StatusBar.Bar = Style{Background: markBG}
	hc := newThemedRig(&theme, func(ctx *Context) Element {
		return StatusBar().Start(Text("Ready")).Styles(StatusBarStyle{Bar: Style{TextColor: markFG}})
	})
	hc.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	hc.frame()
	th = hc.rt.theme
	n = partNode(t, hc.rt, "StatusBar.Bar", "")
	if n.style.Background != th.Surface || n.style.TextColor != th.Text || n.style.BorderColor != th.Text {
		t.Errorf("high contrast: bar %v with text %v and border %v, want %v, %v and %v",
			n.style.Background, n.style.TextColor, n.style.BorderColor, th.Surface, th.Text, th.Text)
	}
}

func testStatusBarParts(t *testing.T, applied map[string]bool) {
	type barCase struct {
		part string
		set  func(*StatusBarStyle, Style)
	}
	cases := []barCase{
		{part: "StatusBar.Bar", set: func(s *StatusBarStyle, m Style) { s.Bar = m }},
		{part: "StatusBar.Start", set: func(s *StatusBarStyle, m Style) { s.Start = m }},
		{part: "StatusBar.End", set: func(s *StatusBarStyle, m Style) { s.End = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st StatusBarStyle
			c.set(&st, Style{Background: markBG})
			r := newAccessRig(func(ctx *Context) Element {
				return StatusBar().Start(Text("Ready")).End(Text("Ln 1")).Styles(st)
			})
			wantBackground(t, partNode(t, r.rt, c.part, ""), c.part, markBG)
		})
	}
}
