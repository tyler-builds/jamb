package uitest_test

import (
	"strings"
	"testing"

	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"
)

// A text area with a MaxHeight draws none of its text past its frame, also inside
// a page that scrolls: the window under the frame stays the page's color.
func TestScreenshotTextAreaClipsToMaxHeight(t *testing.T) {
	v := ui.NewSignal(strings.Repeat("WWWW MMMM WWWW MMMM WWWW\n", 30))
	u := uitest.New(t, func(ctx *ui.Context) ui.Element {
		return ui.Div().Grow(1).ScrollY().Background(ui.Hex("#ffffff")).
			Child(ui.TextArea(v).MinRows(1).MaxHeight(ui.Px(80)).Width(ui.Full).Label("Notes"))
	}, uitest.Size(300, 240), uitest.Theme(ui.LightTheme()))
	img := u.Screenshot()
	dark := func(x, y int) bool {
		c := img.RGBAAt(x, y)
		return int(c.R)+int(c.G)+int(c.B) < 3*160
	}
	// The frame is 80 pixels tall at the top of the window, and its text is drawn in it.
	text := 0
	for y := 10; y < 70; y++ {
		for x := 10; x < 290; x++ {
			if dark(x, y) {
				text++
			}
		}
	}
	if text == 0 {
		t.Fatal("no text drawn inside the text area")
	}
	for y := 84; y < 240; y++ {
		for x := 0; x < 300; x++ {
			if dark(x, y) {
				t.Fatalf("a dark pixel at (%d, %d), under the text area's 80-pixel frame: its text overflows", x, y)
			}
		}
	}
}
