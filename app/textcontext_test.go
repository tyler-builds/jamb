package app

import (
	"testing"

	"github.com/tyler-builds/jamb/internal/platform"
	"github.com/tyler-builds/jamb/internal/uihost"
	"github.com/tyler-builds/jamb/ui"
)

// The runtime numbers a field's purpose and its Enter key as the platform does,
// so textContext passes them on as they are. A masked field's purpose, which
// ui does not export, is 2.
func TestTextContextNumbering(t *testing.T) {
	for _, c := range []struct {
		ui ui.TextPurpose
		p  platform.TextPurpose
	}{
		{ui.PurposeText, platform.PurposeText}, {ui.PurposeVerbatim, platform.PurposeVerbatim},
		{2, platform.PurposePassword}, {ui.PurposeEmail, platform.PurposeEmail}, {ui.PurposeURL, platform.PurposeURL},
		{ui.PurposeNumber, platform.PurposeNumber}, {ui.PurposeDecimal, platform.PurposeDecimal},
		{ui.PurposeTel, platform.PurposeTel}, {ui.PurposeSearch, platform.PurposeSearch},
	} {
		if got := textContext(uihost.TextContext{Purpose: uint8(c.ui)}).Purpose; got != c.p {
			t.Errorf("ui's purpose %d is the platform's %d, want %d", c.ui, got, c.p)
		}
	}
	// ui's own numbering of the Enter keys (see ui/textcontext.go).
	for i, k := range []platform.EnterKey{platform.EnterNewLine, platform.EnterDone, platform.EnterGo,
		platform.EnterSend, platform.EnterSearch} {
		if int(k) != i {
			t.Errorf("the platform numbers Enter key %d as %d", i, k)
		}
	}
	c := textContext(uihost.TextContext{Focused: true, Text: "abc", Anchor: 1, Caret: 2, HasText: true, Multiline: true, Enter: 3})
	if !c.Focused || c.Text != "abc" || c.Anchor != 1 || c.Caret != 2 || !c.HasText || !c.Multiline || c.Enter != platform.EnterSend {
		t.Errorf("textContext gave %+v", c)
	}
}
