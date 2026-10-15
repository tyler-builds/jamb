//go:build windows

package windows

import (
	"syscall"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/access/accesstest"
	"github.com/tyler-builds/jamb/internal/win32"
	"github.com/tyler-builds/jamb/ui"
)

// ownModel is a text model of an element's own, as an editor outside ui has (codeedit's
// editorModel): it is not an access.TextContent, so the Text pattern reads it only
// through access.TextModel. This test does not import codeedit, which would add the
// editor's dependencies to the module graph of every app that imports this package.
type ownModel struct{ *access.TextContent }

// TestOwnTextModelSelectionFromTextPattern selects text in an element with its own text
// model the way a screen reader does, through UI Automation's Text pattern: it finds a
// word in the document range and calls Select on the range found. The request goes
// through the runtime's access bridge to the element's select hook on the next frame.
// codeedit's TestAssistiveSelectionSetsPrimarySelection covers the hook's side.
func TestOwnTextModelSelectionFromTextPattern(t *testing.T) {
	const s = "one two three"
	anchor, caret := -1, -1
	rt := ui.NewRuntime(func(*ui.Context) ui.Element {
		return ui.Div().Child(ui.Div().Role(access.RoleTextArea).Label("Source").Focusable().
			Size(ui.Px(480), ui.Px(300)).
			TextAccess(ui.TextAccess{
				Info:   func() *access.TextInfo { return &access.TextInfo{Content: ownModel{asciiText(s)}} },
				Select: func(a, c int) { anchor, caret = a, c },
			}))
	}, ui.DarkTheme(), nil)
	rt.SetViewport(800, 600, 1)
	now := time.Unix(1_000_000, 0)
	frame := func() { now = now.Add(16 * time.Millisecond); rt.Frame(now) }
	b := rt.AccessBridge()
	b.Activate()
	frame()
	field := accesstest.Find(t, b.Tree(), access.RoleTextArea, "Source")
	if field.Text == nil {
		t.Fatal("the element has no text for the Text pattern")
	}
	if _, ok := field.Text.Content.(*access.TextContent); ok {
		t.Fatal("the element's text is a copied TextContent, want its own model")
	}

	p := patternOn(t, b, field.ID, patternText)
	all := rangeOut(t, comMethod(p, 7), p) // DocumentRange
	needle := bstrOf("two")
	defer win32.SysFreeString(needle)
	found, hr := getOut(comMethod(all, 8), all, needle, 0, 0) // FindText, forward, matching case
	if hr != hrOK || found == 0 {
		t.Fatalf("FindText failed: HRESULT 0x%08X", uint32(hr))
	}
	defer releaseCall(found)
	if got := rangeText(t, found, -1); got != "two" {
		t.Fatalf("FindText found %q, want two", got)
	}
	if hr, _, _ := syscall.SyscallN(comMethod(found, 16), found); hr != hrOK { // Select
		t.Fatalf("Select failed: HRESULT 0x%08X", hr)
	}
	frame()
	if anchor != 4 || caret != 7 {
		t.Fatalf("the select hook got %d to %d, want 4 to 7", anchor, caret)
	}
}
