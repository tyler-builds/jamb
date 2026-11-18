package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

var (
	executeQuery = ui.NewAction("test.execute", "Execute")
	parseQuery   = ui.NewAction("test.parse", "Parse")
)

// queryEditor is a menu bar with a Query menu of Execute and Parse, over a focusable
// "editor" in the Editor key context and a focusable "tree" outside it.
func queryEditor(ctx *ui.Context) ui.Element {
	menu := func() []ui.MenuItem {
		return []ui.MenuItem{{Label: "Execute", Action: executeQuery}, {Label: "Parse", Action: parseQuery}}
	}
	return ui.Div().Column().Children(
		ui.MenuBar(ui.TopMenu{Label: "Query", Items: menu}),
		ui.Div().Row().Gap(8).Children(
			ui.Div().Key("tree").Label("Objects").Size(ui.Px(100), ui.Px(100)).Focusable(),
			ui.Div().Key("editor").Label("Query").KeyContext("Editor").Size(ui.Px(200), ui.Px(100)).Focusable().
				OnAction(executeQuery, func() {}).OnAction(parseQuery, func() {}),
		),
	)
}

// An action bound to several keys shows the binding marked Primary, not the one added last.
func TestKeysForPrefersPrimary(t *testing.T) {
	u := uitest.New(t, queryEditor, uitest.Bindings(
		ui.Bind("f5", executeQuery).Primary(), ui.Bind("ctrl+e", executeQuery), ui.Bind("alt+x", executeQuery),
		ui.Bind("ctrl+f5", parseQuery), ui.Bind("alt+p", parseQuery),
	))
	rt := u.Runtime()
	if got := rt.KeysFor(executeQuery); got != "F5" {
		t.Errorf("KeysFor(execute) = %q, want the primary F5", got)
	}
	if got := rt.KeysFor(parseQuery); got != "Alt+P" {
		t.Errorf("KeysFor(parse) = %q, want the last added, Alt+P, with no primary", got)
	}
	// A later Unbind of the primary's keys turns it off, and the rule falls back.
	rt.AddBindings(ui.Bind("f5", ui.Unbind(executeQuery)))
	if got := rt.KeysFor(executeQuery); got != "Alt+X" {
		t.Errorf("KeysFor(execute) with F5 unbound = %q, want Alt+X", got)
	}
}

// A contextual primary binding is shown where its context applies, and the last added
// binding elsewhere. A menu bar's menu, which hangs outside the editor, shows the keys
// that apply where it runs its actions: the element it returns focus to.
func TestKeysForPrimaryInContext(t *testing.T) {
	u := uitest.New(t, queryEditor, uitest.Size(400, 300), uitest.Bindings(
		ui.BindIn("Editor", "ctrl+enter", executeQuery).Primary(), ui.Bind("f5", executeQuery),
	))
	rt := u.Runtime()
	u.Get("tree").Focus()
	if got := rt.KeysFor(executeQuery); got != "F5" {
		t.Errorf("KeysFor outside the editor = %q, want F5", got)
	}
	u.Get("editor").Focus()
	if got := rt.KeysFor(executeQuery); got != "Ctrl+Enter" {
		t.Errorf("KeysFor in the editor = %q, want Ctrl+Enter", got)
	}
	u.Get("editor").Click()
	u.Get("jamb.menubar.0").Click()
	if got := u.Get("jamb.menu.0.0").Text(); got != "Execute Ctrl+Enter" {
		t.Errorf("the Query menu opened from the editor shows %q, want %q", got, "Execute Ctrl+Enter")
	}
	u.Press("escape")
	u.Get("tree").Click()
	u.Get("jamb.menubar.0").Click()
	if got := u.Get("jamb.menu.0.0").Text(); got != "Execute F5" {
		t.Errorf("the Query menu opened from the tree shows %q, want %q", got, "Execute F5")
	}
}
