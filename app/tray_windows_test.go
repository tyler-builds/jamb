//go:build windows

package app

import (
	"reflect"
	"testing"

	"github.com/tyler-builds/jamb/internal/platform/windows"
)

// TestMenuItems checks the native items of a tray menu: IDs count from 1 over the
// items that run an action, separators and submenu items take none, and each
// action is kept under its ID.
func TestMenuItems(t *testing.T) {
	var ran []string
	act := func(name string) func() { return func() { ran = append(ran, name) } }
	items := []TrayItem{
		{Label: "Show window", OnClick: act("show")},
		TraySeparator(),
		{Label: "Keep running", Checked: true, OnClick: act("keep")},
		{Label: "Export", Submenu: []TrayItem{
			{Label: "CSV", OnClick: act("csv")},
			{Label: "Off", Disabled: true},
		}},
		{Label: "Quit", OnClick: act("quit")},
	}
	actions := map[uint32]func(){}
	var last uint32
	got := menuItems(items, &last, actions)

	want := []windows.MenuItem{
		{ID: 1, Label: "Show window"},
		{Separator: true},
		{ID: 2, Label: "Keep running", Checked: true},
		{Label: "Export", Submenu: []windows.MenuItem{
			{ID: 3, Label: "CSV"},
			{ID: 4, Label: "Off", Disabled: true},
		}},
		{ID: 5, Label: "Quit"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("menuItems = %+v;\nwant %+v", got, want)
	}
	if last != 5 {
		t.Errorf("last ID = %d; want 5", last)
	}
	if len(actions) != 5 {
		t.Errorf("%d actions; want 5", len(actions))
	}
	for _, id := range []uint32{1, 2, 3, 5} {
		if actions[id] == nil {
			t.Errorf("no action for ID %d", id)
		}
	}
	actions[3]()
	actions[5]()
	if !reflect.DeepEqual(ran, []string{"csv", "quit"}) {
		t.Errorf("actions ran %v; want [csv quit]", ran)
	}
}
