package ui

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
)

// TestKbdIsOneStaticText checks that a shortcut is one static text named as a
// menu names the same keys, with no node for its caps.
func TestKbdIsOneStaticText(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return Kbd("ctrl+shift+p") })
	n := accesstest.Find(t, r.tree(), access.RoleStaticText, "Ctrl+Shift+P")
	if len(n.Children) != 0 {
		t.Errorf("the shortcut has children %v, want none", accesstest.Children(r.tree(), n))
	}
	accesstest.Absent(t, r.tree(), access.RoleStaticText, "Ctrl")
}

// TestKbdMatchesKeysFor checks that the shortcut's name is the text that KeysFor
// gives a menu for the same keys, for a sequence too.
func TestKbdMatchesKeysFor(t *testing.T) {
	act := NewAction("test.kbd", "Kbd")
	for _, keys := range []string{"ctrl+shift+p", "ctrl+k ctrl+t", "alt+up", "f2", "ctrl+-", "shift+alt+ctrl+super+pagedown"} {
		r := newAccessRig(func(ctx *Context) Element { return Kbd(keys) })
		r.rt.AddBindings(Bind(keys, act))
		want := r.rt.KeysFor(act)
		accesstest.Find(t, r.tree(), access.RoleStaticText, want)
	}
}

// TestKbdDrawsACapPerKey checks that each key of each stroke is a cap of its own,
// with the key's name in it, in order.
func TestKbdDrawsACapPerKey(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element { return Kbd("ctrl+k ctrl+t") })
	caps := partNodes(r.rt, "Kbd.Key")
	if len(caps) != 4 {
		t.Fatalf("%d key caps, want 4", len(caps))
	}
	// The caps are hidden from assistive technology, so their text is walked here.
	var got []string
	var walk func(n *node)
	walk = func(n *node) {
		if n.kind == kindText {
			got = append(got, n.textEl.text)
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	walk(partNode(t, r.rt, "Kbd.Kbd", ""))
	if s := strings.Join(got, " "); s != "Ctrl K Ctrl T" {
		t.Errorf("the caps read %q, want %q", s, "Ctrl K Ctrl T")
	}
}

// TestKbdPanicsOnMalformedKeys checks that a malformed keys string panics with a
// message that names the call, as Bind does.
func TestKbdPanicsOnMalformedKeys(t *testing.T) {
	defer func() {
		msg, _ := recover().(string)
		if !strings.HasPrefix(msg, `ui.Kbd("ctrl+nope"): unknown key "nope"`) {
			t.Errorf("panic %q, want one that names ui.Kbd and the key", msg)
		}
	}()
	Kbd("ctrl+nope")
}

// TestKbdUnderHighContrast checks that an app color does not reach a key cap
// under high contrast: it is outlined in the text color.
func TestKbdUnderHighContrast(t *testing.T) {
	theme := *DarkTheme()
	theme.Kbd.Key = Style{Background: markBG}
	r := newThemedRig(&theme, func(ctx *Context) Element {
		return Kbd("f2").Styles(KbdStyle{Key: Style{Background: markBG}})
	})
	r.rt.SetSystemSettings(SystemSettings{HighContrast: true, Colors: HighContrastBlack()})
	r.frame()
	th := r.rt.theme
	n := partNode(t, r.rt, "Kbd.Key", "")
	if n.style.Background == markBG {
		t.Errorf("high contrast: an app color reached the key cap")
	}
	if n.style.BorderColor != th.Text {
		t.Errorf("high contrast: the cap's border is %v, want the text color %v", n.style.BorderColor, th.Text)
	}
}

func testKbdParts(t *testing.T, applied map[string]bool) {
	applied["Kbd.Kbd"] = true
	t.Run("Kbd.Kbd", func(t *testing.T) {
		var st KbdStyle
		st.Kbd = Style{Background: markBG}
		r := newAccessRig(func(ctx *Context) Element { return Kbd("ctrl+p").Styles(st) })
		wantBackground(t, partNode(t, r.rt, "Kbd.Kbd", ""), "Kbd.Kbd", markBG)
	})
	applied["Kbd.Key"] = true
	t.Run("Kbd.Key", func(t *testing.T) {
		var st KbdStyle
		st.Key = Style{Background: markBG}
		r := newAccessRig(func(ctx *Context) Element { return Kbd("ctrl+p").Styles(st) })
		caps := partNodes(r.rt, "Kbd.Key")
		if len(caps) != 2 {
			t.Fatalf("%d key caps, want 2", len(caps))
		}
		for _, n := range caps {
			wantBackground(t, n, "Kbd.Key", markBG)
		}
	})
}
