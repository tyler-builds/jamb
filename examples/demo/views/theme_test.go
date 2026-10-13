package views

import (
	"reflect"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// TestDemoNeverChangesTheLibraryThemes runs the app, switches its theme both
// ways, and checks that the library's built-in themes are exactly as they were:
// the demo styles its parts through its own copies (DemoDarkTheme and
// DemoLightTheme), not through ui.DarkTheme() or ui.LightTheme().
func TestDemoNeverChangesTheLibraryThemes(t *testing.T) {
	dark, light := *ui.DarkTheme(), *ui.LightTheme()
	rt := ui.NewRuntime(App(NewStore()), DemoDarkTheme, nil)
	rt.Frame(time.Now())
	rt.SetTheme(DemoLightTheme)
	rt.Frame(time.Now())
	rt.SetTheme(DemoDarkTheme)
	rt.Frame(time.Now())
	if !reflect.DeepEqual(dark, *ui.DarkTheme()) {
		t.Errorf("the demo changed ui.DarkTheme()")
	}
	if !reflect.DeepEqual(light, *ui.LightTheme()) {
		t.Errorf("the demo changed ui.LightTheme()")
	}
	if got := DemoDarkTheme.Tabs.Indicator.Height; got != ui.Px(3) {
		t.Errorf("the demo's dark theme indicator height is %v, want 3 px", got)
	}
}
