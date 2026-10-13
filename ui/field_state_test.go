package ui

import (
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/input"
)

// A read-only field can be focused, its text selected and copied, but typing,
// pasting, cutting, deleting, undo and assistive technology don't change it.
func TestTextFieldReadOnly(t *testing.T) {
	v := NewSignal("alpha beta")
	changed := 0
	r := newEditorRig(t, func(ctx *Context) Element {
		return TextInput(v).Label("Code").ReadOnly(true).OnChange(func(string) { changed++ })
	})
	r.click(50, 17)
	if n, _ := r.editor(); n == nil {
		t.Fatal("a click does not focus a read-only field")
	}
	r.typeText("x")
	r.key(input.KeyBackspace, 0)
	r.key(input.KeyA, ctrl)
	r.key(input.KeyC, ctrl)
	if r.rt.memClipboard != "alpha beta" {
		t.Fatalf("Ctrl+A, Ctrl+C copied %q, want the whole text", r.rt.memClipboard)
	}
	r.key(input.KeyX, ctrl)
	r.rt.memClipboard = "pasted"
	r.key(input.KeyV, ctrl)
	r.key(input.KeyZ, ctrl)
	if v.Peek() != "alpha beta" || changed != 0 {
		t.Fatalf("the read-only value became %q (%d changes)", v.Peek(), changed)
	}
	r.key(input.KeyHome, 0)
	r.key(input.KeyRight, ctrl|shift)
	r.key(input.KeyC, ctrl)
	if r.rt.memClipboard != "alpha " {
		t.Fatalf("copied %q from a read-only field, want %q", r.rt.memClipboard, "alpha ")
	}

	ar := newAccessRig(func(ctx *Context) Element { return TextInput(v).Label("Code").ReadOnly(true) })
	field := accesstest.Find(t, ar.tree(), access.RoleTextField, "Code")
	if !field.States.Has(access.StateReadOnly) || !field.States.Has(access.StateFocusable) {
		t.Fatalf("read-only field states %v", field.States)
	}
	if field.Has(access.ActionSetValue) {
		t.Fatal("a read-only field offers SetValue")
	}
	ar.bridge.Do(field.ID, access.Request{Action: access.ActionSetValue, Value: "changed"})
	ar.frame()
	if v.Peek() != "alpha beta" {
		t.Fatalf("SetValue changed a read-only field to %q", v.Peek())
	}
}

// A disabled field shows its text but can't be focused or changed, and
// assistive technology reports it disabled.
func TestTextFieldDisabled(t *testing.T) {
	v := NewSignal("locked")
	r := newEditorRig(t, func(ctx *Context) Element {
		return Div().Column().Child(TextArea(v).Label("Notes").Disabled(true)).Child(TextInput(NewSignal("")).Label("Next"))
	})
	r.click(50, 30)
	if n, _ := r.editor(); n != nil && n.editor.disabled {
		t.Fatal("a click focused a disabled field")
	}
	r.typeText("x")
	if v.Peek() != "locked" {
		t.Fatalf("typing changed a disabled field to %q", v.Peek())
	}
	r.key(input.KeyTab, 0)
	if n, _ := r.editor(); n == nil || n.editor.name != "Next" {
		t.Fatal("Tab did not skip the disabled field")
	}

	ar := newAccessRig(func(ctx *Context) Element { return TextArea(v).Label("Notes").Disabled(true) })
	field := accesstest.Find(t, ar.tree(), access.RoleTextArea, "Notes")
	if !field.States.Has(access.StateDisabled) || field.States.Has(access.StateFocusable) || len(field.Actions) != 0 {
		t.Fatalf("disabled field states %v, actions %v", field.States, field.Actions)
	}
	if field.Value != "locked" {
		t.Fatalf("disabled field value %q", field.Value)
	}
}

// A read-only field's fill sets it apart in the normal themes, and under high
// contrast it keeps the window color, so its text stays readable.
func TestTextFieldReadOnlyFill(t *testing.T) {
	for _, c := range []struct {
		name  string
		theme *Theme
		want  Color
	}{
		{"dark", DarkTheme(), DarkTheme().SurfaceActive},
		{"light", LightTheme(), LightTheme().SurfaceHover},
	} {
		r := newThemedRig(c.theme, func(ctx *Context) Element {
			return partField("TextInput", NewSignal("x"), FieldStyle{}, false, "readonly")
		})
		if got := partNode(t, r.rt, "TextInput.Frame", "").style.Background; got != c.want {
			t.Errorf("%s: read-only fill %v, want %v", c.name, got, c.want)
		}
	}
	hc := HighContrastTheme(HighContrastBlack(), DarkTheme())
	r := newThemedRig(hc, func(ctx *Context) Element {
		return partField("TextInput", NewSignal("x"), FieldStyle{}, false, "readonly")
	})
	plain := newThemedRig(hc, func(ctx *Context) Element { return partField("TextInput", NewSignal("x"), FieldStyle{}, false, "") })
	got := partNode(t, r.rt, "TextInput.Frame", "").style.Background
	if want := partNode(t, plain.rt, "TextInput.Frame", "").style.Background; got != want {
		t.Errorf("high contrast: read-only fill %v, want the editable field's %v", got, want)
	}
}
