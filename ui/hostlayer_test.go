package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

func layerRuntime(root func(*Context) Element) *Runtime {
	rt := NewRuntime(root, nil, nil)
	rt.SetViewport(400, 300, 1)
	rt.Frame(time.Now())
	return rt
}

// roleCount counts the nodes of the access tree with the role.
func roleCount(rt *Runtime, r access.Role) int {
	n := 0
	rt.AccessTree().Walk(func(a *access.Node) bool {
		if a.Role == r {
			n++
		}
		return true
	})
	return n
}

// The host layer opens a dialog over the app without touching the app's
// elements: their IDs, layout and state stay as they were, also while the app
// re-renders under the dialog.
func TestHostLayerOverTheApp(t *testing.T) {
	label := NewSignal("one")
	open := NewSignal(false)
	var confirmed int
	var counter *Signal[int]
	rt := layerRuntime(func(ctx *Context) Element {
		counter = Local(ctx, "n", 0)
		return Div().Column().Padding(20).Children(
			Button(label.Get()).Key("app-button").OnClick(func() { counter.Update(func(n int) int { return n + 1 }) }),
		)
	})
	before, _ := rt.Find("app-button")
	rt.ClickKey("app-button")

	rt.SetHostLayer(func(ctx *Context) Element {
		if !open.Get() {
			return nil
		}
		return AlertDialog("Save changes?", "Your changes will be lost.").Key("prompt").
			Confirm("Save", func() { confirmed++ }).Cancel("Cancel", func() { open.Set(false) })
	})
	rt.Frame(time.Now())
	if r, _ := rt.Find("app-button"); r != before || !rt.HostLayerShown() {
		t.Errorf("an empty layer moved the app (%+v, was %+v) or is not attached (%v)", r, before, rt.HostLayerShown())
	}
	if roleCount(rt, access.RoleAlertDialog) != 0 {
		t.Fatal("a dialog shows before the layer renders one")
	}

	open.Set(true)
	rt.Frame(time.Now())
	if roleCount(rt, access.RoleAlertDialog) != 1 {
		t.Fatal("the layer's dialog is not in the access tree")
	}
	// The app re-renders under the dialog; the dialog stays, and so does the
	// app's state.
	label.Set("two")
	rt.Frame(time.Now())
	if roleCount(rt, access.RoleAlertDialog) != 1 {
		t.Fatal("the dialog went away when the app's root re-rendered")
	}
	if counter.Peek() != 1 {
		t.Errorf("the app's local state is %d after the layer opened; want 1", counter.Peek())
	}
	if r, _ := rt.Find("app-button"); r.X != before.X || r.Y != before.Y {
		t.Errorf("the dialog moved the app: %+v, was %+v", r, before)
	}
	// The dialog has the focus and takes the input; the app under it does not.
	rt.Press(input.KeyEnter, 0)
	if confirmed != 1 {
		t.Errorf("Enter confirmed %d times; want 1", confirmed)
	}
	rt.ClickKey("app-button")
	if counter.Peek() != 1 {
		t.Error("a click reached the app through the dialog")
	}

	open.Set(false)
	rt.Frame(time.Now())
	if roleCount(rt, access.RoleAlertDialog) != 0 {
		t.Fatal("the dialog stayed after the layer stopped rendering it")
	}
	rt.ClickKey("app-button")
	if counter.Peek() != 2 {
		t.Errorf("the app takes no clicks after the dialog closed (count %d)", counter.Peek())
	}

	open.Set(true)
	rt.SetHostLayer(nil)
	rt.Frame(time.Now())
	if roleCount(rt, access.RoleAlertDialog) != 0 {
		t.Fatal("the dialog stayed after the layer was removed")
	}
}

// A root that lays out no children, such as a Text, has no layer, and says so.
// A component that renders a Div does.
func TestHostLayerRoots(t *testing.T) {
	layer := func(*Context) Element { return AlertDialog("Quit?", "") }
	rt := layerRuntime(func(*Context) Element { return Text("Hello") })
	rt.SetHostLayer(layer)
	rt.Frame(time.Now())
	if rt.HostLayerShown() {
		t.Error("a Text root reports the layer shown")
	}
	rt = layerRuntime(func(*Context) Element {
		return Component(func(*Context) Element { return Div().Child(Text("Hello")) })
	})
	rt.SetHostLayer(layer)
	rt.Frame(time.Now())
	if !rt.HostLayerShown() || roleCount(rt, access.RoleAlertDialog) != 1 {
		t.Error("no dialog over a component that renders a Div")
	}
}
