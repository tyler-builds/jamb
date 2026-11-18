package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// A toolbar enables Execute while the query editor is focused. It reads the editor's
// handle in its render, before the editor is built, and re-renders when focus comes and
// goes, by a click or by Tab, and only then.
func TestFocusHandleFocusedIsReactive(t *testing.T) {
	editor := ui.NewFocusHandle()
	query := ui.NewSignal("select 1")
	renders := 0
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Children(
			ui.Component(func(ctx *ui.Context) ui.Element {
				renders++
				return ui.Div().Row().Children(
					ui.Button("Execute").Key("exec").Disabled(!editor.Focused()),
					ui.Text(map[bool]string{false: "out", true: "in"}[editor.FocusWithin()]).Key("within"),
				)
			}),
			ui.TextArea(query).Key("editor").Label("Query").FocusHandle(editor),
			ui.Button("Other").Key("other"),
			ui.Button("Another").Key("another"),
		)
	}, uitest.Size(400, 300))
	if !u.Get("exec").Disabled() {
		t.Fatal("Execute is enabled with nothing focused")
	}
	u.Get("editor").Click()
	if u.Get("exec").Disabled() || u.Get("within").Text() != "in" {
		t.Errorf("after a click in the editor: Execute disabled %v, focus within %q; want enabled, in",
			u.Get("exec").Disabled(), u.Get("within").Text())
	}
	u.Get("other").Click()
	if !u.Get("exec").Disabled() || u.Get("within").Text() != "out" {
		t.Errorf("after a click elsewhere: Execute disabled %v, focus within %q; want disabled, out",
			u.Get("exec").Disabled(), u.Get("within").Text())
	}
	before := renders
	u.Press("tab") // from Other to Another: the editor's focus does not change
	if renders != before {
		t.Errorf("a focus change that leaves the editor out re-rendered the toolbar %d times", renders-before)
	}
	editor.Focus()
	u.Frame()
	if u.Get("exec").Disabled() {
		t.Error("after editor.Focus(): Execute is disabled")
	}
}

// FocusWithin follows focus into an element's descendants.
func TestFocusHandleFocusWithin(t *testing.T) {
	pane := ui.NewFocusHandle()
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Children(
			ui.Component(func(ctx *ui.Context) ui.Element {
				return ui.Text(map[bool]string{false: "pane out", true: "pane in"}[pane.FocusWithin()]).Key("status")
			}),
			ui.Div().Key("pane").Label("Results").FocusHandle(pane).TabIndex(-1).Child(ui.Button("Copy").Key("copy")),
			ui.Button("Other").Key("other"),
		)
	}, uitest.Size(400, 300))
	u.Get("copy").Click()
	if got := u.Get("status").Text(); got != "pane in" {
		t.Errorf("focus on a button inside the pane: %q, want %q", got, "pane in")
	}
	u.Get("other").Click()
	if got := u.Get("status").Text(); got != "pane out" {
		t.Errorf("focus outside the pane: %q, want %q", got, "pane out")
	}
}

// ctx.Focused reports focus that a click gave, which KeyboardFocus does not, and
// re-renders the component when it changes.
func TestContextFocused(t *testing.T) {
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Children(
			ui.Component(func(ctx *ui.Context) ui.Element {
				label := "idle"
				if ctx.Focused() {
					label = "focused"
				}
				return ui.Div().Key("cell").Label("Cell").Size(ui.Px(100), ui.Px(40)).Focusable().
					Child(ui.Text(label).Key("cell-text"))
			}),
			ui.Button("Other").Key("other"),
		)
	}, uitest.Size(300, 200))
	u.Get("cell").Click()
	if got := u.Get("cell-text").Text(); got != "focused" {
		t.Errorf("after a click: %q, want focused", got)
	}
	u.Get("other").Click()
	if got := u.Get("cell-text").Text(); got != "idle" {
		t.Errorf("after a click elsewhere: %q, want idle", got)
	}
}
