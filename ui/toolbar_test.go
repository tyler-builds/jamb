package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// toolbarRig is a toolbar of three buttons, a disabled one and a separator,
// between a button before it and a button after it.
func toolbarRig() *accessRig {
	return newAccessRig(func(ctx *Context) Element {
		return Div().Column().Gap(8).Padding(10).
			Child(Button("Before").Key("before")).
			Child(Toolbar("Formatting").
				Child(Button("A").Key("a")).
				Child(Button("B").Key("b")).
				Child(ToolbarSeparator()).
				Child(Button("Off").Key("off").Disabled(true)).
				Child(Button("C").Key("c"))).
			Child(Button("After").Key("after"))
	})
}

// wantFocus fails unless the element with key has focus.
func wantFocus(t *testing.T, r *accessRig, key, after string) {
	t.Helper()
	if got := r.focusedKey(); got != key {
		t.Fatalf("after %s focus is on %q, want %q", after, got, key)
	}
}

// TestToolbarIsOneTabStop checks that Tab passes over a toolbar in one stop, at
// its first control, and comes back to the control that had focus last.
func TestToolbarIsOneTabStop(t *testing.T) {
	r := toolbarRig()
	r.focusKey(t, "before")
	r.press(input.KeyTab, 0)
	wantFocus(t, r, "a", "Tab into the toolbar")
	r.press(input.KeyTab, 0)
	wantFocus(t, r, "after", "Tab out of the toolbar")
	r.press(input.KeyTab, input.ModShift)
	wantFocus(t, r, "a", "Shift+Tab back")

	r.press(input.KeyRight, 0)
	wantFocus(t, r, "b", "Right")
	r.press(input.KeyTab, 0)
	wantFocus(t, r, "after", "Tab out from B")
	r.press(input.KeyTab, input.ModShift)
	wantFocus(t, r, "b", "Shift+Tab back to the control that had focus last")
	r.press(input.KeyTab, input.ModShift)
	wantFocus(t, r, "before", "Shift+Tab out of the toolbar")
}

// TestToolbarArrowKeys checks that Left and Right move between the controls,
// skipping the separator and the disabled button and wrapping at the ends, and
// that Home and End go to the first and the last.
func TestToolbarArrowKeys(t *testing.T) {
	r := toolbarRig()
	r.focusKey(t, "a")
	for _, step := range []struct {
		k    input.Key
		want string
	}{
		{input.KeyRight, "b"}, {input.KeyRight, "c"}, {input.KeyRight, "a"},
		{input.KeyLeft, "c"}, {input.KeyHome, "a"}, {input.KeyEnd, "c"}, {input.KeyLeft, "b"},
	} {
		r.press(step.k, 0)
		wantFocus(t, r, step.want, keyLabel(step.k))
	}
	// A modifier leaves the key alone.
	r.press(input.KeyRight, input.ModCtrl)
	wantFocus(t, r, "b", "Ctrl+Right")
}

// TestToolbarRemembersAClick checks that a control focused with the pointer is
// where Tab enters the toolbar next.
func TestToolbarRemembersAClick(t *testing.T) {
	r := toolbarRig()
	r.clickKey(t, "c")
	r.focusKey(t, "after")
	r.press(input.KeyTab, input.ModShift)
	wantFocus(t, r, "c", "Shift+Tab after a click on C")
}

// TestToolbarTextFieldKeepsItsKeys checks that a text field in a toolbar keeps
// Left, Right, Home and End for its caret, as the WAI-ARIA pattern says.
func TestToolbarTextFieldKeepsItsKeys(t *testing.T) {
	q := NewSignal("ab")
	r := newAccessRig(func(ctx *Context) Element {
		return Toolbar("Search").
			Child(Button("Bold").Key("bold")).
			Child(TextInput(q).Label("Find").Key("find")).
			Child(Button("Go").Key("go"))
	})
	r.focusKey(t, "find")
	for _, k := range []input.Key{input.KeyLeft, input.KeyHome, input.KeyRight, input.KeyEnd, input.KeyRight} {
		r.press(k, 0)
		wantFocus(t, r, "find", keyLabel(k)+" in the text field")
	}
	r.focusKey(t, "bold")
	r.press(input.KeyRight, 0)
	wantFocus(t, r, "find", "Right from Bold")
}

// TestToolbarAccess checks that a toolbar is a tool bar named by its label, with
// its controls and a separator inside it.
func TestToolbarAccess(t *testing.T) {
	r := toolbarRig()
	tr := r.tree()
	bar := accesstest.Find(t, tr, access.RoleToolBar, "Formatting")
	var roles []access.Role
	for _, id := range bar.Children {
		roles = append(roles, tr.Node(id).Role)
	}
	want := []access.Role{access.RoleButton, access.RoleButton, access.RoleSeparator, access.RoleButton, access.RoleButton}
	if len(roles) != len(want) {
		t.Fatalf("the toolbar holds %v, want %v", roles, want)
	}
	for i := range want {
		if roles[i] != want[i] {
			t.Fatalf("the toolbar holds %v, want %v", roles, want)
		}
	}
}

// TestToolbarUnderHighContrast checks that an app color does not reach a toolbar
// under high contrast, and that its outline and separators are the text color.
func TestToolbarUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Toolbar.Bar = Style{BorderColor: markBG}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Toolbar("Tools").Child(Button("A")).Child(ToolbarSeparator()).Child(Button("B")).
			Styles(ToolbarStyle{Separator: Style{Background: markBG}})
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	if c := partNode(t, r.rt, "Toolbar.Bar", "").style.BorderColor; c != th.Text {
		t.Errorf("high contrast: the outline is %v, want the text color %v", c, th.Text)
	}
	if c := partNode(t, r.rt, "Toolbar.Separator", "").style.Background; c != th.Text {
		t.Errorf("high contrast: the separator is %v, want the text color %v", c, th.Text)
	}
}

func testToolbarParts(t *testing.T, applied map[string]bool) {
	type barCase struct {
		part string
		set  func(*ToolbarStyle, Style)
	}
	cases := []barCase{
		{part: "Toolbar.Bar", set: func(s *ToolbarStyle, m Style) { s.Bar = m }},
		{part: "Toolbar.Separator", set: func(s *ToolbarStyle, m Style) { s.Separator = m }},
	}
	for _, c := range cases {
		applied[c.part] = true
		t.Run(c.part, func(t *testing.T) {
			var st ToolbarStyle
			c.set(&st, Style{Background: markBG})
			r := newAccessRig(func(ctx *Context) Element {
				return Toolbar("Tools").Child(Button("A")).Child(ToolbarSeparator()).Child(Button("B")).Styles(st)
			})
			wantBackground(t, partNode(t, r.rt, c.part, ""), c.part, markBG)
		})
	}
}
