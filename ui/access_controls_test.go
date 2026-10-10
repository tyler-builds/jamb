package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/icons"
)

// warningsContaining returns the warnings that contain s.
func warningsContaining(warnings []string, s string) []string {
	var out []string
	for _, w := range warnings {
		if strings.Contains(w, s) {
			out = append(out, w)
		}
	}
	return out
}

func TestAccessTogglesReportTheirState(t *testing.T) {
	clicks := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Div().Role(access.RoleSwitch).Checked(true).Label("Dark mode").OnClick(func() { clicks++ }),
			Div().Role(access.RoleCheckBox).Checked(false).Label("Compact").OnClick(func() { clicks++ }),
			IconButton(icons.Bell).Label("Pin note").Checked(true).OnClick(func() { clicks++ }),
		)
	})
	tr := r.tree()
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleSwitch, "Dark mode"), access.RoleSwitch, "Dark mode",
		access.StateToggle|access.StateChecked)
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleCheckBox, "Compact"), access.RoleCheckBox, "Compact", access.StateToggle)
	pin := accesstest.Find(t, tr, access.RoleButton, "Pin note")
	accesstest.Check(t, pin, access.RoleButton, "Pin note", access.StateToggle|access.StateChecked)
	if !pin.Has(access.ActionToggle) {
		t.Fatalf("the toggle button has no toggle action: %v", pin.Actions)
	}
	r.bridge.Perform(pin.ID, access.ActionToggle)
	r.frame()
	if clicks != 1 {
		t.Fatalf("toggle ran its click %d times, want 1", clicks)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings for stated toggles: %v", r.warnings)
	}
}

func TestAccessToggleWithoutACheckedStateWarns(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Role(access.RoleCheckBox).Label("Choice").OnClick(func() {}))
	})
	r.tree()
	if got := warningsContaining(r.warnings, "has no checked state"); len(got) != 1 {
		t.Fatalf("warnings = %v, want one for the check box without a state", r.warnings)
	}
}

func TestAccessExpandAndCollapseRunTheClickWhenNeeded(t *testing.T) {
	open := NewSignal(false)
	clicks := 0
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Expanded(open.Get()).Child(Text("Build 42")).OnClick(func() {
			clicks++
			open.Set(!open.Peek())
		}))
	})
	row := accesstest.Find(t, r.tree(), access.RoleButton, "Build 42")
	if !row.States.Has(access.StateExpandable) || row.States.Has(access.StateExpanded) {
		t.Fatalf("a closed row has states [%s]", row.States)
	}
	r.bridge.Perform(row.ID, access.ActionExpand)
	r.frame()
	if clicks != 1 || !open.Peek() {
		t.Fatalf("expand ran %d clicks, open = %v", clicks, open.Peek())
	}
	if !accesstest.Find(t, r.tree(), access.RoleButton, "Build 42").States.Has(access.StateExpanded) {
		t.Fatal("the row does not report expanded after Expand")
	}
	// Expanding an open row does nothing, and collapsing it closes it.
	r.bridge.Perform(row.ID, access.ActionExpand)
	r.frame()
	if clicks != 1 {
		t.Fatalf("expand of an open row ran a click (%d)", clicks)
	}
	r.bridge.Perform(row.ID, access.ActionCollapse)
	r.frame()
	if clicks != 2 || open.Peek() {
		t.Fatalf("collapse ran %d clicks, open = %v", clicks, open.Peek())
	}
}

func TestAccessSelectionItemsAndContainers(t *testing.T) {
	sel := NewSignal("All")
	clicks := 0
	chip := func(name, cur string) Element {
		return Div().Role(access.RoleButton).Selected(cur == name).OnClick(func() {
			clicks++
			sel.Set(name)
		}).Child(Text(name))
	}
	r := newAccessRig(func(ctx *Context) Element {
		cur := sel.Get()
		return Div().Label("Status filter").SelectionContainer().Row().Children(chip("All", cur), chip("Active", cur))
	})
	tr := r.tree()
	group := accesstest.Find(t, tr, access.RoleGroup, "Status filter")
	if !group.States.Has(access.StateSelectionContainer) {
		t.Fatalf("the filter group is not a selection container: [%s]", group.States)
	}
	all := accesstest.Find(t, tr, access.RoleButton, "All")
	accesstest.Check(t, all, access.RoleButton, "All", access.StateSelectable|access.StateSelected)
	active := accesstest.Find(t, tr, access.RoleButton, "Active")
	if !active.States.Has(access.StateSelectable) || active.States.Has(access.StateSelected) {
		t.Fatalf("an unchosen chip has states [%s]", active.States)
	}
	r.bridge.Perform(active.ID, access.ActionSelect)
	r.frame()
	if sel.Peek() != "Active" || clicks != 1 {
		t.Fatalf("select chose %q after %d clicks", sel.Peek(), clicks)
	}
	tr = r.tree()
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleButton, "Active"), access.RoleButton, "Active", access.StateSelected)
	// Selecting the chosen item does nothing.
	r.bridge.Perform(active.ID, access.ActionSelect)
	r.frame()
	if clicks != 1 {
		t.Fatalf("select of a chosen item ran a click (%d)", clicks)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings for a filter group: %v", r.warnings)
	}
}

func TestAccessSelectionItemOutsideAContainerWarns(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Role(access.RoleButton).Selected(true).Child(Text("Loose")).OnClick(func() {}))
	})
	r.tree()
	if got := warningsContaining(r.warnings, "outside a selection container"); len(got) != 1 {
		t.Fatalf("warnings = %v, want one for the item outside a container", r.warnings)
	}
}

func TestAccessHeadingLevelAndName(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Heading(2).Child(Text("Appearance")))
	})
	h := accesstest.Find(t, r.tree(), access.RoleHeading, "Appearance")
	if h.Heading != 2 {
		t.Fatalf("heading level %d, want 2", h.Heading)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("Heading(7) did not panic")
		}
	}()
	Div().Heading(7)
}

func TestAccessLabelledByAndDescribedByResolveKeys(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			Text("Name").Key("name-label"),
			Text("Up to 40 characters").Key("name-help"),
			TextInput(NewSignal("")).Label("Name").LabelledBy("name-label").DescribedBy("name-help"),
			Div().Role(access.RoleSwitch).Checked(true).Label("Dark mode").LabelledBy("name-label").OnClick(func() {}),
		)
	})
	tr := r.tree()
	label := accesstest.Find(t, tr, access.RoleStaticText, "Name")
	help := accesstest.Find(t, tr, access.RoleStaticText, "Up to 40 characters")
	field := accesstest.Find(t, tr, access.RoleTextField, "Name")
	if len(field.LabelledBy) != 1 || field.LabelledBy[0] != label.ID {
		t.Fatalf("LabelledBy = %v, want [%d]", field.LabelledBy, label.ID)
	}
	if len(field.DescribedBy) != 1 || field.DescribedBy[0] != help.ID {
		t.Fatalf("DescribedBy = %v, want [%d]", field.DescribedBy, help.ID)
	}
	sw := accesstest.Find(t, tr, access.RoleSwitch, "Dark mode")
	if len(sw.LabelledBy) != 1 || sw.LabelledBy[0] != label.ID {
		t.Fatalf("switch LabelledBy = %v, want [%d]", sw.LabelledBy, label.ID)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestAccessRelationToMissingKeyWarns(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(TextInput(NewSignal("")).Label("Name").LabelledBy("nowhere"))
	})
	r.tree()
	if got := warningsContaining(r.warnings, `LabelledBy key "nowhere" matches no exposed element`); len(got) != 1 {
		t.Fatalf("warnings = %v, want one for the missing key", r.warnings)
	}
}

func TestAccessScrollContainerReportsAndScrolls(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Div().Size(Px(200), Px(100)).ScrollY().Child(Div().Height(Px(400)).Child(Text("Tall content"))))
	})
	scrollerOf := func(tr *access.Tree) *access.Node {
		var s *access.Node
		tr.Walk(func(n *access.Node) bool {
			if n.Role == access.RoleScrollArea {
				s = n
				return false
			}
			return true
		})
		if s == nil {
			t.Fatalf("no scroll area in the tree:\n%s", tr.Dump())
		}
		return s
	}
	s := scrollerOf(r.tree())
	if !s.Scroll.Container || !s.Scroll.CanY || s.Scroll.CanX || s.Scroll.Y != 0 {
		t.Fatalf("scroll state %+v", s.Scroll)
	}
	if s.Scroll.ViewY < 24 || s.Scroll.ViewY > 26 {
		t.Fatalf("visible part %.1f%%, want 25%%", s.Scroll.ViewY)
	}
	r.bridge.Do(s.ID, access.Request{Action: access.ActionSetScrollPercent, X: -1, Y: 50})
	r.frame()
	if got := scrollerOf(r.tree()).Scroll.Y; got < 49.9 || got > 50.1 {
		t.Fatalf("SetScrollPercent(50) left the position at %.1f%%", got)
	}
	// One view down from 50% is 83.3%; a second one reaches the end.
	r.bridge.Do(s.ID, access.Request{Action: access.ActionScroll, X: 0, Y: 1})
	r.frame()
	if got := scrollerOf(r.tree()).Scroll.Y; got < 83.2 || got > 83.4 {
		t.Fatalf("one view down left the position at %.1f%%, want 83.3", got)
	}
	r.bridge.Do(s.ID, access.Request{Action: access.ActionScroll, X: 0, Y: 1})
	r.frame()
	if got := scrollerOf(r.tree()).Scroll.Y; got != 100 {
		t.Fatalf("a scroll past the end left the position at %.1f%%, want 100", got)
	}
}

func TestAccessScrollIntoViewRevealsARow(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		rows := Div().Column()
		for i := 0; i < 20; i++ {
			rows.Child(Div().Height(Px(30)).Role(access.RoleListItem).Child(Text(fmt.Sprintf("Row %d", i+1))))
		}
		return Div().Column().Child(Div().Key("scroller").Size(Px(200), Px(100)).ScrollY().Child(rows))
	})
	row := accesstest.Find(t, r.tree(), access.RoleListItem, "Row 15")
	if !row.Has(access.ActionScrollIntoView) {
		t.Fatalf("a row in a scroll container cannot be scrolled into view: %v", row.Actions)
	}
	r.bridge.Perform(row.ID, access.ActionScrollIntoView)
	r.frame()
	// Row 15 spans 420 to 450; the view is 100 high, so the offset is 350.
	if got := r.nodeByKey(t, "scroller").st.targetY; got != 350 {
		t.Fatalf("scroll target %v after ScrollIntoView, want 350", got)
	}
}

func TestAccessTextFieldsExposeAndSetTheirValue(t *testing.T) {
	text := NewSignal("a")
	var changes []string
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Children(
			TextInput(text).Label("Search").OnChange(func(s string) { changes = append(changes, s) }),
			TextArea(NewSignal("body")).Label("Notes"),
		)
	})
	tr := r.tree()
	field := accesstest.Find(t, tr, access.RoleTextField, "Search")
	if field.Value != "a" || !field.Has(access.ActionSetValue) {
		t.Fatalf("field value %q, actions %v", field.Value, field.Actions)
	}
	if notes := accesstest.Find(t, tr, access.RoleTextArea, "Notes"); notes.Value != "body" {
		t.Fatalf("text area value %q, want body", notes.Value)
	}
	r.bridge.Do(field.ID, access.Request{Action: access.ActionSetValue, Value: "hello"})
	r.frame()
	if text.Peek() != "hello" {
		t.Fatalf("the signal holds %q after SetValue", text.Peek())
	}
	if len(changes) != 1 || changes[0] != "hello" {
		t.Fatalf("OnChange saw %v, want [hello]", changes)
	}
	if got := accesstest.Find(t, r.tree(), access.RoleTextField, "Search").Value; got != "hello" {
		t.Fatalf("the tree reports %q after SetValue", got)
	}
}

func TestAccessDropdownMenuIsOneExpandableControl(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(DropdownMenu(Button("Project actions").Secondary(), func() []MenuItem {
			return []MenuItem{{Label: "Open", OnClick: func() {}}}
		}))
	})
	tr := r.tree()
	count := 0
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleButton && n.Name == "Project actions" {
			count++
		}
		return true
	})
	if count != 1 {
		t.Fatalf("%d buttons named Project actions, want the trigger once:\n%s", count, tr.Dump())
	}
	btn := accesstest.Find(t, tr, access.RoleButton, "Project actions")
	if !btn.States.Has(access.StateExpandable) || btn.States.Has(access.StateExpanded) {
		t.Fatalf("a closed trigger has states [%s]", btn.States)
	}
	r.bridge.Perform(btn.ID, access.ActionExpand)
	r.frame()
	tr = r.tree()
	accesstest.Find(t, tr, access.RoleMenuItem, "Open")
	if !accesstest.Find(t, tr, access.RoleButton, "Project actions").States.Has(access.StateExpanded) {
		t.Fatal("the trigger does not report that its menu is open")
	}
	r.bridge.Perform(btn.ID, access.ActionCollapse)
	r.frame()
	accesstest.Absent(t, r.tree(), access.RoleMenuItem, "Open")
}

func TestAccessSubmenuExpandsAndCollapses(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Key("target").Size(Full, Full).ContextMenu(func() []MenuItem {
			return []MenuItem{{Label: "Status", Submenu: []MenuItem{
				{Label: "Active", OnClick: func() {}}, {Label: "Done", OnClick: func() {}}}}}
		}).Child(Text("Body"))
	})
	r.rt.openContextMenu(r.nodeByKey(t, "target"), 20, 20, false)
	status := accesstest.Find(t, r.tree(), access.RoleMenuItem, "Status")
	if !status.States.Has(access.StateExpandable) || status.States.Has(access.StateExpanded) {
		t.Fatalf("a closed submenu item has states [%s]", status.States)
	}
	accesstest.Absent(t, r.tree(), access.RoleMenuItem, "Active")
	r.bridge.Perform(status.ID, access.ActionExpand)
	r.frame()
	accesstest.Find(t, r.tree(), access.RoleMenuItem, "Active")
	if !accesstest.Find(t, r.tree(), access.RoleMenuItem, "Status").States.Has(access.StateExpanded) {
		t.Fatal("the submenu item does not report expanded")
	}
	r.bridge.Perform(status.ID, access.ActionCollapse)
	r.frame()
	accesstest.Absent(t, r.tree(), access.RoleMenuItem, "Active")
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestAccessRadioMenuItemsAreSelectionItems(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Key("target").Size(Full, Full).ContextMenu(func() []MenuItem {
			return []MenuItem{
				{Label: "Active", Check: MenuCheckRadio, Checked: true, OnClick: func() {}},
				{Label: "Done", Check: MenuCheckRadio, OnClick: func() {}},
			}
		}).Child(Text("Body"))
	})
	r.rt.openContextMenu(r.nodeByKey(t, "target"), 20, 20, false)
	tr := r.tree()
	accesstest.Check(t, accesstest.Find(t, tr, access.RoleMenuItemRadio, "Active"), access.RoleMenuItemRadio, "Active",
		access.StateSelectable|access.StateSelected)
	done := accesstest.Find(t, tr, access.RoleMenuItemRadio, "Done")
	if !done.States.Has(access.StateSelectable) || done.States.Has(access.StateSelected) {
		t.Fatalf("an unchosen radio item has states [%s]", done.States)
	}
	menu := accesstest.Find(t, tr, access.RoleMenu, "Actions")
	if !menu.States.Has(access.StateSelectionContainer) {
		t.Fatalf("a menu of radio items is not a selection container: [%s]", menu.States)
	}
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestAccessListRowsWithTheirOwnRoleAreTheItems(t *testing.T) {
	sel := NewSignal(2)
	r := newAccessRig(func(ctx *Context) Element {
		items := make([]int, 10)
		for i := range items {
			items[i] = i
		}
		return Div().Size(Full, Px(120)).Child(ListOf(items, func(i int) string { return fmt.Sprint(i) },
			func(i int) Element {
				return Div().Role(access.RoleListItem).Selected(i == sel.Get()).OnClick(func() { sel.Set(i) }).
					Child(Text(fmt.Sprintf("Note %d", i+1)))
			}).SelectionContainer().Style(Style{Height: Px(120)}))
	})
	tr := r.tree()
	note := accesstest.Find(t, tr, access.RoleListItem, "Note 3")
	accesstest.Check(t, note, access.RoleListItem, "Note 3", access.StateSelectable|access.StateSelected)
	if note.PosInSet != 3 || note.SetSize != 10 {
		t.Fatalf("note 3 is %d of %d, want 3 of 10", note.PosInSet, note.SetSize)
	}
	tr.Walk(func(n *access.Node) bool {
		if n.Role == access.RoleListItem && n.Parent != 0 {
			if p := tr.Node(n.Parent); p != nil && p.Role == access.RoleListItem {
				t.Fatalf("list item %q is nested in another list item", n.Name)
			}
		}
		return true
	})
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

func TestAccessEventsDeriveFromTheSnapshot(t *testing.T) {
	dark := NewSignal(false)
	open := NewSignal(false)
	r := newAccessRig(func(ctx *Context) Element {
		d := Div().Column().Child(Div().Role(access.RoleSwitch).Checked(dark.Get()).Label("Dark mode").
			OnClick(func() { dark.Set(!dark.Peek()) }))
		if open.Get() {
			d.Child(Modal("Confirm", Text("Sure?")))
		}
		return d
	})
	var updates []access.Update
	r.bridge.Listen(func(u access.Update) { updates = append(updates, u) })
	last := func() access.Update { return updates[len(updates)-1] }

	sw := accesstest.Find(t, r.tree(), access.RoleSwitch, "Dark mode")
	r.bridge.Perform(sw.ID, access.ActionToggle)
	r.frame()
	if u := last(); len(u.Properties) != 1 || u.Properties[0] != (access.PropertyChange{ID: sw.ID, Property: access.PropertyToggle}) {
		t.Fatalf("toggling reported %v, want one toggle change for the switch", u.Properties)
	}

	open.Set(true)
	r.frame()
	u := last()
	dialog := accesstest.Find(t, r.bridge.Tree(), access.RoleDialog, "Confirm")
	if len(u.Opened) != 1 || u.Opened[0].ID != dialog.ID {
		t.Fatalf("opening the dialog reported Opened %v, want [%d]", u.Opened, dialog.ID)
	}
	open.Set(false)
	r.frame()
	if u := last(); len(u.Closed) != 1 || u.Closed[0].ID != dialog.ID {
		t.Fatalf("closing the dialog reported Closed %v, want [%d]", u.Closed, dialog.ID)
	}
}

// A DropdownMenu's trigger takes focus through the wrapper that assistive technology
// sees: the wrapper is focusable, and the focus action moves focus to the button.
func TestAccessDropdownTriggerTakesFocus(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(DropdownMenu(Button("Project actions").Secondary(), func() []MenuItem {
			return []MenuItem{{Label: "Open", OnClick: func() {}}}
		}))
	})
	btn := accesstest.Find(t, r.tree(), access.RoleButton, "Project actions")
	if !btn.States.Has(access.StateFocusable) {
		t.Fatalf("the trigger is not focusable for assistive technology: [%s]", btn.States)
	}
	r.bridge.Perform(btn.ID, access.ActionFocus)
	r.frame()
	if tr := r.tree(); tr.Focus != btn.ID {
		t.Fatalf("focus is on node %d after Focus, want the trigger %d:\n%s", tr.Focus, btn.ID, tr.Dump())
	}
}

// A disabled button is exposed with its Invoke pattern, but a request on it does
// nothing: the element cannot be operated.
func TestAccessDisabledElementRefusesRequests(t *testing.T) {
	clicked := false
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().Child(Button("Create project").Disabled(true).OnClick(func() { clicked = true }))
	})
	btn := accesstest.Find(t, r.tree(), access.RoleButton, "Create project")
	if !btn.States.Has(access.StateDisabled) {
		t.Fatalf("the button is not disabled: [%s]", btn.States)
	}
	r.bridge.Perform(btn.ID, access.ActionClick)
	r.frame()
	if clicked {
		t.Fatal("a click request ran the handler of a disabled button")
	}
}

// Each menu panel has a name, since it takes keyboard focus: a dropdown menu has
// its trigger's name, and a menu bar's popup has its menu's name.
func TestAccessMenuPanelsAreNamed(t *testing.T) {
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().
			Child(DropdownMenu(Button("Project actions").Secondary(), func() []MenuItem {
				return []MenuItem{{Label: "Open", OnClick: func() {}}}
			})).
			Child(MenuBar(TopMenu{Label: "File", Items: func() []MenuItem {
				return []MenuItem{{Label: "Save", OnClick: func() {}}}
			}}))
	})
	btn := accesstest.Find(t, r.tree(), access.RoleButton, "Project actions")
	r.bridge.Perform(btn.ID, access.ActionExpand)
	r.frame()
	accesstest.Find(t, r.tree(), access.RoleMenu, "Project actions")
	r.bridge.Perform(btn.ID, access.ActionCollapse)
	r.frame()
	if err := r.rt.ClickKey("jamb.menubar.0"); err != nil {
		t.Fatal(err)
	}
	r.frame()
	accesstest.Find(t, r.tree(), access.RoleMenu, "File")
	if len(r.warnings) != 0 {
		t.Fatalf("warnings: %v", r.warnings)
	}
}

// A context menu is named by the element it hangs from, or "Actions" when that
// element has no name.
func TestAccessContextMenuIsNamedByItsElement(t *testing.T) {
	menu := func() []MenuItem { return []MenuItem{{Label: "Open", OnClick: func() {}}} }
	r := newAccessRig(func(ctx *Context) Element {
		return Div().Column().
			Child(Div().Key("card").Role(access.RoleListItem).Label("Project Alpha").Size(Full, Px(40)).ContextMenu(menu)).
			Child(Div().Key("plain").Size(Full, Px(40)).ContextMenu(menu))
	})
	r.rt.openContextMenu(r.nodeByKey(t, "card"), 20, 20, false)
	accesstest.Find(t, r.tree(), access.RoleMenu, "Project Alpha")
	r.rt.closeContextMenu(false)
	r.frame()
	r.rt.openContextMenu(r.nodeByKey(t, "plain"), 20, 60, false)
	accesstest.Find(t, r.tree(), access.RoleMenu, "Actions")
}
