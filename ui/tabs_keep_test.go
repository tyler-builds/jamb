package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/input"
)

// keptDemo is tabsDemo with KeepPanels. The first panel holds a text field bound
// to text, and the third a counter in a component, so the tests can see whether
// each panel's state survives switching tabs.
func keptDemo(sel *Signal[string], built *[]string, text *Signal[string], keep bool) Element {
	tabs := Tabs(sel,
		Tab[string]{Value: "a", Label: "Alpha", Panel: func() Element {
			*built = append(*built, "a")
			return Div().Column().Children(
				TextInput(text).Key("field-a").Label("Alpha note"),
				Button("Inside alpha").Key("inside-a"),
			)
		}},
		Tab[string]{Value: "b", Label: "Beta", Disabled: true, Panel: func() Element {
			*built = append(*built, "b")
			return Text("Beta panel")
		}},
		Tab[string]{Value: "c", Label: "Gamma", Panel: func() Element {
			*built = append(*built, "c")
			return Component(func(ctx *Context) Element {
				count := Local(ctx, "count", 0)
				return Div().Column().Children(
					Button("Add").Key("add-c").OnClick(func() { count.Set(count.Peek() + 1) }),
					Text(fmt.Sprintf("Count %d", count.Get())).Key("count-c"),
				)
			})
		}},
	).Key("settings").Label("Settings sections")
	if keep {
		tabs.KeepPanels()
	}
	return Div().Column().Child(tabs)
}

// tabPanels returns the tab panels in the accessibility tree.
func tabPanels(r *accessRig) []*access.Node {
	var out []*access.Node
	r.tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleTabPanel {
			out = append(out, n)
		}
		return true
	})
	return out
}

func TestTabsKeepPanelsBuildEachPanelOnceAndKeepIt(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element { return keptDemo(sel, &built, text, true) })
	if got := strings.Join(built, ","); got != "a" {
		t.Fatalf("built panels %q at first, want only the selected one (a)", got)
	}
	r.clickKey(t, "settings/c")
	r.clickKey(t, "settings/a")
	r.clickKey(t, "settings/c")
	if got := strings.Join(built, ","); got != "a,c" {
		t.Fatalf("built panels %q after switching back and forth, want a,c: each is built once", got)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsKeepPanelsStateSurvivesSwitching(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element { return keptDemo(sel, &built, text, true) })
	r.clickKey(t, "field-a")
	r.typeText("hi")
	if got := text.Peek(); got != "hi" {
		t.Fatalf("typing into the field gave %q, want hi", got)
	}
	r.clickKey(t, "settings/c")
	r.clickKey(t, "add-c")
	r.clickKey(t, "add-c")
	if namedInTree(r, "Alpha note") {
		t.Fatalf("the field of the hidden Alpha panel is in the accessibility tree")
	}
	if !namedInTree(r, "Count 2") {
		t.Fatalf("the Gamma panel does not show its count")
	}
	r.clickKey(t, "settings/a")
	if got := text.Peek(); got != "hi" {
		t.Fatalf("text is %q after switching back, want hi", got)
	}
	if !namedInTree(r, "Alpha note") {
		t.Fatalf("the field of the shown Alpha panel is not in the accessibility tree")
	}
	if namedInTree(r, "Count 2") {
		t.Fatalf("the hidden Gamma panel is in the accessibility tree")
	}
	r.clickKey(t, "settings/c")
	if !namedInTree(r, "Count 2") {
		t.Fatalf("the count was lost after switching back to Gamma")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsDefaultDiscardsPanelsWhenSwitching(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element { return keptDemo(sel, &built, text, false) })
	r.clickKey(t, "settings/c")
	r.clickKey(t, "settings/a")
	if got := strings.Join(built, ","); got != "a,c,a" {
		t.Fatalf("built panels %q, want a,c,a: without KeepPanels a panel is built again", got)
	}
	if namedInTree(r, "Gamma panel") || len(tabPanels(r)) != 1 {
		t.Fatalf("without KeepPanels only the selected panel is in the tree")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsKeepPanelsKeyboardAndTabOrder(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element { return keptDemo(sel, &built, text, true) })
	r.clickKey(t, "settings/c")
	r.clickKey(t, "settings/a")
	inOrder := func(key string) bool {
		id := r.nodeByKey(t, key).id
		for _, x := range r.rt.tabOrder() {
			if x == id {
				return true
			}
		}
		return false
	}
	if inOrder("settings/c") || inOrder("add-c") {
		t.Fatalf("a hidden tab or the hidden Gamma panel is in the Tab order")
	}
	if !inOrder("settings/a") || !inOrder("inside-a") {
		// inside-a is a button in the selected panel, so it is in the order.
		t.Fatalf("the selected tab or its panel is missing from the Tab order")
	}
	r.focusKey(t, "inside-a")
	r.press(input.KeyTab, input.ModCtrl)
	if sel.Peek() != "c" {
		t.Fatalf("Ctrl+Tab from the kept panel selected %q, want c", sel.Peek())
	}
	if got := r.focusedKey(); got != "settings/c" {
		t.Fatalf("focus is on %q after Ctrl+Tab, want the selected tab", got)
	}
	r.press(input.KeyTab, input.ModCtrl) // wraps to Alpha, past the hidden Beta
	if sel.Peek() != "a" {
		t.Fatalf("Ctrl+Tab did not wrap: selected %q", sel.Peek())
	}
	r.press(input.KeyLeft, 0) // Beta is disabled, so there is nothing before Alpha
	if sel.Peek() != "a" {
		t.Fatalf("Left selected %q, want a", sel.Peek())
	}
	r.press(input.KeyEnd, 0)
	if sel.Peek() != "c" || strings.Join(built, ",") != "a,c" {
		t.Fatalf("End selected %q with built %q, want c with a,c", sel.Peek(), strings.Join(built, ","))
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestTabsKeepPanelsAccessTreeShowsOnlySelectedPanel(t *testing.T) {
	sel := NewSignal("a")
	var built []string
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element { return keptDemo(sel, &built, text, true) })
	r.clickKey(t, "settings/c")
	panels := tabPanels(r)
	if len(panels) != 1 {
		t.Fatalf("the tree has %d tab panels, want the one selected", len(panels))
	}
	gamma := r.stateOf(t, access.RoleTab, "Gamma")
	if len(panels[0].LabelledBy) != 1 || panels[0].LabelledBy[0] != gamma.ID {
		t.Fatalf("the panel is labelled by %v, want the Gamma tab %d", panels[0].LabelledBy, gamma.ID)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}
