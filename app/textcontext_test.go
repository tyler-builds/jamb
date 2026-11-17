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
		if got := textContext(uihost.TextContext{Purpose: uint8(c.ui)}, 1).Purpose; got != c.p {
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
	c := textContext(uihost.TextContext{Focused: true, Text: "abc", Anchor: 1, Caret: 2, HasText: true, Multiline: true, Enter: 3}, 1)
	if !c.Focused || c.Text != "abc" || c.Anchor != 1 || c.Caret != 2 || !c.HasText || !c.Multiline || c.Enter != platform.EnterSend {
		t.Errorf("textContext gave %+v", c)
	}
}

// The field's geometry goes to the platform in physical pixels.
func TestTextContextGeometry(t *testing.T) {
	c := textContext(uihost.TextContext{Focused: true, Box: [4]float32{10, 20, 100, 40}, TextX: 10, TextY: 5,
		FontSize: 14, LineHeight: 18}, 2)
	if c.Box != (platform.Rect{X: 20, Y: 40, W: 200, H: 80}) || c.TextX != 20 || c.TextY != 10 || c.FontSize != 28 || c.LineHeight != 36 {
		t.Errorf("textContext at scale 2 gave box %+v, text at %v,%v, size %v, line height %v", c.Box, c.TextX, c.TextY, c.FontSize, c.LineHeight)
	}
}
