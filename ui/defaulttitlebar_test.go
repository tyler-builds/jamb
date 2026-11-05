package ui

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
)

// The default title bar takes the top of the window: the app is laid out under
// it, keeps its IDs, and the bar drags the window and holds the caption buttons.
func TestDefaultTitleBar(t *testing.T) {
	clicks := 0
	rt := NewRuntime(func(*Context) Element {
		return Div().Column().Children(
			Button("Top").Key("top").OnClick(func() { clicks++ }),
			Div().Grow(1).Key("rest"),
		)
	}, nil, nil)
	rt.SetViewport(400, 300, 1)
	rt.Frame(time.Now())
	before, _ := rt.Find("top")
	rest, _ := rt.Find("rest")

	rt.SetDefaultTitleBar(true, "Report.txt")
	rt.Frame(time.Now())
	after, _ := rt.Find("top")
	if after.Y != before.Y+DefaultTitleBarHeight || after.X != before.X {
		t.Fatalf("the app's first element is at %+v; want %+v moved down %d", after, before, DefaultTitleBarHeight)
	}
	if r, _ := rt.Find("rest"); r.Y+r.H != rest.Y+rest.H || r.H != rest.H-DefaultTitleBarHeight {
		t.Errorf("the app does not fill the rest of the window: %+v, was %+v", r, rest)
	}
	var title bool
	buttons := map[string]bool{}
	rt.AccessTree().Walk(func(n *access.Node) bool {
		title = title || (n.Role == access.RoleStaticText && n.Name == "Report.txt")
		if n.Role == access.RoleButton {
			buttons[n.Name] = true
		}
		return true
	})
	if !title || !buttons["Minimize"] || !buttons["Close"] {
		t.Errorf("title shown %v, buttons %v", title, buttons)
	}
	if a := rt.hostWindowAreaAt(100, 10); a != WindowAreaDrag {
		t.Errorf("the bar's middle is area %v; want a drag area", a)
	}
	if a := rt.hostWindowAreaAt(100, DefaultTitleBarHeight+20); a == WindowAreaDrag {
		t.Error("the app under the bar is a drag area")
	}
	rt.ClickKey("top")
	if clicks != 1 {
		t.Error("a click on the app under the bar did not reach it")
	}

	rt.SetDefaultTitleBar(true, "Report 2.txt")
	rt.Frame(time.Now())
	renamed := false
	rt.AccessTree().Walk(func(n *access.Node) bool {
		renamed = renamed || n.Name == "Report 2.txt"
		return true
	})
	if !renamed {
		t.Error("the bar kept its old title")
	}

	rt.SetDefaultTitleBar(false, "")
	rt.Frame(time.Now())
	if r, _ := rt.Find("top"); r != before {
		t.Errorf("without the bar the app is at %+v; want %+v", r, before)
	}
}
