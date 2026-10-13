package keymap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

const exampleKeymap = `[
  {"bindings": {"ctrl+shift+n": "notes.new"}},
  {"context": "Editor", "bindings": {
    "ctrl+d": "editor.delete-forward",
    "ctrl+a": null,
    "ctrl+z": {"unbind": "editor.undo"}
  }}
]`

// rig drives a runtime headless, through its exported API only.
type rig struct {
	rt  *ui.Runtime
	now time.Time
}

func newRig(root func(*ui.Context) ui.Element) *rig {
	r := &rig{now: time.Unix(1_000_000, 0)}
	r.rt = ui.NewRuntime(root, ui.DarkTheme(), nil)
	r.rt.SetViewport(500, 400, 1)
	r.frame()
	return r
}

func (r *rig) frame() { r.rt.Frame(r.now) }

func (r *rig) key(k input.Key, mods input.Modifiers) {
	r.rt.KeyDown(k, mods, false)
	r.rt.KeyUp(k, mods)
	r.frame()
}

func (r *rig) click(x, y float32) {
	r.rt.PointerDown(x, y, input.ButtonLeft, 1, 0)
	r.rt.PointerUp(x, y, input.ButtonLeft, 0)
	r.frame()
}

func (r *rig) typeText(s string) {
	r.rt.TextInput(s)
	r.frame()
}

func TestParseKeepsFileOrder(t *testing.T) {
	newNote := ui.NewAction("notes.new", "New note")
	bs, err := Parse([]byte(exampleKeymap), []*ui.Action{newNote})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(bs) != 4 {
		t.Fatalf("%d bindings, want 4", len(bs))
	}
	r := newRig(func(ctx *ui.Context) ui.Element { return ui.Div() })
	r.rt.AddBindings(bs...)
	if got := r.rt.KeysFor(newNote); got != "Ctrl+Shift+N" {
		t.Errorf("KeysFor(notes.new) = %q, want Ctrl+Shift+N", got)
	}
	if got := r.rt.KeysFor(ui.NoAction); got != "" {
		t.Errorf("KeysFor(NoAction) = %q, want nothing", got)
	}
	if got := r.rt.KeysFor(ui.Undo); got != "" {
		t.Errorf("KeysFor(Undo) = %q after its unbind, want nothing", got)
	}
}

func TestKeymapFileOverridesDefaults(t *testing.T) {
	bs, err := Parse([]byte(exampleKeymap), []*ui.Action{ui.NewAction("notes.new", "New note")})
	if err != nil {
		t.Fatal(err)
	}
	v := ui.NewSignal("one two")
	r := newRig(func(ctx *ui.Context) ui.Element { return ui.Div().Padding(10).Child(ui.TextInput(v)) })
	r.rt.AddBindings(bs...)
	r.click(100, 27)
	r.key(input.KeyHome, 0)
	r.key(input.KeyD, input.ModCtrl)
	if v.Peek() != "ne two" {
		t.Errorf("Ctrl+D from the file (delete forward): %q, want %q", v.Peek(), "ne two")
	}
	r.key(input.KeyA, input.ModCtrl)
	r.typeText("x")
	if v.Peek() != "xne two" {
		t.Errorf("Ctrl+A disabled by the file, then typing x: %q, want %q (nothing selected)", v.Peek(), "xne two")
	}
}

// Two keys for one action: the later one in the file is the one KeysFor shows.
func TestKeysForIsDeterministic(t *testing.T) {
	bs, err := Parse([]byte(`[{"context": "Editor", "bindings": {
		"ctrl+y": "editor.redo", "ctrl+shift+z": "editor.redo"}}]`), nil)
	if err != nil {
		t.Fatal(err)
	}
	r := newRig(func(ctx *ui.Context) ui.Element { return ui.Div() })
	r.rt.AddBindings(bs...)
	if got := r.rt.KeysFor(ui.Redo); got != "Ctrl+Shift+Z" {
		t.Errorf("KeysFor(Redo) = %q, want Ctrl+Shift+Z", got)
	}
}

// Every kind of problem is reported with its section, context and keys, and
// the valid bindings in the same file still come back.
func TestParseReportsEveryProblem(t *testing.T) {
	data := `[
  {"bindings": {"ctrl+n": "notes.new", "ctrl+q": "no.such"}},
  {"context": "Editr &&", "bindings": {"ctrl+b": "editor.undo"}},
  {"context": "Editor", "bindings": {
    "ctrl+d": "editor.delet-forward",
    "ctrl+shfit+x": "editor.undo",
    "f9": 5,
    "ctrl+u": {"unbind": "editor.nope"}
  }},
  {"bindngs": {}},
  {"context": "Editor", "bindings": {"ctrl+y": "editor.redo"}}
]`
	bs, err := Parse([]byte(data), []*ui.Action{ui.NewAction("notes.new", "New note")})
	if err == nil {
		t.Fatal("Parse reported no problems")
	}
	msg := err.Error()
	for _, want := range []string{
		`keymap section 1 (no context): "ctrl+q": unknown action "no.such"`,
		`keymap section 2 (context "Editr &&"): context:`,
		`keymap section 3 (context "Editor"): "ctrl+d": unknown action "editor.delet-forward"`,
		`keymap section 3 (context "Editor"): "ctrl+shfit+x": unknown modifier "shfit"`,
		`keymap section 3 (context "Editor"): "f9": the value must be`,
		`keymap section 3 (context "Editor"): "ctrl+u": unknown action "editor.nope"`,
		`keymap section 4: unknown field "bindngs"`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("errors do not contain %q\nerrors:\n%s", want, msg)
		}
	}
	if len(bs) != 2 {
		t.Errorf("%d valid bindings, want 2 (ctrl+n and ctrl+y)", len(bs))
	}
	// The section with a bad context binds nothing, and the valid ones are
	// the global ctrl+n and the Editor ctrl+y.
	r := newRig(func(ctx *ui.Context) ui.Element { return ui.Div() })
	r.rt.AddBindings(bs...)
	if got := r.rt.KeysFor(ui.Redo); got != "Ctrl+Y" {
		t.Errorf("KeysFor(Redo) = %q, want Ctrl+Y", got)
	}
}

func TestParseRejectsWrongShape(t *testing.T) {
	for _, data := range []string{`{"bindings": {}}`, `[`, `"ctrl+n"`} {
		bs, err := Parse([]byte(data), nil)
		if err == nil || bs != nil {
			t.Errorf("Parse(%s) = %v, %v; want no bindings and an error", data, bs, err)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	bs, err := Load(filepath.Join(dir, "missing.json"), nil)
	if bs != nil || err != nil {
		t.Errorf("missing file: %v, %v; want nil, nil", bs, err)
	}

	path := filepath.Join(dir, "keymap.json")
	if err := os.WriteFile(path, []byte(`[{"bindings": {"ctrl+n": "test.file.new"}}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	act := ui.NewAction("test.file.new", "New")
	bs, err = Load(path, []*ui.Action{act})
	if err != nil || len(bs) != 1 {
		t.Fatalf("Load: %v, %v; want one binding for the named action", bs, err)
	}

	if err := os.WriteFile(path, []byte(`[{"bindings": {"ctrl+n": "test.file.nope"}}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	bs, err = Load(path, []*ui.Action{act})
	if err == nil || !strings.Contains(err.Error(), path) || len(bs) != 0 {
		t.Errorf("bad file: %v, %v; want an error that names %s", bs, err, path)
	}
}

// A keymap file's layer keeps its place: the app binding for the same keys
// loses to the file while the file binds them, and wins again once the file
// no longer does.
func TestFileApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keymap.json")
	write := func(s string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	newNote := ui.NewAction("notes.new", "New note")
	other := ui.NewAction("test.other", "Other")
	var fired, appFired int
	r := newRig(func(ctx *ui.Context) ui.Element {
		return ui.Div().OnAction(newNote, func() { fired++ }).OnAction(other, func() { appFired++ })
	})
	f := &File{Path: path, Actions: []*ui.Action{newNote}}
	r.rt.AddBindings(ui.Bind("ctrl+shift+n", other))
	press := func(k input.Key) { r.key(k, input.ModCtrl|input.ModShift) }
	expect := func(what string, wantFired, wantApp int) {
		t.Helper()
		if fired != wantFired || appFired != wantApp {
			t.Fatalf("%s: notes.new ran %d times, app binding %d times; want %d and %d",
				what, fired, appFired, wantFired, wantApp)
		}
	}

	if changed, err := f.Apply(r.rt); changed || err != nil {
		t.Fatalf("missing file: changed %v, err %v; want false, nil", changed, err)
	}
	press(input.KeyN)
	expect("no file", 0, 1)

	write(`[{"bindings": {"ctrl+shift+n": "notes.new"}}]`)
	if changed, err := f.Apply(r.rt); !changed || err != nil {
		t.Fatalf("new file: changed %v, err %v; want true, nil", changed, err)
	}
	press(input.KeyN)
	expect("file binding", 1, 1)

	if changed, err := f.Apply(r.rt); changed || err != nil {
		t.Fatalf("same bytes: changed %v, err %v; want false, nil", changed, err)
	}

	write(`[{"bindings": {"ctrl+shift+n": "notes.new"}`) // truncated, as during a save
	if changed, err := f.Apply(r.rt); !changed || err == nil {
		t.Fatalf("invalid JSON: changed %v, err %v; want true and an error", changed, err)
	}
	press(input.KeyN)
	expect("invalid JSON keeps the layer", 2, 1)

	write(`[{"bindings": {"ctrl+shift+m": "notes.new"}}]`)
	if changed, err := f.Apply(r.rt); !changed || err != nil {
		t.Fatalf("fixed file: changed %v, err %v; want true, nil", changed, err)
	}
	press(input.KeyM)
	expect("fixed file applies", 3, 1)
	press(input.KeyN)
	expect("the layer no longer binds ctrl+shift+n", 3, 2)

	write(`[{"bindings": {"ctrl+shift+n": "notes.new", "ctrl+shift+q": "no.such.action"}}]`)
	if changed, err := f.Apply(r.rt); !changed || err == nil {
		t.Fatalf("partly bad file: changed %v, err %v; want true and an error", changed, err)
	}
	press(input.KeyN)
	expect("valid entries of a partly bad file apply", 4, 2)

	write(`[]`)
	if changed, err := f.Apply(r.rt); !changed || err != nil {
		t.Fatalf("empty array: changed %v, err %v; want true, nil", changed, err)
	}
	press(input.KeyN)
	expect("empty array removes the file's bindings", 4, 3)

	write(`[{"bindings": {"ctrl+shift+m": "notes.new"}}]`)
	f.Apply(r.rt)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if changed, err := f.Apply(r.rt); !changed || err != nil {
		t.Fatalf("deleted file: changed %v, err %v; want true, nil", changed, err)
	}
	press(input.KeyM)
	expect("deleted file removes its layer", 4, 3)
}

// A file's unbind turns off a default binding, and the rest of the defaults
// stay in place.
func TestFileApplyKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keymap.json")
	if err := os.WriteFile(path, []byte(exampleKeymap), 0o644); err != nil {
		t.Fatal(err)
	}
	r := newRig(func(ctx *ui.Context) ui.Element { return ui.Div() })
	f := &File{Path: path, Actions: []*ui.Action{ui.NewAction("notes.new", "New note")}}
	if changed, err := f.Apply(r.rt); !changed || err != nil {
		t.Fatalf("Apply: changed %v, err %v", changed, err)
	}
	if got := r.rt.KeysFor(ui.Undo); got != "" {
		t.Errorf("file unbinds Undo, but KeysFor shows %q", got)
	}
	if got := r.rt.KeysFor(ui.Redo); got != "Ctrl+Y" {
		t.Errorf("KeysFor(Redo) = %q, want the default Ctrl+Y", got)
	}
}
