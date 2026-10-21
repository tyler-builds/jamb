package views

import (
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui/uitest"
)

// BenchmarkThemeSwitch switches the demo between its dark and light themes on
// the Components page, the largest page, and runs a frame after each switch.
// Every component is rendered again and the page is laid out and painted in
// the new theme.
func BenchmarkThemeSwitch(b *testing.B) {
	s := NewStore()
	u := uitest.New(b, App(s), uitest.Size(1200, 760), uitest.Theme(DemoDarkTheme))
	s.Page.Set(PageComponents)
	u.Frame()
	rt := u.Runtime()
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		if i%2 == 0 {
			rt.SetTheme(DemoLightTheme)
		} else {
			rt.SetTheme(DemoDarkTheme)
		}
		u.Frame()
	}
	b.ReportMetric(float64(rt.Stats().Nodes), "nodes")
}

// BenchmarkPageSwitch goes from one page of the demo to the next, cycling
// through all six, and runs a frame after each switch. The first frame of a page
// builds its tree, so this is what a reader waits for after a click on the
// sidebar.
func BenchmarkPageSwitch(b *testing.B) {
	pages := []Page{PageDashboard, PageProjects, PageActivity, PageComponents, PageSettings, PageMarkdown}
	s := NewStore()
	u := uitest.New(b, App(s), uitest.Size(1200, 760), uitest.Theme(DemoDarkTheme))
	u.Frame()
	rt := u.Runtime()
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		s.Page.Set(pages[i%len(pages)])
		u.Advance(16 * time.Millisecond)
	}
	b.ReportMetric(float64(rt.Stats().Nodes), "nodes")
}
