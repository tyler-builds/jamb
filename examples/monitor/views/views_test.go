package views

import (
	"image"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/access"
	"github.com/tyler-builds/jamb/ui"
	"github.com/tyler-builds/jamb/ui/uitest"

	"github.com/tyler-builds/jamb/examples/monitor/sample"
	"github.com/tyler-builds/jamb/examples/monitor/store"
)

// open renders the app over a made-up machine that has been sampled three times.
func open(t *testing.T) (*uitest.UI, *App, *store.Store, *sample.Synthetic) {
	t.Helper()
	src := sample.NewSynthetic(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), time.Second)
	s := store.New(src, time.Second)
	for range 3 {
		s.Step()
	}
	a := New(s, Layout{})
	u := uitest.New(t, a.Root(), uitest.Size(1360, 860), uitest.Bindings(Bindings()...))
	return u, a, s, src
}

// proc returns the key of the first process named name in the latest sample.
func proc(t *testing.T, s *store.Store, name string) string {
	t.Helper()
	for _, p := range s.Latest.Peek().Procs {
		if p.Name == name {
			return p.Key
		}
	}
	t.Fatalf("no process %q", name)
	return ""
}

func TestOverviewShowsTheLatestSample(t *testing.T) {
	u, _, s, _ := open(t)
	snap := s.Latest.Peek()
	if got, want := u.Get("tile-cpu").Text(), percent(snap.CPU); !strings.Contains(got, want) {
		t.Errorf("the processor tile shows %q, want it to contain %q", got, want)
	}
	if got := u.Get("tile-memory").Text(); !strings.Contains(got, "of 16.0 GB") {
		t.Errorf("the memory tile shows %q, want the installed memory", got)
	}
	s.Step()
	u.Frame()
	if got, want := u.Get("tile-cpu").Text(), percent(s.Latest.Peek().CPU); !strings.Contains(got, want) {
		t.Errorf("after a sample the processor tile shows %q, want %q", got, want)
	}
	if w := u.Warnings(); len(w) > 0 {
		t.Errorf("warnings: %q", w)
	}
}

func TestSelectionStaysOnItsProcessAsTheRowsMove(t *testing.T) {
	u, a, s, _ := open(t)
	key := proc(t, s, "editor.exe")
	a.selectProcess(key)
	u.Frame()
	order := func() []string {
		var keys []string
		procs := slices.Clone(s.Latest.Peek().Procs)
		slices.SortStableFunc(procs, func(x, y sample.Process) int { return -strings.Compare(percent(x.CPU), percent(y.CPU)) })
		for _, p := range procs {
			keys = append(keys, p.Key)
		}
		return keys
	}
	before := order()
	moved := false
	for range 12 {
		s.Step()
		u.Frame()
		if !slices.Equal(order(), before) {
			moved = true
		}
	}
	if !moved {
		t.Fatal("the busiest processes never changed places, so the test shows nothing")
	}
	if sel := a.selected.Peek(); len(sel) != 1 || sel[0] != key {
		t.Errorf("selection = %v, want the editor %s", sel, key)
	}
	if got := u.Get("detail-name").Text(); got != "Editor" {
		t.Errorf("the details show %q, want the program's name, Editor", got)
	}
}

func TestFilterNarrowsTheTable(t *testing.T) {
	u, _, _, _ := open(t)
	u.Press("ctrl+f")
	u.Type("browser")
	if got := u.Get("filter").Text(); !strings.Contains(got, "3 found") {
		t.Errorf("filter = %q, want 3 found", got)
	}
	grid := u.ByRole(access.RoleDataGrid, "Processes")
	if n := grid.Node().GridRows; n != 3 {
		t.Errorf("%d rows, want the 3 browser processes", n)
	}
	u.Type(" 34")
	if n := u.ByRole(access.RoleDataGrid, "Processes").Node().GridRows; n != 2 {
		t.Errorf("%d rows for 'browser 34', want 2 (PIDs 3488 and 3492)", n)
	}
}

func TestEndProcessAsksFirst(t *testing.T) {
	u, a, s, _ := open(t)
	key := proc(t, s, "build.exe")
	a.selectProcess(key)
	u.Frame()
	u.Get("end").Click()
	u.ByRole(access.RoleButton, "Cancel").Click()
	s.Step()
	u.Frame()
	if _, ok := s.Process(key); !ok {
		t.Fatal("Cancel ended the process")
	}

	u.Get("end").Click()
	u.ByRole(access.RoleButton, "End process").Click()
	s.Step()
	u.Frame()
	if _, ok := s.Process(key); ok {
		t.Fatal("the process is still running after End process")
	}
	if !strings.Contains(u.Get("dock").Text(), "The process has exited") {
		t.Errorf("the details do not say the process exited")
	}
}

func TestDeleteInTheTableAsksToEndTheSelectedProcess(t *testing.T) {
	u, a, s, _ := open(t)
	a.selectProcess(proc(t, s, "terminal.exe"))
	u.Frame()
	u.ByRole(access.RoleDataGrid, "Processes").Focus()
	u.Press("delete")
	u.ByName("End Terminal?")
}

func TestAlertIsRaisedWhenTheLoadStaysHigh(t *testing.T) {
	u, _, s, src := open(t)
	s.Rules.Set(store.Rules{CPU: store.Rule{On: true, Threshold: 90, For: 3}})
	src.Spike = 100
	for range 5 {
		s.Step()
	}
	u.Frame()
	alerts := s.Alerts.Peek()
	if len(alerts) != 1 || alerts[0].Kind != store.AlertCPU {
		t.Fatalf("alerts = %+v, want one processor alert", alerts)
	}
	if !slices.Contains(s.Firing.Peek(), store.AlertCPU) {
		t.Errorf("firing = %v, want cpu", s.Firing.Peek())
	}
	if !u.Has("cpu-firing") {
		t.Errorf("the processor rule is not marked as above its limit")
	}
	if got := u.Get("alert-log").Text(); !strings.Contains(got, "above 90% for") {
		t.Errorf("alert log = %q", got)
	}
	src.Spike = 0
	s.Step()
	u.Frame()
	if len(s.Firing.Peek()) != 0 || u.Has("cpu-firing") {
		t.Errorf("the rule still fires after the load fell")
	}
}

func TestShowPanelOpensAClosedPanelAgain(t *testing.T) {
	u, a, _, _ := open(t)
	a.layout.Dock.Set(a.layout.Dock.Peek().Close(panelDetails))
	u.Frame()
	if a.layout.Dock.Peek().Has(panelDetails) {
		t.Fatal("the details did not close")
	}
	u.Press("ctrl+6")
	if !a.layout.Dock.Peek().Has(panelDetails) {
		t.Error("Ctrl+6 did not open the details again")
	}
}

func TestPauseStopsTheTitleBarFromSampling(t *testing.T) {
	u, _, s, _ := open(t)
	u.Press("ctrl+p")
	if !s.Paused.Peek() {
		t.Fatal("Ctrl+P did not pause")
	}
	if got := u.ByRole(access.RoleButton, "Resume sampling"); !got.Checked() {
		t.Errorf("the pause button is not shown as pressed")
	}
	u.Press("ctrl+p")
	if s.Paused.Peek() {
		t.Error("Ctrl+P did not resume")
	}
}

// browserGroup is the key of the group of the made-up browser's processes.
const browserGroup = `group:c:\programs\browser.exe`

func TestProcessesAreGroupedByProgram(t *testing.T) {
	u, a, s, _ := open(t)
	grid := u.ByRole(access.RoleDataGrid, "Processes")
	// The sections are open: Apps holds the four programs with windows (the browser's
	// three processes in a closed group), Background the build tool, and Windows
	// processes its six.
	if n := grid.Node().GridRows; n != 14 {
		t.Errorf("%d rows shown, want 14: three sections and their eleven entries", n)
	}
	// The sections keep their order whichever way the table sorts.
	for _, by := range []ui.SortState{{Key: colMemory, Desc: true}, {Key: colMemory}, {Key: colName, Desc: true}} {
		a.layout.Sort.Set(by)
		u.Frame()
		order := sectionOrder(u)
		if want := []string{"Apps (4)", "Background processes (1)", "Windows processes (6)"}; !slices.Equal(order, want) {
			t.Errorf("sorted by %v, the sections show in the order %q", by, order)
		}
	}
	// The group is named for the program, not its executable.
	u.ByName("Browser (3)")
	child := proc(t, s, "browser.exe")
	a.selectProcess(child)
	u.Frame()
	if !slices.Contains(a.expanded.Peek(), browserGroup) {
		t.Errorf("selecting a process in a closed group did not open it: %v", a.expanded.Peek())
	}
	if got := u.Get("detail-name").Text(); got != "Browser" {
		t.Errorf("details show %q", got)
	}

	u.ByRole(access.RoleRadioButton, "All processes").Click()
	if n := u.ByRole(access.RoleDataGrid, "Processes").Node().GridRows; n != len(s.Latest.Peek().Procs) {
		t.Errorf("%d rows in the plain list, want every process", n)
	}
}

func TestSelectingAGroupSumsItUp(t *testing.T) {
	u, a, _, _ := open(t)
	a.selected.Set([]string{browserGroup})
	u.Frame()
	if got := u.Get("detail-facts").Text(); !strings.Contains(got, "Processes 3") {
		t.Errorf("group facts = %q", got)
	}
}

func TestProgramIconsAreReadOncePerExecutable(t *testing.T) {
	src := sample.NewSynthetic(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), time.Second)
	s := store.New(src, time.Second)
	s.Step()
	var mu sync.Mutex
	reads := map[string]int{}
	a := New(s, Layout{FileIcon: func(path string, size int) (image.Image, error) {
		mu.Lock()
		reads[path]++
		mu.Unlock()
		return image.NewNRGBA(image.Rect(0, 0, size, size)), nil
	}})
	u := uitest.New(t, a.Root(), uitest.Size(1360, 860))
	u.WaitFor("the browser's icon", func() bool { return a.icons.get(`C:\Programs\browser.exe`) != nil })
	a.expanded.Set([]string{browserGroup})
	for range 3 {
		s.Step()
		u.Frame()
	}
	mu.Lock()
	defer mu.Unlock()
	if n := reads[`C:\Programs\browser.exe`]; n != 1 {
		t.Errorf("the browser's icon was read %d times for its three processes, want once", n)
	}
}

// sectionOrder returns the names of the sections' rows, in the order the table
// shows them.
func sectionOrder(u *uitest.UI) []string {
	var out []string
	u.Tree().Walk(func(n *access.Node) bool {
		if n.Role == access.RoleDataItem && strings.HasSuffix(n.Name, ")") &&
			(strings.HasPrefix(n.Name, "Apps") || strings.HasPrefix(n.Name, "Background") || strings.HasPrefix(n.Name, "Windows processes")) {
			out = append(out, n.Name)
		}
		return true
	})
	return out
}
