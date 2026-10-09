package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// The window's minimum size is 760x480 (see main.go). Every page must fit
// that width without overflowing horizontally.
func TestPagesFitMinimumWindowWidth(t *testing.T) {
	for _, page := range []Page{PageDashboard, PageProjects, PageActivity, PageComponents, PageSettings} {
		s := NewStore()
		s.Page.Set(page)
		rt := ui.NewRuntime(App(s), ui.DarkTheme, nil)
		rt.SetViewport(760, 480, 1)
		rt.Frame(time.Now())
		if w := rt.ContentWidth(); w > 760.5 {
			t.Errorf("page %d needs %.0fpx of width at the 760px minimum window size", page, w)
		}
	}
}
