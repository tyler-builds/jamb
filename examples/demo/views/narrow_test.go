package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

// The window's minimum size is 760x480 (see main.go). Every page must fit
// that width without overflowing horizontally, at every Text size the settings
// allow: a larger text size takes more width for the same words.
func TestPagesFitMinimumWindowWidth(t *testing.T) {
	for _, scale := range []int{100, 150, 225} {
		for _, page := range []Page{PageDashboard, PageProjects, PageActivity, PageComponents, PageSettings, PageMarkdown} {
			s := NewStore()
			s.Page.Set(page)
			rt := ui.NewRuntime(App(s), DemoDarkTheme, nil)
			rt.SetSystemSettings(ui.SystemSettings{TextScale: scale})
			rt.SetViewport(760, 480, 1)
			rt.Frame(time.Now())
			if w := rt.ContentWidth(); w > 760.5 {
				t.Errorf("text %d%%: page %d needs %.0fpx of width at the 760px minimum window size", scale, page, w)
			}
		}
	}
}
