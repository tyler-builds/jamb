package ui

import "testing"

// countLive returns the nodes of the tree, and the nodes the runtime has by ID. A
// node rebuilt in place is one node in both counts, so they agree when no node is
// left behind by a build or missing from it.
func countLive(rt *Runtime) (inTree, byID int) {
	inTree = len(nodeSet(rt.tree, map[*node]bool{}))
	return inTree, len(rt.byID)
}

// TestRebuildKeepsNodesAndDropsRemovedOnes checks that a list which changes on each
// frame, with rows added, removed and rebuilt, keeps the nodes of the rows that stay
// (the same objects), and leaves neither a node in the tree that the runtime has
// lost nor a node the runtime keeps that the tree does not show.
func TestRebuildKeepsNodesAndDropsRemovedOnes(t *testing.T) {
	rows := NewSignal([]string{"a", "b", "c", "d"})
	r := newAccessRig(func(ctx *Context) Element {
		box := Div().Key("box").Column()
		for _, k := range rows.Get() {
			box.Child(Div().Key("row-" + k).Column().Child(Text(k)))
		}
		return box
	})
	kept := r.nodeByKey(t, "row-a")
	for i, next := range [][]string{{"a", "c", "d"}, {"a", "e", "c", "d"}, {"c", "d", "a", "f"}, {"a"}} {
		rows.Set(next)
		r.frame()
		if inTree, byID := countLive(r.rt); inTree != byID {
			t.Fatalf("step %d: %d nodes in the tree, %d by ID", i, inTree, byID)
		}
	}
	if r.nodeByKey(t, "row-a") != kept {
		t.Error("a row that stayed was built as a new node")
	}
	if r.rt.nodesReused == 0 {
		t.Error("no node was rebuilt in place")
	}
}

// TestFindKeyInABuildSeesTheLaidOutChildren checks that a node rebuilt in place
// answers a key search made during the build with the children it had when it was
// last laid out (see findKey), since its new children are not built yet. The inner
// component renders while its parent box is being built, and looks for a child of
// the box that the new output drops.
func TestFindKeyInABuildSeesTheLaidOutChildren(t *testing.T) {
	showX := NewSignal(true)
	tick := NewSignal(0)
	var foundDuringBuild []bool
	r := newAccessRig(func(ctx *Context) Element {
		box := Div().Key("box").Column()
		if showX.Get() {
			box.Child(Div().Key("x"))
		}
		box.Child(Component(func(ctx *Context) Element {
			tick.Get()
			foundDuringBuild = append(foundDuringBuild, ctx.rt.findKey(ctx.rt.tree, "x") != nil)
			return Div().Key("inner")
		}))
		return box
	})
	showX.Set(false)
	tick.Set(1)
	r.frame()
	if len(foundDuringBuild) < 2 {
		t.Fatalf("the inner component rendered %d times, want it to render again", len(foundDuringBuild))
	}
	if last := foundDuringBuild[len(foundDuringBuild)-1]; !last {
		t.Error("during the rebuild, the key of a dropped child was not found in the laid-out tree")
	}
}

// TestFocusStaysOnANodeRebuiltInPlace checks that a text field keeps the focus when
// its parent is rebuilt in place, and that its text is the text of the new build.
func TestFocusStaysOnANodeRebuiltInPlace(t *testing.T) {
	label := NewSignal("first")
	text := NewSignal("")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Key("form").Column().
			Child(Text(label.Get()).Key("label")).
			Child(TextInput(text).Label("Name").Key("name"))
	})
	r.focusKey(t, "name")
	label.Set("second")
	r.frame()
	if got := r.focusedKey(); got != "name" {
		t.Fatalf("after the rebuild, focus is on %q, want the field", got)
	}
	if n := r.nodeByKey(t, "label"); findText(n, "second") == nil {
		t.Error("the rebuilt label does not show the new text")
	}
	if r.rt.nodesReused == 0 {
		t.Error("the form was not rebuilt in place")
	}
}
