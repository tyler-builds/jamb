package hot

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tyler-builds/jamb/input"
	"github.com/tyler-builds/jamb/ui"
)

// restart simulates jamb dev's restart: it writes a checkpoint, forgets this
// process's signals and reads the checkpoint, as the next build does at start.
func restart(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	if _, err := Checkpoint(path); err != nil {
		t.Fatal(err)
	}
	fresh()
	if err := load(path); err != nil {
		t.Fatal(err)
	}
}

// fresh forgets every signal, as a new process starts with none. Unless a
// test is about startup, the app has started: its signals are made after it.
func fresh() {
	state.Lock()
	defer state.Unlock()
	state.counts = map[string]int{}
	state.tracked = nil
	state.runtimes, state.views = nil, nil
	state.restored = map[string]*entry{}
	state.dec = nil
	state.started = true
	state.kept, state.fresh, state.reset, state.summary = 0, 0, nil, true // no summary in tests
}

// starting makes the app not started yet, for a test of what startup code does.
func starting() {
	state.Lock()
	state.started = false
	state.Unlock()
}

type note struct {
	Title *ui.Signal[string]
	tags  []string
	seen  map[string]time.Time
	mu    sync.Mutex
	next  *note
}

type model struct {
	Notes    []*note
	Selected *note // shares a pointer with Notes
	count    int
	ratio    float64
	any      any
	OnChange func() // nil: kept
}

func TestSignalsKeepTheirValues(t *testing.T) {
	fresh()
	t.Cleanup(fresh)
	when := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	a := &note{Title: Signal(ui.NewSignal("A"), "store:New/Note.Title"), tags: []string{"x"}, seen: map[string]time.Time{"me": when}}
	b := &note{Title: Signal(ui.NewSignal("B"), "store:New/Note.Title")}
	a.next, b.next = b, a // a cycle
	m := Signal(ui.NewSignal(&model{}), "main:main/m")
	m.Set(&model{Notes: []*note{a, b}, Selected: b, count: 3, ratio: 0.5, any: "text"})
	a.Title.Set("A, edited")
	clicks := Signal(ui.NewSignal(0), "main:main/clicks")
	clicks.Set(7)

	restart(t)

	// The new build makes its signals again, in the same order, with their
	// initial values; each takes its old one.
	a2 := Signal(ui.NewSignal("A"), "store:New/Note.Title")
	b2 := Signal(ui.NewSignal("B"), "store:New/Note.Title")
	m2 := Signal(ui.NewSignal(&model{}), "main:main/m")
	clicks2 := Signal(ui.NewSignal(0), "main:main/clicks")
	if clicks2.Peek() != 7 || a2.Peek() != "A, edited" || b2.Peek() != "B" {
		t.Fatalf("clicks %d, titles %q %q", clicks2.Peek(), a2.Peek(), b2.Peek())
	}
	got := m2.Peek()
	if len(got.Notes) != 2 || got.count != 3 || got.ratio != 0.5 || got.any != "text" {
		t.Fatalf("model: %+v", got)
	}
	if got.Selected != got.Notes[1] {
		t.Error("a shared pointer was not shared after the restart")
	}
	if got.Notes[0].next != got.Notes[1] || got.Notes[1].next != got.Notes[0] {
		t.Error("the cycle was not rebuilt")
	}
	if got.Notes[0].Title != a2 {
		t.Error("the model's note does not hold the restored title signal")
	}
	if got.Notes[0].tags[0] != "x" || !got.Notes[0].seen["me"].Equal(when) {
		t.Errorf("unexported fields: %+v", got.Notes[0])
	}
}

func TestChangedTypesStartAgain(t *testing.T) {
	fresh()
	t.Cleanup(fresh)
	Signal(ui.NewSignal("hello"), "k").Set("changed")
	Signal(ui.NewSignal(struct{ A, B int }{1, 2}), "s").Set(struct{ A, B int }{3, 4})
	restart(t)
	n := Signal(ui.NewSignal(5), "k") // was a string
	if n.Peek() != 5 {
		t.Errorf("a signal whose type changed has %d; want its initial 5", n.Peek())
	}
	// A struct with a field added and one removed keeps the field it still
	// has, and the added field has its default.
	s := Signal(ui.NewSignal(struct {
		A int
		C string
	}{1, "new"}), "s")
	if s.Peek().A != 3 || s.Peek().C != "new" {
		t.Errorf("struct across a change: %+v", s.Peek())
	}
	if state.fresh != 1 || state.kept != 1 {
		t.Errorf("fresh %d, kept %d; want the changed type fresh and the struct kept", state.fresh, state.kept)
	}
}

func TestValuesThatCannotBeKept(t *testing.T) {
	fresh()
	t.Cleanup(fresh)
	f := Signal(ui.NewSignal(func() {}), "f")
	_ = f
	ok := Signal(ui.NewSignal(1), "ok")
	ok.Set(2)
	path := filepath.Join(t.TempDir(), "state.json")
	n, err := Checkpoint(path)
	if err != nil || n != 1 {
		t.Fatalf("checkpoint kept %d values, %v; want the one without a func", n, err)
	}
}

// A component's locals come back in the component at the same place.
func TestLocalsKeepTheirValues(t *testing.T) {
	fresh()
	t.Cleanup(fresh)
	var open *ui.Signal[bool]
	root := func(ctx *ui.Context) ui.Element {
		return ui.Div().Child(ui.Component(func(ctx *ui.Context) ui.Element {
			open = ui.Local(ctx, "open", false)
			return ui.Text("x")
		}))
	}
	rt := ui.NewRuntime(root, nil, nil)
	rt.SetViewport(100, 100, 1)
	rt.Frame(time.Now())
	open.Set(true)
	restart(t)
	rt2 := ui.NewRuntime(root, nil, nil)
	rt2.SetViewport(100, 100, 1)
	rt2.Frame(time.Now())
	if !open.Peek() {
		t.Error("the local did not keep its value")
	}
}

// A checkpoint is plain JSON, with a version.
func TestCheckpointFile(t *testing.T) {
	fresh()
	t.Cleanup(fresh)
	Signal(ui.NewSignal([]int{1, 2}), "list")
	path := filepath.Join(t.TempDir(), "state.json")
	if _, err := Checkpoint(path); err != nil {
		t.Fatal(err)
	}
	var f file
	b, _ := readFile(path)
	if err := json.Unmarshal(b, &f); err != nil || f.Version != stateVersion || len(f.Entries) != 1 || f.Entries[0].ID != "list#1" {
		t.Fatalf("file: %s %v", b, err)
	}
	if !reflect.DeepEqual(f.Entries[0].Value.V.V.A[1], &node{K: kInt, I: 2}) {
		t.Errorf("entry: %s", b)
	}
}

func readFile(p string) ([]byte, error) { return os.ReadFile(p) }

// Startup code that sets a signal after making it, as Go apps do: when it sets
// what it set last time, the user's value from before the restart wins; when it
// sets something new, its new value does.
func TestStartupWrites(t *testing.T) {
	fresh()
	t.Cleanup(fresh)
	run := func(first string) (selected, query *ui.Signal[string]) {
		starting()
		selected = Signal(ui.NewSignal(""), "store:Open/Store.Selected")
		query = Signal(ui.NewSignal(""), "store:Open/Store.Query")
		selected.Set(first) // the startup code selects the first note
		started()
		return selected, query
	}
	sel, q := run("note-1")
	sel.Set("note-3") // the user's choices
	q.Set("lig")

	restart(t)
	sel, q = run("note-1") // the same startup
	if sel.Peek() != "note-3" || q.Peek() != "lig" {
		t.Fatalf("after a restart with the same startup: selected %q, query %q", sel.Peek(), q.Peek())
	}

	restart(t)
	sel, q = run("note-9") // the data changed: startup selects another note
	if sel.Peek() != "note-9" || q.Peek() != "lig" {
		t.Fatalf("after a restart with a new startup value: selected %q, query %q", sel.Peek(), q.Peek())
	}
	if state.fresh != 1 {
		t.Errorf("%d values took the new start; want 1", state.fresh)
	}

	// A default changed in the code takes effect.
	restart(t)
	starting()
	q = Signal(ui.NewSignal("new default"), "store:Open/Store.Query")
	started()
	if q.Peek() != "new default" {
		t.Errorf("a changed default: %q", q.Peek())
	}
}

func TestMapsEncodeInOrder(t *testing.T) {
	m := map[string]int{}
	for i := range 50 {
		m[strconv.Itoa(i)] = i
	}
	first := baseOf(reflect.ValueOf(&m))
	for range 20 {
		if !bytes.Equal(baseOf(reflect.ValueOf(&m)), first) {
			t.Fatal("a map encoded differently twice")
		}
	}
}

// A component's locals follow it when the components around it move: a local
// is known by the component's call in the source, not its index.
func TestLocalsFollowTheirComponent(t *testing.T) {
	fresh()
	t.Cleanup(fresh)
	swapped := false
	var a, b *ui.Signal[int]
	root := func(ctx *ui.Context) ui.Element {
		ca := Component(ui.Component(func(ctx *ui.Context) ui.Element {
			a = ui.Local(ctx, "n", 0)
			return ui.Text("a")
		}), "views:Root/A")
		cb := Component(ui.Component(func(ctx *ui.Context) ui.Element {
			b = ui.Local(ctx, "n", 0)
			return ui.Text("b")
		}), "views:Root/B")
		if swapped {
			return ui.Div().Children(cb, ca)
		}
		return ui.Div().Children(ca, cb)
	}
	frame := func() {
		rt := ui.NewRuntime(root, nil, nil)
		rt.SetViewport(100, 100, 1)
		rt.Frame(time.Now())
	}
	frame()
	a.Set(1)
	b.Set(2)
	restart(t)
	swapped = true // the code changed: B comes first now
	frame()
	if a.Peek() != 1 || b.Peek() != 2 {
		t.Errorf("after the components swapped places: a %d, b %d; want 1 and 2", a.Peek(), b.Peek())
	}
}

// The focus, a scroll container's position and a text field's caret survive.
func TestViewStateKept(t *testing.T) {
	fresh()
	t.Cleanup(fresh)
	text := ui.NewSignal("hello world")
	root := func(ctx *ui.Context) ui.Element {
		list := ui.Div().Column().ScrollY().Height(ui.Px(100)).Key("list")
		for i := range 40 {
			list.Child(ui.Text("row " + strconv.Itoa(i)).Key("row" + strconv.Itoa(i)))
		}
		return ui.Div().Column().Children(ui.TextInput(text).Key("field").Label("Field"), list)
	}
	rt := ui.NewRuntime(root, nil, nil)
	rt.SetViewport(300, 300, 1)
	rt.Frame(time.Now())
	rt.ClickKey("field")
	rt.Press(input.KeyHome, 0)
	rt.Press(input.KeyRight, 0)
	rt.Press(input.KeyRight, 0) // the caret after "he"
	r, _ := rt.Find("list")
	rt.Scroll(r.X+10, r.Y+10, 0, 300, true, 0)
	rt.Frame(time.Now())
	before, _ := rt.Find("row0")

	restart(t)
	rt2 := ui.NewRuntime(root, nil, nil)
	rt2.SetViewport(300, 300, 1)
	rt2.Frame(time.Now())
	rt2.Frame(time.Now())
	if after, _ := rt2.Find("row0"); after.Y != before.Y {
		t.Errorf("the list scrolled to %v; it was at %v", after.Y, before.Y)
	}
	rt2.Type("X")
	if text.Peek() != "heXllo world" {
		t.Errorf("typing after the restart gave %q; want the focus and caret kept, \"heXllo world\"", text.Peek())
	}
}

// A ListOf of rows of different heights comes back with the same row at the
// top, though a new process estimates the rows it has not measured anew.
func TestListScrollKept(t *testing.T) {
	fresh()
	t.Cleanup(fresh)
	items := make([]int, 300)
	for i := range items {
		items[i] = i
	}
	root := func(ctx *ui.Context) ui.Element {
		return ui.Div().Height(ui.Px(300)).Child(
			ui.ListOf(items, func(i int) string { return "item" + strconv.Itoa(i) }, func(_ int, i int) ui.Element {
				return ui.Div().Height(ui.Px(float32(20+(i%5)*15))).Key("row" + strconv.Itoa(i)).Child(ui.Text("row " + strconv.Itoa(i)))
			}).Key("list").Grow(1))
	}
	topRow := func(rt *ui.Runtime) string {
		lr, _ := rt.Find("list")
		best, bestY := "", float32(1e9)
		for i := range items {
			if r, ok := rt.Find("row" + strconv.Itoa(i)); ok && r.H > 0 && r.Y+r.H > lr.Y && r.Y < bestY {
				best, bestY = strconv.Itoa(i), r.Y
			}
		}
		return best
	}
	rt := ui.NewRuntime(root, nil, nil)
	rt.SetViewport(300, 300, 1)
	rt.Frame(time.Now())
	lr, _ := rt.Find("list")
	for range 30 {
		rt.Scroll(lr.X+10, lr.Y+10, 0, 400, true, 0)
		rt.Frame(time.Now())
	}
	before := topRow(rt)
	if before == "" || before == "0" {
		t.Fatalf("the list did not scroll (top row %q)", before)
	}
	restart(t)
	rt2 := ui.NewRuntime(root, nil, nil)
	rt2.SetViewport(300, 300, 1)
	for range 3 {
		rt2.Frame(time.Now())
	}
	if after := topRow(rt2); after != before {
		t.Errorf("the top row is %s after the restart; it was %s", after, before)
	}
}
