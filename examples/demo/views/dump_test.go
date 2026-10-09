package views

import (
	"os"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/ui"
)

func TestDumpDashboard(t *testing.T) {
	s := NewStore()
	s.Page.Set(PageDashboard)
	rt := ui.NewRuntime(App(s), ui.DarkTheme, nil)
	rt.SetViewport(1200, 760, 1)
	rt.Frame(time.Now())
	rt.Dump(os.Stdout, 7)
}
