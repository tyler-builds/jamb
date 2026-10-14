package views

import (
	"strings"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// The clipboard sample copies its image and pastes it back: the headless
// runtime keeps a clipboard of its own, so the system's is not touched.
func TestClipboardSampleRoundTrip(t *testing.T) {
	rt := ui.NewRuntime(func(ctx *ui.Context) ui.Element {
		return ui.Div().ScrollY().Child(clipboardSection(ctx.Theme()))
	}, ui.DarkTheme(), nil)
	rt.SetViewport(1100, 900, 1)
	rt.Frame(time.Now())
	rt.ClickKey("clip-paste")
	var dump strings.Builder
	rt.Dump(&dump, 0)
	if !strings.Contains(dump.String(), "The clipboard holds no image") {
		t.Fatal("Paste with an empty clipboard did not say so")
	}
	rt.ClickKey("clip-copy")
	rt.ClickKey("clip-paste")
	dump.Reset()
	rt.Dump(&dump, 0)
	if !strings.Contains(dump.String(), "Pasted with the button: 200 x 12") && strings.Contains(dump.String(), "image") {
		t.Fatalf("the pasted image was not shown:\n%s", dump.String())
	}
}
