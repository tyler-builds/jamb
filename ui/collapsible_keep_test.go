package ui

import (
	"fmt"
	"testing"
)

// keptSection is a collapsible section named Details whose content holds a
// counter in a component. keep sets KeepMounted.
func keptSection(open *Signal[bool], keep bool) Element {
	c := Collapsible("Details", open, Component(func(ctx *Context) Element {
		count := Local(ctx, "count", 0)
		return Div().Column().Children(
			Button("Add").Key("add").OnClick(func() { count.Set(count.Peek() + 1) }),
			Text(fmt.Sprintf("Count %d", count.Get())),
		)
	})).Key("details")
	if keep {
		c.KeepMounted()
	}
	return Div().Column().Child(c)
}

func TestCollapsibleKeepMountedStateSurvivesClosing(t *testing.T) {
	open := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element { return keptSection(open, true) })
	if namedInTree(r, "Count 0") || namedInTree(r, "Add") {
		t.Fatalf("the content of a section that was never opened is in the tree")
	}
	r.clickKey(t, "details/header")
	r.clickKey(t, "add")
	r.clickKey(t, "add")
	if !namedInTree(r, "Count 2") {
		t.Fatalf("the open section does not show its count")
	}
	r.clickKey(t, "details/header") // close
	if namedInTree(r, "Count 2") || namedInTree(r, "Add") {
		t.Fatalf("the content of the closed section is in the accessibility tree")
	}
	r.clickKey(t, "details/header") // open again
	if !namedInTree(r, "Count 2") {
		t.Fatalf("the count was lost when the section closed")
	}
	r.clickKey(t, "add")
	if !namedInTree(r, "Count 3") {
		t.Fatalf("the kept content does not respond once shown again")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestCollapsibleDefaultDiscardsContentWhenClosed(t *testing.T) {
	open := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element { return keptSection(open, false) })
	r.clickKey(t, "details/header")
	r.clickKey(t, "add")
	r.clickKey(t, "details/header") // close
	r.clickKey(t, "details/header") // open again
	if !namedInTree(r, "Count 0") {
		t.Fatalf("without KeepMounted the count should start again at 0 on opening")
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}
