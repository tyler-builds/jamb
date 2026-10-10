package ui

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
)

// testImage is a small image for the tests that need one.
func testImage() *ImageSource { return ImageFromImage(image.NewRGBA(image.Rect(0, 0, 4, 4))) }

// accessRig is a headless runtime with its access bridge activated, as an
// assistive technology would leave it, and the warnings it printed.
type accessRig struct {
	rt       *Runtime
	bridge   *access.Bridge
	warnings []string
	now      time.Time
}

func newAccessRig(root func(*Context) Element) *accessRig {
	r := &accessRig{now: time.Unix(1_000_000, 0)}
	r.rt = NewRuntime(root, DarkTheme, nil)
	r.rt.clock = func() time.Time { return r.now }
	r.rt.SetWarnFunc(func(s string) { r.warnings = append(r.warnings, s) })
	r.rt.SetAccessWarnings(true)
	r.rt.SetViewport(500, 400, 1)
	r.rt.Frame(r.now)
	r.bridge = r.rt.AccessBridge()
	r.bridge.Activate()
	return r
}

func (r *accessRig) frame() {
	r.now = r.now.Add(16 * time.Millisecond)
	r.rt.Frame(r.now)
}

// tree returns the published snapshot after a frame.
func (r *accessRig) tree() *access.Tree {
	r.frame()
	return r.bridge.Tree()
}

// nodeByKey returns the node built for the element with the key.
func (r *accessRig) nodeByKey(t *testing.T, key string) *node {
	t.Helper()
	for _, n := range r.rt.byID {
		if n.key == key {
			return n
		}
	}
	t.Fatalf("no node with key %q", key)
	return nil
}

func TestAccessRolesForBuiltInWidgets(t *testing.T) {
	text := NewSignal("hello")
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Button("Save"),
			IconButton(icons.Bell).Label("Notifications"),
			TextInput(text).Label("Search"),
			TextArea(text).Label("Notes"),
			Image(testImage()).Alt("Avatar of a sample user"),
			Text("Standalone text"),
		)
	})
	tr := r.tree()
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Save"), access.RoleButton, "Save", access.StateFocusable)
	accesstest.Find(t, tr, access.RoleButton, "Notifications")
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleTextField, "Search"), access.RoleTextField, "Search", access.StateFocusable)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleTextArea, "Notes"), access.RoleTextArea, "Notes", access.StateMultiLine)
	accesstest.Find(t, tr, access.RoleImage, "Avatar of a sample user")
	accesstest.Find(t, tr, access.RoleStaticText, "Standalone text")
	if len(r.warnings) != 0 {
		t.Fatalf("unexpected warnings: %q", r.warnings)
	}
}

func TestAccessIconButtonWithoutLabelWarnsOnce(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(IconButton(icons.Bell), Image(testImage()))
	})
	r.frame()
	r.frame()
	var button, image int
	for _, w := range r.warnings {
		switch {
		case strings.Contains(w, "button has no accessible name"):
			button++
		case strings.Contains(w, "image has no alternative text"):
			image++
		}
	}
	if button != 1 || image != 1 {
		t.Fatalf("warnings: %d button, %d image, want 1 each: %q", button, image, r.warnings)
	}
}

func TestAccessNamePrecedence(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			// Label wins over the text inside.
			Div().Role(access.RoleButton).Label("Explicit").Focusable().OnClick(func() {}).Child(Text("Visible")),
			// Text inside names a clickable element when there is no label.
			Div().Focusable().OnClick(func() {}).Key("text-named").Child(Text("From text")),
			// The tooltip names an element with nothing else.
			Div().Role(access.RoleButton).Tooltip("From tooltip").Focusable().OnClick(func() {}),
		)
	})
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleButton, "Explicit")
	accesstest.Absent(t, tr, access.RoleButton, "Visible")
	accesstest.Find(t, tr, access.RoleButton, "From text")
	accesstest.Find(t, tr, access.RoleButton, "From tooltip")
}

func TestAccessLabelBeatsTooltipAndAllowsDescription(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Children(IconButton(icons.Bell).Label("Notifications").Tooltip("Show alerts"))
	})
	tr := r.tree()
	n := accesstest.Find(t, tr, access.RoleButton, "Notifications")
	if n.Description != "Show alerts" {
		t.Fatalf("description %q, want the tooltip as help text", n.Description)
	}
}

func TestAccessFlattensLayoutContainers(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Div().Row().Children(
				Div().Column().Children(Button("Inner")),
			),
			Div().Child(Text("Nested text")),
		)
	})
	tr := r.tree()
	window := tr.Node(access.RootID)
	// The window's children are the button and the text, not the containers.
	got := accesstest.Children(tr, window)
	want := []string{"Inner", "Nested text"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("window children %q, want %q\n%s", got, want, tr.Dump())
	}
}

func TestAccessHiddenSubtreeIsLeftOut(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Div().Hidden().Child(Button("Secret")),
			Button("Shown"),
		)
	})
	tr := r.tree()
	accesstest.Absent(t, tr, access.RoleButton, "Secret")
	accesstest.Find(t, tr, access.RoleButton, "Shown")
}

func TestAccessOverlayTooltipFollowsItsAnchor(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Button("First"),
			Button("Anchor").Tooltip("Help for anchor"),
			Button("Last"),
		)
	})
	r.frame()
	// Show the tooltip of the middle button and check that it sits inside that
	// button, rather than at the end of the window.
	for _, n := range r.rt.byID {
		if n.div != nil && n.div.tooltip == "Help for anchor" {
			r.rt.showTip(n.id)
		}
	}
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleTooltip, "Help for anchor")
	var anchorNode *access.Node
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleButton && n.Name == "Anchor" {
			anchorNode = n
		}
		return true
	})
	if anchorNode == nil {
		t.Fatalf("no Anchor button:\n%s", tr.Dump())
	}
	kids := anchorNode.Children
	if len(kids) != 1 || tr.Node(kids[0]).Role != access.RoleTooltip {
		t.Fatalf("tooltip is not a child of its anchor: %q", accesstest.Children(tr, anchorNode))
	}
	window := accesstest.Children(tr, tr.Node(access.RootID))
	if len(window) != 3 || window[0] != "First" || window[2] != "Last" {
		t.Fatalf("window children %q, want First, Anchor, Last in order", window)
	}
}

func TestAccessVirtualizedListPositions(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Size(Full, Px(120)).Child(UniformList(10000, func(i int) Element {
			return Text(fmt.Sprintf("Row %d", i+1)).NoWrap()
		}).Style(Style{Height: Px(120)}))
	})
	var rows int
	tr := r.tree()
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleListItem {
			rows++
			if n.SetSize != 10000 || n.PosInSet < 1 || n.PosInSet > 10000 {
				t.Fatalf("row %q has position %d of %d", n.Name, n.PosInSet, n.SetSize)
			}
			if n.Name != fmt.Sprintf("Row %d", n.PosInSet) {
				t.Fatalf("row at position %d is named %q", n.PosInSet, n.Name)
			}
		}
		return true
	})
	if rows == 0 || rows > 100 {
		t.Fatalf("%d list rows; only the visible rows should be built", rows)
	}
	accesstest.Find(t, tr, access.RoleList, "")
}

func TestAccessMenuItemsAndRoles(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Key("target").Size(Full, Full).ContextMenu(func() []MenuItem {
			return []MenuItem{
				{Label: "Open", OnClick: func() {}},
				MenuSeparator(),
				{Label: "Pinned", Check: MenuCheckBox, Checked: true, OnClick: func() {}},
				{Label: "Locked", Disabled: true, OnClick: func() {}},
			}
		}).Child(Text("Body"))
	})
	r.rt.openContextMenu(r.nodeByKey(t, "target"), 20, 20, false)
	tr := r.tree()
	accesstest.Find(t, tr, access.RoleMenu, "Actions")
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleMenuItem, "Open"), access.RoleMenuItem, "Open", 0)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleMenuItemCheck, "Pinned"), access.RoleMenuItemCheck, "Pinned", access.StateChecked)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleMenuItem, "Locked"), access.RoleMenuItem, "Locked", access.StateDisabled)
	accesstest.Find(t, tr, access.RoleSeparator, "")
}

func TestAccessDialogIsNamedByItsTitle(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Size(Full, Full).Child(Modal("New project", Text("Choose a name")).Footer(Button("Cancel")))
	})
	tr := r.tree()
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleDialog, "New project"), access.RoleDialog, "New project", 0)
}

func TestAccessIDsAreStableAcrossFrames(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(Button("One"), Button("Two"))
	})
	first := r.tree()
	second := r.tree()
	if fmt.Sprint(first.Dump()) != fmt.Sprint(second.Dump()) {
		t.Fatalf("tree changed without a UI change:\n%s\n%s", first.Dump(), second.Dump())
	}
	one := accesstest.Find(t, first, access.RoleButton, "One").ID
	if got := accesstest.Find(t, second, access.RoleButton, "One").ID; got != one {
		t.Fatalf("ID of \"One\" changed from %d to %d", one, got)
	}
}

func TestAccessPublishesOnlyChanges(t *testing.T) {
	label := NewSignal("Count: 1")
	var got []access.Update
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(Button("Other"), Text(label.Get()))
	})
	r.bridge.Listen(func(u access.Update) { got = append(got, u) })
	label.Set("Count: 2")
	r.frame()
	if len(got) == 0 {
		t.Fatal("no update published after a change")
	}
	u := got[len(got)-1]
	if u.Structure {
		t.Fatalf("text change reported as a structure change: %+v", u)
	}
	if len(u.Changed) == 0 {
		t.Fatalf("text change reported no changed nodes: %+v", u)
	}
	for _, id := range u.Changed {
		if n := r.bridge.Tree().Node(id); n == nil || (n.Role != access.RoleStaticText && n.Role != access.RoleWindow && n.Role != access.RoleGroup) {
			t.Fatalf("unexpected changed node %v (%+v)", id, n)
		}
	}
}

func TestAccessStructureChangeWhenNodesAppear(t *testing.T) {
	show := NewSignal(false)
	var got []access.Update
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(Button("Base"), If(show.Get(), Button("Extra")))
	})
	r.bridge.Listen(func(u access.Update) { got = append(got, u) })
	show.Set(true)
	r.frame()
	u := got[len(got)-1]
	if !u.Structure || len(u.Added) == 0 {
		t.Fatalf("appearing button did not raise a structure change: %+v", u)
	}
	accesstest.Find(t, r.bridge.Tree(), access.RoleButton, "Extra")
}

func TestAccessFocusFollowsKeyboardFocus(t *testing.T) {
	var got []access.Update
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(Button("Start"), TextInput(NewSignal("")).Label("Name"))
	})
	r.bridge.Listen(func(u access.Update) { got = append(got, u) })
	r.rt.setFocus(accessIDFocus(t, r, "Name"))
	tr := r.tree()
	accesstest.Focused(t, tr, access.RoleTextField, "Name")
	u := got[len(got)-1]
	if !u.FocusChanged || u.Focus != tr.Focus {
		t.Fatalf("focus change not reported: %+v", u)
	}
}

// accessIDFocus returns the element ID of the text field with the name, for
// setting focus directly.
func accessIDFocus(t *testing.T, r *accessRig, name string) uint64 {
	t.Helper()
	tr := r.tree()
	n := accesstest.Find(t, tr, access.RoleTextField, name)
	for _, nd := range r.rt.byID {
		if accID(nd.id) == n.ID {
			return nd.id
		}
	}
	t.Fatalf("no element for %q", name)
	return 0
}

func TestAccessActionsRunOnTheUIThread(t *testing.T) {
	clicks := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Button("Press").OnClick(func() { clicks++ }),
			TextInput(NewSignal("")).Label("Field"),
		)
	})
	tr := r.tree()
	press := accesstest.Find(t, tr, access.RoleButton, "Press")
	r.bridge.Perform(press.ID, access.ActionClick)
	r.frame()
	if clicks != 1 {
		t.Fatalf("clicks = %d after an invoke, want 1", clicks)
	}
	field := accesstest.Find(t, r.bridge.Tree(), access.RoleTextField, "Field")
	r.bridge.Perform(field.ID, access.ActionFocus)
	r.frame()
	if r.bridge.Tree().Focus != field.ID {
		t.Fatalf("focus did not move to the field:\n%s", r.bridge.Tree().Dump())
	}
	// An action on a node that is gone is ignored.
	r.bridge.Perform(access.ID(12345), access.ActionClick)
	r.frame()
	if clicks != 1 {
		t.Fatalf("an action on a missing node ran: clicks = %d", clicks)
	}
}

func TestAccessLinkInRichTextIsInvokable(t *testing.T) {
	opened := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Rich(Seg("Read the "), Link("guide", func() { opened++ }), Seg(" first")))
	})
	tr := r.tree()
	link := accesstest.Find(t, tr, access.RoleLink, "guide")
	r.bridge.Perform(link.ID, access.ActionClick)
	r.frame()
	if opened != 1 {
		t.Fatalf("link ran %d times, want 1", opened)
	}
}

func TestAccessAnnouncementsAreDelivered(t *testing.T) {
	var got []access.Update
	r := newAccessRig(func(ctx *Context) Element { return Button("Save") })
	r.bridge.Listen(func(u access.Update) { got = append(got, u) })
	Announce("Saved", access.Polite)
	Announce("Failed", access.Assertive)
	r.rt.markAccess()
	r.frame()
	u := got[len(got)-1]
	if len(u.Announcements) != 2 || u.Announcements[0].Text != "Saved" || u.Announcements[1].Politeness != access.Assertive {
		t.Fatalf("announcements %+v", u.Announcements)
	}
	r.rt.markAccess()
	r.frame()
	if n := len(got[len(got)-1].Announcements); n != 0 {
		t.Fatalf("announcements were delivered twice: %d", n)
	}
}

func TestAccessNothingIsBuiltWithoutAnAssistiveTechnology(t *testing.T) {
	text := NewSignal("a")
	rt := NewRuntime(func(ctx *Context) Element {
		return Div().Column().Children(Button("Go"), Text(text.Get()))
	}, DarkTheme, nil)
	rt.SetViewport(500, 400, 1)
	now := time.Unix(1_000_000, 0)
	for i := 0; i < 5; i++ {
		text.Set(fmt.Sprint(i))
		rt.Frame(now)
		now = now.Add(16 * time.Millisecond)
	}
	if rt.acc != nil {
		t.Fatal("the access state was created without an assistive technology asking for it")
	}
	if rt.AccessBridge().Tree() != nil {
		t.Fatal("a tree was published before activation")
	}
}

func TestAccessDumpIsReadable(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(Button("New project").Key("new-project"), TextInput(NewSignal("")).Label("Search"))
	})
	dump := r.rt.AccessTree().Dump()
	for _, want := range []string{`window`, `button "New project"`, `text field "Search"`} {
		if !strings.Contains(dump, want) {
			t.Fatalf("dump lacks %q:\n%s", want, dump)
		}
	}
}

func TestAccessWarningsRunWithoutAnAssistiveTechnology(t *testing.T) {
	rt := NewRuntime(func(ctx *Context) Element {
		return Div().Column().Child(IconButton(icons.Bell))
	}, DarkTheme, nil)
	var warnings []string
	rt.SetWarnFunc(func(s string) { warnings = append(warnings, s) })
	rt.SetAccessWarnings(true)
	rt.SetViewport(500, 400, 1)
	now := time.Unix(1_000_000, 0)
	rt.Frame(now)
	rt.Frame(now.Add(time.Second))
	if len(warnings) != 1 || !strings.Contains(warnings[0], "accessibility: button has no accessible name") {
		t.Fatalf("warnings = %q", warnings)
	}
	if rt.AccessBridge().Active() {
		t.Fatal("warnings activated the bridge")
	}
}
