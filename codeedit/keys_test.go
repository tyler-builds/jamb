package codeedit

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// TestPunctuationBindingsAreInTheBindings checks that the comment, fold and unfold commands have
// their punctuation bindings, as the labels that KeysFor shows for them.
func TestPunctuationBindingsAreInTheBindings(t *testing.T) {
	rt := ui.NewRuntime(func(*ui.Context) ui.Element { return ui.Div() }, ui.DarkTheme(), nil)
	rt.AddBindings(Bindings()...)
	for _, tc := range []struct {
		a    *ui.Action
		want string
	}{
		{CmdComment, "Ctrl+/"},
		{CmdFold, "Ctrl+Shift+["},
		{CmdUnfold, "Ctrl+Shift+]"},
	} {
		if got := rt.KeysFor(tc.a); got != tc.want {
			t.Errorf("KeysFor(%s) = %q, want %q", tc.a.Name(), got, tc.want)
		}
	}
}

// TestPunctuationKeysRunTheCommands checks that the keys reach the commands in the editor, not only
// that the bindings exist.
func TestPunctuationKeysRunTheCommands(t *testing.T) {
	h := newHarness(t, "x := 1", func(e *Editor) *Editor { return e.Comment("// ") })
	h.doc.SetCaret(0)
	h.key(input.KeySlash, input.ModCtrl)
	if h.doc.Text() != "// x := 1" {
		t.Fatalf("ctrl+/ gave %q, want the line commented", h.doc.Text())
	}

	h = newHarness(t, "func a() {\n\tone\n}\nnext")
	h.doc.SetCaret(0)
	h.key(input.KeyLeftBracket, input.ModCtrl|input.ModShift)
	if h.doc.VisibleRows() != 3 {
		t.Fatalf("ctrl+shift+[ left %d rows shown, want 3", h.doc.VisibleRows())
	}
	h.key(input.KeyRightBracket, input.ModCtrl|input.ModShift)
	if h.doc.VisibleRows() != 4 {
		t.Fatalf("ctrl+shift+] left %d rows shown, want 4", h.doc.VisibleRows())
	}
}
