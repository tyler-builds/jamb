package codeedit

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/internal/inspect"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// TestEditorStyleLayersAreNamed checks that the editor's box reports the radius that its
// Styles set, with the built-in default's radius overridden below it.
func TestEditorStyleLayersAreNamed(t *testing.T) {
	ed := Editor(NewDocument("func main() {}")).Key("code").Styles(Style{Box: ui.Style{Radius: 2}})
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element { return ui.Div().Child(ed) }, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	rt.Frame(time.Now())
	tg, ok := uihost.Of(rt).(inspect.Target)
	if !ok {
		t.Fatal("the runtime's host side does not implement inspect.Target")
	}
	tg.Attach(func() {})
	rt.Frame(time.Now())

	box, ok := findRadius(tg, tg.Root(), "2")
	if !ok {
		t.Fatal("no element with a radius of 2 from the instance Styles")
	}
	for _, f := range tg.Fields(box) {
		if f.Name != "Radius" {
			continue
		}
		if len(f.Layers) < 2 || f.Layers[0].Source != inspect.SourceDefault || !f.Layers[0].Overridden {
			t.Fatalf("Radius of the editor box = %+v, want the built-in default overridden", f)
		}
		return
	}
	t.Fatal("the editor box has no Radius")
}

// findRadius returns the first element of the subtree at id whose Radius is the value
// from the instance Styles.
func findRadius(tg inspect.Target, id uint64, value string) (uint64, bool) {
	for _, f := range tg.Fields(id) {
		if f.Name == "Radius" && f.Source == inspect.SourceInstance && f.Value == value {
			return id, true
		}
	}
	e, ok := tg.Element(id)
	if !ok {
		return 0, false
	}
	for _, c := range e.Children {
		if f, ok := findRadius(tg, c, value); ok {
			return f, true
		}
	}
	return 0, false
}
