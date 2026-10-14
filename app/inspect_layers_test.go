package app

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// inspFindPart returns the first element of the subtree at id whose part is part.
func inspFindPart(tg inspect.Target, id uint64, part string) (uint64, bool) {
	e, ok := tg.Element(id)
	if !ok {
		return 0, false
	}
	if e.Part == part {
		return id, true
	}
	for _, c := range e.Children {
		if f, ok := inspFindPart(tg, c, part); ok {
			return f, true
		}
	}
	return 0, false
}

// TestInspectorWindowShowsOverriddenLayers renders the inspector's window for the box of
// a check box that its theme and its instance both style. The window lists the value
// that wins, the layer it came from, and the values it overrides, with no accessibility
// warnings.
func TestInspectorWindowShowsOverriddenLayers(t *testing.T) {
	th := ui.DarkTheme()
	th.Checkbox.Box = ui.Style{Radius: 5}
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		return ui.Div().Key("root").Child(
			ui.Checkbox("Remember", ui.NewSignal(false)).Styles(ui.CheckboxStyle{Box: ui.Style{Radius: 9}}))
	}, th, nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Now())
	tg, ok := uihost.Of(rt).(inspect.Target)
	if !ok {
		t.Fatal("the runtime's host side does not implement inspect.Target")
	}
	h := &host{rt: rt, handle: &appWindow{}, cfg: Window{Title: "Sample"}}
	ins := newInspector(h, tg)
	tg.Attach(ins.changed)
	rt.Frame(time.Now()) // the components are rendered again, with their layers
	box, ok := inspFindPart(tg, tg.Root(), "Checkbox.Box")
	if !ok {
		t.Fatal("no element with the part Checkbox.Box")
	}
	tg.Select(box)

	irt := ui.NewRuntime(ins.root, ui.DarkTheme(), nil)
	var warnings []string
	irt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
	irt.SetAccessWarnings(true)
	irt.SetViewport(1100, 760, 1)
	irt.Frame(time.Now())
	if len(warnings) != 0 {
		t.Fatalf("the inspector's window has accessibility warnings: %v", warnings)
	}
	var dump strings.Builder
	irt.Dump(&dump, 0)
	// The dump shortens each text to 32 characters, so the lines are matched by their
	// start: the layer of the value is in the rest of the line.
	for _, want := range []string{
		"Radius = 9   (instance Styles)",
		"overridden: Radius = 5",
		"overridden: Radius = 4",
	} {
		if !strings.Contains(dump.String(), want) {
			t.Errorf("the inspector's window does not show %q", want)
		}
	}
}
