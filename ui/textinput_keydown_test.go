package ui_test

import (
	"testing"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// A field's OnKeyDown runs on a key press before the field handles the key: here the
// field's Enter (its OnSubmit) still runs when the handler lets the key through.
func TestTextInputOnKeyDownRunsBeforeTheFieldsKey(t *testing.T) {
	var seen []input.Key
	submits := 0
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Child(ui.TextInput(ui.NewSignal("")).Key("in").Label("Search").
			OnKeyDown(func(e *ui.KeyEvent) { seen = append(seen, e.Key) }).
			OnSubmit(func(string) { submits++ }))
	})
	u.Get("in").Focus()
	u.Press("enter")
	if len(seen) != 1 || seen[0] != input.KeyEnter {
		t.Errorf("OnKeyDown saw %v, want one Enter", seen)
	}
	if submits != 1 {
		t.Errorf("OnSubmit ran %d times, want 1 when the key is not stopped", submits)
	}
}

// StopPropagation in OnKeyDown keeps the field from handling the key, so the field's
// Enter does not submit. An app uses it for a key the field would otherwise take.
func TestTextInputOnKeyDownStopKeepsTheFieldFromTheKey(t *testing.T) {
	submits, previous := 0, 0
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Column().Child(ui.TextInput(ui.NewSignal("")).Key("in").Label("Search").
			OnKeyDown(func(e *ui.KeyEvent) {
				if e.Key == input.KeyEnter && e.Mods.Has(input.ModShift) {
					previous++
					e.StopPropagation()
				}
			}).
			OnSubmit(func(string) { submits++ }))
	})
	u.Get("in").Focus()
	u.Press("shift+enter")
	if previous != 1 {
		t.Errorf("Shift+Enter ran the handler %d times, want 1", previous)
	}
	if submits != 0 {
		t.Errorf("the stopped key still reached the field: %d submits", submits)
	}
	u.Press("enter")
	if submits != 1 {
		t.Errorf("a plain Enter submits %d times, want 1", submits)
	}
}
